package router

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	defaultMaxPerTenant = 8
	tenantGateKeyPrefix = "asynq:tenant_gate:"
	tenantGateTTL       = 5 * time.Minute
)

var tenantGateScript = redis.NewScript(`
local key   = KEYS[1]
local maxW  = tonumber(ARGV[1])
local ttlMs = tonumber(ARGV[2])

local count = redis.call('INCR', key)
redis.call('PEXPIRE', key, ttlMs)
if count <= maxW then
    return 1
end
redis.call('DECR', key)
return 0
`)

// TenantGateMiddleware limits concurrent asynq tasks per tenant so one noisy
// tenant cannot starve others. Tasks that exceed the per-tenant cap are
// returned to the queue for later retry.
func TenantGateMiddleware(redisClient redis.UniversalClient) func(asynq.Handler) asynq.Handler {
	maxPerTenant := defaultMaxPerTenant
	if v := os.Getenv("WEKNORA_TENANT_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxPerTenant = n
		}
	}

	// No limit configured — pass-through to avoid Redis dependency.
	if maxPerTenant <= 0 {
		return func(next asynq.Handler) asynq.Handler {
			return next
		}
	}

	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			tenantID := extractTenantID(task.Payload())
			if tenantID == 0 {
				// No tenant in payload — pass through.
				return next.ProcessTask(ctx, task)
			}

			key := fmt.Sprintf("%s%d", tenantGateKeyPrefix, tenantID)

			// Acquire per-tenant slot.
			for {
				result, err := tenantGateScript.Run(ctx, redisClient,
					[]string{key},
					maxPerTenant, tenantGateTTL.Milliseconds(),
				).Int64()
				if err != nil {
					logger.Warnf(ctx, "[TenantGate] Redis error for tenant %d, passing through: %v", tenantID, err)
					break
				}
				if result == 1 {
					break
				}

				// Tenant at limit — wait and retry.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(2 * time.Second):
				}
			}

			// Run the real handler.
			err := next.ProcessTask(ctx, task)

			// Release the slot.
			if n, decrErr := redisClient.Decr(ctx, key).Result(); decrErr == nil && n <= 0 {
				redisClient.Del(ctx, key)
			}

			return err
		})
	}
}

func extractTenantID(payload []byte) uint64 {
	var partial struct {
		TenantID uint64 `json:"tenant_id"`
	}
	if err := json.Unmarshal(payload, &partial); err != nil {
		return 0
	}
	return partial.TenantID
}
