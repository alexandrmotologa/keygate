.PHONY: all build test lint run clean benchmark

all: build

build-web:
	cd web && npm install && npm run build

build: build-web
	go build -ldflags="-s -w" -o bin/keygate cmd/keygate/main.go

test:
	go test -v -race ./...

benchmark:
	go test -bench=. -benchmem ./...

run: build
	./bin/keygate serve --port 8000 --mock-upstream --db ./keygate.db

clean:
	rm -rf bin/ web/dist/ keygate.db*
