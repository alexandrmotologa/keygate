package ratelimit

import "strings"

// StandardTier represents pre-configured tier settings.
type StandardTier struct {
	Name     string
	Limit    int
	Duration int // seconds
	Burst    int
}

var DefaultTiers = map[string]StandardTier{
	"free": {
		Name:     "free",
		Limit:    10,
		Duration: 60,
		Burst:    2,
	},
	"pro": {
		Name:     "pro",
		Limit:    500,
		Duration: 60,
		Burst:    25,
	},
	"enterprise": {
		Name:     "enterprise",
		Limit:    10000,
		Duration: 60,
		Burst:    500,
	},
}

// ResolveTier returns the configuration for a named tier or default free tier.
func ResolveTier(tierName string) StandardTier {
	tier := strings.ToLower(strings.TrimSpace(tierName))
	if t, exists := DefaultTiers[tier]; exists {
		return t
	}
	return DefaultTiers["free"]
}
