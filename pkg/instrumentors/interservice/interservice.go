package interservice

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
	"os"
	"sync"
	"time"
)

const (
	REDIS_ADDR     = "REDIS_ADDRESS"
	REDIS_PASSWORD = "REDIS_PASSWORD"
)

var redisTimeout = 5 * time.Second

// define struct for redis conn
type redisConfig struct {
	Address  string
	Password string
}

func mustLookupEnv(envName string) string {
	value, exists := os.LookupEnv(envName)
	if exists {
		return value
	}

	panic(fmt.Sprintf("Not found env %s", envName))
}

func parseEnv() redisConfig {
	cfg := redisConfig{}
	cfg.Address = mustLookupEnv(REDIS_ADDR)
	cfg.Password = mustLookupEnv(REDIS_PASSWORD)
	return cfg
}

var rdb *redis.Client
var syncConn = sync.Once{}
var lock = sync.Mutex{}

func InitRedisConnection() {
	cfg := parseEnv()
	rdb = redis.NewClient(&redis.Options{
		Addr:       cfg.Address,
		Password:   cfg.Password,
		DB:         0,
		MaxRetries: 3,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}
}

func SetMappingTraceID(ctx context.Context, id trace.TraceID, pid trace.TraceID) error {
	return rdb.Set(ctx, id.String(), pid, redisTimeout).Err()
}

func GetMappingTraceID(ctx context.Context, id trace.TraceID) (trace.TraceID, error) {
	pid := trace.TraceID{}
	err := rdb.Get(ctx, id.String()).Scan(&pid)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// cache miss
			return pid, nil
		}
		return pid, err
	}

	// cache hit
	return pid, nil
}
