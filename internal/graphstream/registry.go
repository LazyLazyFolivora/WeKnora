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
}

var (
	mu         sync.RWMutex
	sinks      = make(map[string]*Sink) // streamKey -> sink
	tokenSeq   uint64
	runFailer  func(ctx context.Context, streamKey string)
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
	mu.Unlock()
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
	}
}

// Dispatch turns one notification payload into an EventBus event.
// Returns false when no live tool call matches (late / stale notification).
// Emit runs asynchronously so mcp-go's SSE reader is never blocked on DB/Redis.
func Dispatch(params map[string]any) bool {
	if params == nil {
		return false
	}

	streamKey, _ := params["session_id"].(string)
	if streamKey == "" {
		return false
	}

	mu.RLock()
	sink := sinks[streamKey]
	mu.RUnlock()
	if sink == nil || sink.EventBus == nil {
		return false
	}

	seq := asInt64(params["seq"])
	eventRaw, _ := params["event"].(map[string]any)
	if eventRaw == nil {
		logger.Debugf(sink.Ctx, "[GraphStream] missing event object stream_key=%s seq=%d", streamKey, seq)
		return false
	}

	eventType, _ := eventRaw["event_type"].(string)
	eventID, _ := eventRaw["event_id"].(string)
	ts := asFloat64(eventRaw["timestamp"])

	payload := make(map[string]any, len(eventRaw))
	for k, v := range eventRaw {
		payload[k] = v
	}

	logger.Debugf(sink.Ctx, "[GraphStream] recv seq=%d type=%s stream_key=%s", seq, eventType, streamKey)

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
		_ = bus.Emit(ctx, evt)
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
