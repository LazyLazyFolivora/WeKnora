package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SystemDefaultKB associates a set of knowledge-base IDs with every chat request.
// KB IDs come from a row in system_settings with key "system_default_kb_ids".
const systemDefaultKBKey = "system_default_kb_ids"

// ChatRoutePrefixes are the URL prefixes that the middleware should intercept.
var chatRoutePrefixes = []string{
	"/api/v1/knowledge-chat/",
	"/api/v1/agent-chat/",
}

// SystemDefaultKB returns a Gin middleware that injects system default KB IDs
// into KnowledgeQA / AgentQA request bodies.
func SystemDefaultKB(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !shouldIntercept(c) {
			c.Next()
			return
		}

		var setting types.SystemSetting
		if err := db.Where("key = ?", systemDefaultKBKey).First(&setting).Error; err != nil {
			c.Next()
			return
		}

		var ids []string
		if err := json.Unmarshal(json.RawMessage(setting.Value), &ids); err != nil || len(ids) == 0 {
			c.Next()
			return
		}

		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}
		c.Request.Body.Close()

		var body map[string]interface{}
		if err := json.Unmarshal(raw, &body); err != nil {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
			c.Next()
			return
		}

		existing := toStringSlice(body["knowledge_base_ids"])
		merged := dedup(append(existing, ids...))
		body["knowledge_base_ids"] = merged
		logger.Infof(c.Request.Context(), "[SystemDefaultKB] Injected KBs: existing=%v, default=%v, merged=%v", existing, ids, merged)
		ctx := context.WithValue(c.Request.Context(), types.SystemDefaultKBIDsContextKey, ids)
		c.Request = c.Request.WithContext(ctx)
		newRaw, _ := json.Marshal(body)

		c.Request.Body = io.NopCloser(bytes.NewBuffer(newRaw))
		c.Request.ContentLength = int64(len(newRaw))
		c.Request.Header.Set("Content-Length", strconv.Itoa(len(newRaw)))

		c.Next()
	}
}

func shouldIntercept(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost {
		return false
	}
	path := c.Request.URL.Path
	for _, prefix := range chatRoutePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func dedup(slice []string) []string {
	seen := make(map[string]bool, len(slice))
	out := make([]string, 0, len(slice))
	for _, s := range slice {
		if s == "" {
			continue
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

