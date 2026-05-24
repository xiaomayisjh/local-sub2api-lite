package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimitFailureMode int

const (
	RateLimitFailOpen RateLimitFailureMode = iota
	RateLimitFailClose
)

type RateLimitOptions struct {
	FailureMode RateLimitFailureMode
}

type rateLimitEntry struct {
	count    int64
	expireAt time.Time
}

var rateLimitMu sync.Mutex
var rateLimitEntries = make(map[string]*rateLimitEntry)

func rateLimitRun(ctx context.Context, key string, windowMillis int64) (int64, bool, error) {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()

	now := time.Now()
	entry, ok := rateLimitEntries[key]
	if !ok || now.After(entry.expireAt) {
		entry = &rateLimitEntry{
			count:    1,
			expireAt: now.Add(time.Duration(windowMillis) * time.Millisecond),
		}
		rateLimitEntries[key] = entry
		return 1, false, nil
	}

	entry.count++
	repaired := entry.expireAt.Sub(now) <= 0
	if repaired {
		entry.expireAt = now.Add(time.Duration(windowMillis) * time.Millisecond)
	}

	return entry.count, repaired, nil
}

type RateLimiter struct {
	prefix string
}

func NewRateLimiter(_ interface{}) *RateLimiter {
	return &RateLimiter{prefix: "rate_limit:"}
}

func (r *RateLimiter) Limit(key string, limit int, window time.Duration) gin.HandlerFunc {
	return r.LimitWithOptions(key, limit, window, RateLimitOptions{})
}

func (r *RateLimiter) LimitWithOptions(key string, limit int, window time.Duration, opts RateLimitOptions) gin.HandlerFunc {
	windowMillis := window.Milliseconds()

	return func(c *gin.Context) {
		fullKey := r.prefix + key

		count, _, err := rateLimitRun(c.Request.Context(), fullKey, windowMillis)
		if err != nil {
			log.Printf("Rate limit error: %v", err)
			if opts.FailureMode == RateLimitFailOpen {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		remaining := int64(limit) - count
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(window).Unix(), 10))

		if count > int64(limit) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		c.Next()
	}
}

func parseInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case int:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	default:
		return 0, fmt.Errorf("cannot parse %T as int64", v)
	}
}
