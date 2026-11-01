package graphstream

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
)

// NotificationMethod is the MCP notification method BioDSA uses for graph events.
const NotificationMethod = "notifications/biodsa/graph_event"

// Sink routes graph notifications from one MCP tool call back to the
// EventBus of the agent turn that issued it.
type Sink struct {
	token              uint64
	Ctx                context.Context
	EventBus           *event.EventBus
	SessionID          string
	AssistantMessageID string
	ToolCallID         string
	ServiceID          string
	TenantID           uint64
}

var (
	mu        sync.RWMutex
	sinks     = make(map[string]*Sink) // streamKey -> sink
	tokenSeq  uint64
	runFailer func(ctx context.Context, streamKey string)
)

// SetRunFailer registers an optional hook used when a tool call ends in error
// while a graph stream was still running. Wired by AgentGraphService construction.
func SetRunFailer(fn func(ctx context.Context, streamKey string)) {
	runFailer = fn
}

// FailRunIfHooked marks a stream's run failed when the tool call errored.
func FailRunIfHooked(ctx context.Context, streamKey string) {
	if streamKey == "" || runFailer == nil {
		return
	}
	runFailer(ctx, streamKey)
}

// Register associates a stream_key with the live agent turn that owns it.
// Returns a token that must be passed to Unregister to avoid ABA deletes.
func Register(streamKey string, s *Sink) uint64 {
	if streamKey == "" || s == nil {
		return 0
	}
	tok := atomic.AddUint64(&tokenSeq, 1)
	s.token = tok
	mu.Lock()
	sinks[streamKey] = s
	n := len(sinks)
	mu.Unlock()
	logger.Infof(context.Background(), "[GraphStream] register stream_key=%s token=%d live=%d tenant=%d",
		streamKey, tok, n, s.TenantID)
	return tok
}

// Unregister removes a stream_key only when the token still matches.
func Unregister(streamKey string, token uint64) {
	if streamKey == "" || token == 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if cur, ok := sinks[streamKey]; ok && cur != nil && cur.token == token {
		delete(sinks, streamKey)
		logger.Infof(context.Background(), "[GraphStream] unregister stream_key=%s token=%d live=%d",
			streamKey, token, len(sinks))
	}
}

// Dispatch turns one notification payload into an EventBus event.
// Returns false when no live tool call matches (late / stale notification).
// Emit runs asynchronously so mcp-go's SSE reader is never blocked on DB/Redis.
func Dispatch(params map[string]any) bool {
	if params == nil {
		logger.Warnf(context.Background(), "[GraphStream] dispatch drop: nil params")
		return false
	}

	streamKey, _ := params["session_id"].(string)
	if streamKey == "" {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		logger.Warnf(context.Background(), "[GraphStream] dispatch drop: empty session_id keys=%v", keys)
		return false
	}

	seq := asInt64(params["seq"])

	mu.RLock()
	sink := sinks[streamKey]
	live := len(sinks)
	mu.RUnlock()
	if sink == nil || sink.EventBus == nil {
		logger.Warnf(context.Background(),
			"[GraphStream] dispatch drop: no live sink stream_key=%s seq=%d live=%d", streamKey, seq, live)
		return false
	}

	eventRaw, _ := params["event"].(map[string]any)
	if eventRaw == nil {
		logger.Warnf(sink.Ctx, "[GraphStream] dispatch drop: missing event object stream_key=%s seq=%d", streamKey, seq)
		return false
	}

	eventType, _ := eventRaw["event_type"].(string)
	eventID, _ := eventRaw["event_id"].(string)
	ts := asFloat64(eventRaw["timestamp"])

	payload := make(map[string]any, len(eventRaw))
	for k, v := range eventRaw {
		payload[k] = v
	}

	// Info for first event, RunComplete, and every 25th — enough to reconcile
	// with BioDSA STREAM sent=N without drowning logs on large runs.
	if seq == 1 || seq%25 == 0 || eventType == "RunComplete" {
		logger.Infof(sink.Ctx, "[GraphStream] recv seq=%d type=%s stream_key=%s", seq, eventType, streamKey)
	} else {
		logger.Debugf(sink.Ctx, "[GraphStream] recv seq=%d type=%s stream_key=%s", seq, eventType, streamKey)
	}

	evt := event.Event{
		Type: event.EventAgentGraph,
		Data: event.AgentGraphData{
			StreamKey:          streamKey,
			Seq:                seq,
			EventType:          eventType,
			EventID:            eventID,
			Timestamp:          ts,
			Payload:            payload,
			SessionID:          sink.SessionID,
			AssistantMessageID: sink.AssistantMessageID,
			ToolCallID:         sink.ToolCallID,
			TenantID:           sink.TenantID,
		},
	}
	bus := sink.EventBus
	ctx := sink.Ctx
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf(ctx, "[GraphStream] emit panic: %v", r)
			}
		}()
		if err := bus.Emit(ctx, evt); err != nil {
			logger.Errorf(ctx, "[GraphStream] emit failed seq=%d type=%s stream_key=%s err=%v",
				seq, eventType, streamKey, err)
		}
	}()
	return true
}

// SummarizeRunCompletePayload keeps only lightweight fields for storage/SSE.
func SummarizeRunCompletePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for _, k := range []string{"event_id", "timestamp", "event_type", "total_steps", "duration_seconds", "final_response_preview"} {
		if v, ok := payload[k]; ok {
			out[k] = v
		}
	}
	if ents, ok := payload["entities"].([]any); ok {
		out["entity_count"] = len(ents)
	}
	if rels, ok := payload["relations"].([]any); ok {
		out["relation_count"] = len(rels)
	}
	return out
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	default:
		return 0
	}
}

func asFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return 0
	}
}
