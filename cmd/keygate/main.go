package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/alexandrmotologa/keygate/internal/keys"
	"github.com/alexandrmotologa/keygate/internal/proxy"
	"github.com/alexandrmotologa/keygate/internal/ratelimit"
	"github.com/alexandrmotologa/keygate/internal/server"
	"github.com/alexandrmotologa/keygate/internal/storage"
)

const banner = `
  _  __          ____       _       
 | |/ /___ _   _/ ___| __ _| |_ ___ 
 | ' // _ \ | | | |  _ / _` + "`" + ` | __/ _ \
 | . \  __/ |_| | |_| | (_| | ||  __/
 |_|\_\___|\__, |\____|\__,_|\__\___|
           |___/                     
`

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "serve":
		cmdServe(os.Args[2:])
	case "key":
		cmdKey(os.Args[2:])
	case "benchmark":
		cmdBenchmark(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("KeyGate v1.0.0 (Go 1.23 / React 19)")
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(banner)
	fmt.Println("KeyGate — High-Throughput API Key Gateway & Multi-Tier Rate Limiter")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  keygate serve [flags]       Start proxy gateway, control plane API, and web studio")
	fmt.Println("  keygate key <subcommand>    Manage API keys via CLI (create, verify, revoke, list)")
	fmt.Println("  keygate benchmark [flags]   Execute high-throughput sub-millisecond load benchmark")
	fmt.Println("  keygate version             Print version information")
	fmt.Println()
	fmt.Println("Serve Flags:")
	fmt.Println("  --port <int>                HTTP listen port (default: 8000)")
	fmt.Println("  --target <url>              Upstream target backend for reverse proxy (default: http://localhost:3000)")
	fmt.Println("  --mock-upstream             Start internal echo backend for testing with zero dependencies")
	fmt.Println("  --db <path>                 SQLite database file path (default: ./keygate.db)")
	fmt.Println()
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8000, "HTTP server port")
	target := fs.String("target", "http://localhost:3000", "Reverse proxy upstream target URL")
	mockUpstream := fs.Bool("mock-upstream", false, "Start embedded mock upstream echo server")
	dbPath := fs.String("db", "./keygate.db", "SQLite database path")
	_ = fs.Parse(args)

	fmt.Print(banner)
	fmt.Printf("Starting KeyGate on port %d...\n", *port)

	// 1. Storage
	db, err := storage.Open(*dbPath)
	if err != nil {
		fmt.Printf("Database initialization error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Printf("  • SQLite WAL database: %s\n", *dbPath)

	// 2. Cache & Validator
	validator, err := keys.NewValidator(db)
	if err != nil {
		fmt.Printf("Validator cache initialization error: %v\n", err)
		os.Exit(1)
	}
	defer validator.Close()
	fmt.Println("  • In-Memory Ristretto cache: initialized")

	// 3. Rate Limiter
	limiter := ratelimit.NewLimiter()
	defer limiter.Close()
	fmt.Println("  • Sliding-window rate limiter: ready")

	// 4. SSE Telemetry Broker
	broker := server.NewEventBroker()

	// 5. Upstream target resolution
	targetURL := *target
	if *mockUpstream {
		mockSrv, err := proxy.StartMockUpstream("127.0.0.1:8001")
		if err != nil {
			fmt.Printf("Failed to start mock upstream: %v\n", err)
			os.Exit(1)
		}
		defer mockSrv.Close()
		targetURL = mockSrv.Addr
		fmt.Printf("  • Embedded mock upstream: listening on %s\n", mockSrv.Addr)
	} else {
		fmt.Printf("  • Reverse proxy upstream: %s\n", targetURL)
	}

	// 6. Reverse Proxy Handler
	proxyHandler, err := proxy.NewProxy(proxy.ProxyConfig{
		TargetURL: targetURL,
		Validator: validator,
		Limiter:   limiter,
		DB:        db,
		Sink:      broker,
	})
	if err != nil {
		fmt.Printf("Failed to configure reverse proxy: %v\n", err)
		os.Exit(1)
	}

	// 7. REST API & Static Files
	api := server.NewAPI(server.APIConfig{
		DB:        db,
		Validator: validator,
		Limiter:   limiter,
		Broker:    broker,
	})

	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	// Register static SPA handler
	err = server.RegisterStaticFiles(mux)
	if err != nil {
		fmt.Printf("Notice: Embedded web assets not mounted: %v\n", err)
	}

	// Root dispatcher:
	// If path starts with /v1/ or /healthz -> API
	// If incoming request has Authorization or X-API-Key header AND is not an API route -> Proxy
	// Otherwise -> Web Studio SPA
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API & Health routes
		if strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/healthz" {
			mux.ServeHTTP(w, r)
			return
		}

		// If client supplies API key credentials and targets an upstream path, forward via proxy
		hasAuth := r.Header.Get("Authorization") != "" || r.Header.Get("X-API-Key") != ""
		if hasAuth {
			proxyHandler.ServeHTTP(w, r)
			return
		}

		// Serve Web Studio
		mux.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      rootHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	fmt.Println()
	fmt.Printf("KeyGate Ready:\n")
	fmt.Printf("  • Management Studio: http://localhost:%d/\n", *port)
	fmt.Printf("  • Verification API:  http://localhost:%d/v1/keys.verifyKey\n", *port)
	fmt.Printf("  • Proxy Gateway:     http://localhost:%d/ (via Bearer token)\n", *port)
	fmt.Println()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server failure: %v\n", err)
		}
	}()

	<-stop
	fmt.Println("\nShutting down KeyGate gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	fmt.Println("Server stopped.")
}

func cmdKey(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: keygate key <create|verify|revoke|list> [flags]")
		os.Exit(1)
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "create":
		keyCreate(subArgs)
	case "verify":
		keyVerify(subArgs)
	case "revoke":
		keyRevoke(subArgs)
	case "list":
		keyList(subArgs)
	default:
		fmt.Printf("Unknown key subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func keyCreate(args []string) {
	fs := flag.NewFlagSet("key create", flag.ExitOnError)
	name := fs.String("name", "CLI Key", "Key name")
	wsID := fs.String("workspace", "ws_default", "Workspace ID")
	prefix := fs.String("prefix", "sk_live", "Key prefix")
	owner := fs.String("owner", "usr_cli", "Owner ID")
	tier := fs.String("tier", "pro", "Rate limit tier (free, pro, enterprise)")
	limit := fs.Int("limit", 0, "Custom rate limit per window")
	scopesStr := fs.String("scopes", "", "Comma-separated scopes")
	dbPath := fs.String("db", "./keygate.db", "Database path")
	_ = fs.Parse(args)

	db, err := storage.Open(*dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()
	gen, err := keys.Generate(keys.KeyGenerationOptions{
		Prefix:      *prefix,
		WorkspaceID: *wsID,
	})
	if err != nil {
		fmt.Printf("Generation error: %v\n", err)
		os.Exit(1)
	}

	salt, err := db.GetWorkspaceSalt(ctx, *wsID)
	if err != nil {
		fmt.Printf("Salt error: %v\n", err)
		os.Exit(1)
	}

	keyHash := keys.HashKey(gen.RawKey, salt)
	tierConfig := ratelimit.ResolveTier(*tier)
	finalLimit := *limit
	if finalLimit <= 0 {
		finalLimit = tierConfig.Limit
	}

	var scopes []string
	if *scopesStr != "" {
		for _, s := range strings.Split(*scopesStr, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				scopes = append(scopes, trimmed)
			}
		}
	}

	now := time.Now().UTC()
	apiKey := &storage.APIKey{
		ID:           gen.KeyID,
		KeyHash:      keyHash,
		Prefix:       gen.Prefix,
		Name:         *name,
		OwnerID:      *owner,
		WorkspaceID:  *wsID,
		Tier:         *tier,
		RateLimit:    finalLimit,
		RateDuration: tierConfig.Duration,
		Burst:        tierConfig.Burst,
		Scopes:       scopes,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := db.CreateKey(ctx, apiKey); err != nil {
		fmt.Printf("Database insert error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("API Key Successfully Created:")
	fmt.Printf("  Key ID:     %s\n", apiKey.ID)
	fmt.Printf("  Plaintext:  %s\n", gen.RawKey)
	fmt.Printf("  Tier:       %s (%d req/%ds)\n", apiKey.Tier, apiKey.RateLimit, apiKey.RateDuration)
	fmt.Printf("  Workspace:  %s\n", apiKey.WorkspaceID)
	fmt.Printf("  Owner:      %s\n", apiKey.OwnerID)
	fmt.Println("\nNote: The plaintext key cannot be displayed again.")
}

func keyVerify(args []string) {
	fs := flag.NewFlagSet("key verify", flag.ExitOnError)
	rawKey := fs.String("key", "", "API key to verify")
	wsID := fs.String("workspace", "", "Workspace ID (optional)")
	dbPath := fs.String("db", "./keygate.db", "Database path")
	_ = fs.Parse(args)

	if *rawKey == "" {
		fmt.Println("Error: --key is required")
		os.Exit(1)
	}

	db, err := storage.Open(*dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	validator, err := keys.NewValidator(db)
	if err != nil {
		fmt.Printf("Validator error: %v\n", err)
		os.Exit(1)
	}
	defer validator.Close()

	ctx := context.Background()
	res := validator.Verify(ctx, keys.ValidationRequest{
		RawKey:      *rawKey,
		WorkspaceID: *wsID,
	})

	fmt.Printf("Verification Result:\n")
	fmt.Printf("  Valid:       %v\n", res.Valid)
	fmt.Printf("  Code:        %s\n", res.Code)
	fmt.Printf("  Message:     %s\n", res.Message)
	fmt.Printf("  Latency:     %d µs\n", res.DurationUs)
	fmt.Printf("  Cache Hit:   %v\n", res.Cached)
	if res.Key != nil {
		fmt.Printf("  Key ID:      %s\n", res.Key.ID)
		fmt.Printf("  Name:        %s\n", res.Key.Name)
		fmt.Printf("  Tier:        %s\n", res.Key.Tier)
		fmt.Printf("  Rate Limit:  %d req/%ds\n", res.Key.RateLimit, res.Key.RateDuration)
	}
}

func keyRevoke(args []string) {
	fs := flag.NewFlagSet("key revoke", flag.ExitOnError)
	keyID := fs.String("id", "", "Key ID to revoke")
	dbPath := fs.String("db", "./keygate.db", "Database path")
	_ = fs.Parse(args)

	if *keyID == "" {
		fmt.Println("Error: --id is required")
		os.Exit(1)
	}

	db, err := storage.Open(*dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	revokedAt, err := db.RevokeKey(context.Background(), *keyID)
	if err != nil {
		fmt.Printf("Revocation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Key %s successfully revoked at %s\n", *keyID, revokedAt.Format(time.RFC3339))
}

func keyList(args []string) {
	fs := flag.NewFlagSet("key list", flag.ExitOnError)
	wsID := fs.String("workspace", "ws_default", "Workspace ID")
	status := fs.String("status", "all", "Status filter (all, active, revoked)")
	dbPath := fs.String("db", "./keygate.db", "Database path")
	_ = fs.Parse(args)

	db, err := storage.Open(*dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	keysList, total, err := db.ListKeys(context.Background(), storage.ListKeysOptions{
		WorkspaceID: *wsID,
		Status:      *status,
		Limit:       50,
	})
	if err != nil {
		fmt.Printf("List failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d API Keys in workspace %s:\n", total, *wsID)
	fmt.Printf("%-24s %-12s %-10s %-8s %-20s\n", "KEY ID", "PREFIX", "TIER", "STATUS", "NAME")
	fmt.Println(strings.Repeat("-", 80))
	for _, k := range keysList {
		statusStr := "ACTIVE"
		if k.RevokedAt != nil {
			statusStr = "REVOKED"
		} else if !k.Enabled {
			statusStr = "DISABLED"
		}
		fmt.Printf("%-24s %-12s %-10s %-8s %-20s\n", k.ID, k.Prefix, k.Tier, statusStr, k.Name)
	}
}

func cmdBenchmark(args []string) {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)
	numKeys := fs.Int("keys", 1000, "Number of distinct keys to create")
	concurrency := fs.Int("concurrency", 50, "Number of concurrent worker goroutines")
	totalRequests := fs.Int("requests", 50000, "Total verification requests to execute")
	dbPath := fs.String("db", "./keygate_bench.db", "Temporary benchmark database")
	_ = fs.Parse(args)

	defer os.Remove(*dbPath)

	fmt.Print(banner)
	fmt.Println("Running KeyGate High-Throughput Verification Benchmark")
	fmt.Printf("  • Distinct keys:    %d\n", *numKeys)
	fmt.Printf("  • Total requests:   %d\n", *totalRequests)
	fmt.Printf("  • Concurrency:      %d workers\n", *concurrency)
	fmt.Println()

	db, err := storage.Open(*dbPath)
	if err != nil {
		fmt.Printf("Failed to create bench db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	validator, err := keys.NewValidator(db)
	if err != nil {
		fmt.Printf("Failed to create validator: %v\n", err)
		os.Exit(1)
	}
	defer validator.Close()

	ctx := context.Background()
	salt, _ := db.GetWorkspaceSalt(ctx, "ws_default")

	fmt.Print("Generating and seeding keys into SQLite...")
	generatedKeys := make([]string, *numKeys)
	for i := 0; i < *numKeys; i++ {
		gen, _ := keys.Generate(keys.KeyGenerationOptions{
			Prefix:      "sk_live",
			WorkspaceID: "ws_default",
		})
		keyHash := keys.HashKey(gen.RawKey, salt)
		_ = db.CreateKey(ctx, &storage.APIKey{
			ID:           gen.KeyID,
			KeyHash:      keyHash,
			Prefix:       gen.Prefix,
			Name:         fmt.Sprintf("Bench Key %d", i),
			OwnerID:      "usr_bench",
			WorkspaceID:  "ws_default",
			Tier:         "enterprise",
			RateLimit:    100000,
			RateDuration: 60,
			Burst:        1000,
			Enabled:      true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		})
		generatedKeys[i] = gen.RawKey
	}
	fmt.Println(" Done.")

	// Pre-warm cache with keys
	fmt.Print("Warming up in-memory cache...")
	for _, k := range generatedKeys {
		validator.Verify(ctx, keys.ValidationRequest{RawKey: k})
	}
	time.Sleep(50 * time.Millisecond)
	fmt.Println(" Done.")

	fmt.Println("\nStarting concurrent load test...")

	var completedReqs int64
	var validReqs int64
	var cacheHits int64

	latencies := make([]int64, *totalRequests)
	var latIndex int64

	reqPerWorker := *totalRequests / *concurrency
	var wg sync.WaitGroup

	startTime := time.Now()

	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < reqPerWorker; i++ {
				keyIndex := (workerID*reqPerWorker + i) % len(generatedKeys)
				reqKey := generatedKeys[keyIndex]

				t0 := time.Now()
				res := validator.Verify(ctx, keys.ValidationRequest{RawKey: reqKey})
				durUs := time.Since(t0).Microseconds()

				idx := atomic.AddInt64(&latIndex, 1) - 1
				if idx < int64(len(latencies)) {
					latencies[idx] = durUs
				}

				atomic.AddInt64(&completedReqs, 1)
				if res.Valid {
					atomic.AddInt64(&validReqs, 1)
				}
				if res.Cached {
					atomic.AddInt64(&cacheHits, 1)
				}
			}
		}(w)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// Sort latencies for percentiles
	validLats := latencies[:latIndex]
	sort.Slice(validLats, func(i, j int) bool { return validLats[i] < validLats[j] })

	var sumLat int64
	for _, l := range validLats {
		sumLat += l
	}
	avgLatUs := float64(sumLat) / float64(len(validLats))

	p50 := percentileInt64(validLats, 50)
	p90 := percentileInt64(validLats, 90)
	p95 := percentileInt64(validLats, 95)
	p99 := percentileInt64(validLats, 99)

	throughput := float64(completedReqs) / totalDuration.Seconds()

	fmt.Println("\n================ Benchmark Summary ================")
	fmt.Printf("Completed Requests:    %d / %d\n", completedReqs, *totalRequests)
	fmt.Printf("Success Rate:          %.2f%%\n", (float64(validReqs)/float64(completedReqs))*100)
	fmt.Printf("Cache Hit Ratio:       %.2f%%\n", (float64(cacheHits)/float64(completedReqs))*100)
	fmt.Printf("Total Elapsed Time:    %v\n", totalDuration)
	fmt.Printf("Throughput:            %.0f verifications / second\n", throughput)
	fmt.Println("---------------------------------------------------")
	fmt.Printf("Average Latency:       %.2f µs (%.4f ms)\n", avgLatUs, avgLatUs/1000.0)
	fmt.Printf("P50 Latency:           %d µs (%.4f ms)\n", p50, float64(p50)/1000.0)
	fmt.Printf("P90 Latency:           %d µs (%.4f ms)\n", p90, float64(p90)/1000.0)
	fmt.Printf("P95 Latency:           %d µs (%.4f ms)\n", p95, float64(p95)/1000.0)
	fmt.Printf("P99 Latency:           %d µs (%.4f ms)\n", p99, float64(p99)/1000.0)
	fmt.Println("===================================================")
}

func percentileInt64(sorted []int64, pct float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * (pct / 100.0))
	return sorted[idx]
}
