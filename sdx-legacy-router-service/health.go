package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/ONSdigital/sdx-onyx-gazelle/lib/api"
	"github.com/ONSdigital/sdx-onyx-gazelle/lib/redis"
)

type health struct {
	Service      bool               `json:"service"`
	Dependencies healthDependencies `json:"dependencies"`
}

type healthDependencies struct {
	Cache         bool `json:"cache"`     // Redis cache
	QueueIncoming bool `json:"queue_in"`  // Incoming channel
	QueueOutgoing bool `json:"queue_out"` // Outgoing channel
}

var currentHealth *health

const (
	healthUpdateInterval = time.Second * 5
)

// HealthcheckHandler responds to a healthcheck request with the current
// health of the service.
func HealthcheckHandler(rw http.ResponseWriter, r *http.Request) {

	h, err := getHealth()
	if err != nil {
		log.Printf(`event="Error attempting to fetch health" error="%v"`, err)
		api.WriteProblemResponse(api.Problem{
			Title:  "Problem when attempting to construct health",
			Status: http.StatusInternalServerError,
		}, rw)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write(h)
	return
}

// StartHealthChecking starts up a goroutine that monitors service health at
// intervals. Returns a cancel function to stop the goroutine.
func StartHealthChecking() (func(), error) {

	if err := updateHealth(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	log.Printf(`event="Starting healthcheck watcher"`)

	go func(ctx context.Context) {

		ticker := time.NewTicker(healthUpdateInterval)

		for {
			select {
			case <-ctx.Done():
				log.Printf(`event="Canceling healthchecking"`)
				return
			case <-ticker.C:
				log.Printf(`event="Updating health for service"`)
			}
		}

	}(ctx)

	return cancel, nil
}

func updateHealth() error {

	// Start optermistic
	cacheStatus := true
	queueStatus := true

	if err := redis.SetWithExpiry("sdx-legacy-router-healthcheck", "ok", int16(1), redisConn); err != nil {
		cacheStatus = false
	}

	// TODO how to check rabbit connection? Should possibly be done automatically
	// 		via registering something on NotifyClose()?

	currentHealth = &health{
		Service: cacheStatus && queueStatus,
		Dependencies: healthDependencies{
			Cache: cacheStatus,
			Queue: queueStatus,
		},
	}
	return nil
}

func getHealth() ([]byte, error) {
	if currentHealth == nil {
		return nil, errors.New("can't get health - no current health set")
	}
	return json.Marshal(currentHealth)
}
