// Package redis is a convenience layer above the lower level redis library to provide
// some more user friendly common function
package redis

import (
	"log"
	"time"

	"github.com/garyburd/redigo/redis"
)

type (
	// Conn is a proxied redigo connection so that client services don't have
	// to also import redigo
	Conn redis.Conn
)

// ConnectWithRetry will repeatedly try to connect to a redis instance at the
// specified intervals
func ConnectWithRetry(uri string, retryInterval time.Duration) (conn redis.Conn) {
	var err error
	for {
		conn, err = redis.DialURL(uri)
		if err == nil {
			log.Println(`event="Established connection to redis"`)
			return conn
		}
		log.Printf(`event="Failed to connect to redis - retrying" err="%v"`, err)
		time.Sleep(retryInterval)
	}
}

// SetWithExpiry sets a simple key in redis with a TTL
func SetWithExpiry(key, value string, expiry int16, conn redis.Conn) error {
	_, err := conn.Do("SET", key, value, "ex", expiry)
	if err != nil {
		return err
	}
	return nil
}

// Set sets a simple key in redis (key does not set explict expiry)
func Set(key, value string, conn redis.Conn) error {
	_, err := conn.Do("SET", key, value)
	if err != nil {
		return err
	}
	return nil
}

// GetString retrieves a simple key in redis
func GetString(key string, conn redis.Conn) (string, error) {
	value, err := redis.String(conn.Do("GET", key))
	if err != nil {
		return "", err
	}
	return value, nil
}
