package config

import (
	"log"
	"os"
)

// C holds the loaded configuration
var C map[string]string

// Load imports the mandatory environment variables that we care about into a
// map that is accessible to the rest of the service.
func Load() {

	// We know how many items we're going to have in the map
	// so we can pre-declare the length as a compiler hint.
	C = make(map[string]string, 5)

	required := []string{
		"PORT",
		"RABBIT_URL",
		"NOTIFICATION_EXCHANGE",
		"LEGACY_EXCHANGE",
		"DOWNSTREAM_EXCHANGE",
	}

	for _, r := range required {
		if C[r] = os.Getenv(r); len(C[r]) == 0 {
			log.Fatalf(`event="Failed to start - Must supply %s environment variable"`, r)
		}
	}
}
