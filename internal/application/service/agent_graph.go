package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/graphstream"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

type agentGraphService struct {
	repo interfaces.AgentGraphRepository
}

// NewAgentGraphService creates the agent graph persistence service.
func NewAgentGraphService(repo interfaces.AgentGraphRepository) interfaces.AgentGraphService {
	s := &agentGraphService{repo: repo}
	graphstream.SetRunFailer(func(ctx context.Context, streamKey string) {
		_ = s.MarkFailed(ctx, streamKey)
	})
	return s
}

func (s *agentGraphService) MarkFailed(ctx context.Context, streamKey string) error {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 || streamKey == "" {
		return nil
	}
	return s.repo.MarkRunFailed(ctx, tenantID, streamKey)
}

func (s *agentGraphService) Record(
	ctx context.Context, sessionID, messageID string, data event.AgentGraphData,
) error {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if (!ok || tenantID == 0) && data.TenantID > 0 {
		tenantID = data.TenantID
		ok = true
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	}
	if !ok || tenantID == 0 {
		logger.Warnf(ctx, "[AgentGraph] skip record: tenant_id missing stream_key=%s seq=%d type=%s",
			data.StreamKey, data.Seq, data.EventType)
		return nil
	}
	if data.StreamKey == "" || data.Seq <= 0 {
		logger.Warnf(ctx, "[AgentGraph] skip record: invalid key/seq stream_key=%q seq=%d type=%s",
			data.StreamKey, data.Seq, data.EventType)
		return nil
	}
	if sessionID == "" {
		sessionID = data.SessionID
	}
	if messageID == "" {
		messageID = data.AssistantMessageID
	}

	err := s.repo.WithTx(ctx, func(tx interfaces.AgentGraphRepository) error {
		if err := s.ensureRun(ctx, tx, tenantID, sessionID, messageID, data); err != nil {
			return err
		}

		msgSeq, err := tx.BumpMsgSeq(ctx, tenantID, messageID, data.StreamKey)
		if err != nil {
			return err
		}

		payload := data.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		persistPayload := payload
		if data.EventType == types.AgentGraphEventRunComplete {
			persistPayload = graphstream.SummarizeRunCompletePayload(payload)
		}

		payloadJSON, err := json.Marshal(persistPayload)
		if err != nil {
			return fmt.Errorf("marshal graph event payload: %w", err)
		}

		var emittedAt *time.Time
		if data.Timestamp > 0 {
			t := time.Unix(0, int64(data.Timestamp*float64(time.Second))).UTC()
			emittedAt = &t
		}

		inserted, err := tx.InsertEvent(ctx, &types.AgentGraphEvent{
			ID:        uuid.NewString(),
			TenantID:  tenantID,
			SessionID: sessionID,
			MessageID: messageID,
			StreamKey: data.StreamKey,
			Seq:       data.Seq,
			MsgSeq:    msgSeq,
			EventType: data.EventType,
			EventID:   data.EventID,
			Payload:   types.JSON(payloadJSON),
			EmittedAt: emittedAt,
		})
		if err != nil {
			return err
		}
		if !inserted {
			logger.Debugf(ctx, "[AgentGraph] duplicate event skipped stream_key=%s seq=%d", data.StreamKey, data.Seq)
			return nil
		}

		switch data.EventType {
		case types.AgentGraphEventEntitySearching:
			return s.handleEntitySearching(ctx, tx, tenantID, sessionID, messageID, msgSeq, data)
		case types.AgentGraphEventEntityConfirmed:
			return s.handleEntityConfirmed(ctx, tx, tenantID, sessionID, messageID, msgSeq, data)
		case types.AgentGraphEventRelationFound:
			return s.handleRelationFound(ctx, tx, tenantID, sessionID, messageID, msgSeq, data)
		case types.AgentGraphEventPhaseChange:
			return tx.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, map[string]interface{}{
				"phase": asString(data.Payload["phase"]),
			})
		case types.AgentGraphEventProgress:
			return tx.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, map[string]interface{}{
				"step":  asInt(data.Payload["step"]),
				"phase": asString(data.Payload["current_phase"]),
			})
		case types.AgentGraphEventLiteratureSearching:
			return tx.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, nil)
		case types.AgentGraphEventRunComplete:
			return s.handleRunComplete(ctx, tx, tenantID, sessionID, messageID, msgSeq, data)
		default:
			return tx.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, nil)
		}
	})
	if err != nil {
		logger.Errorf(ctx, "[AgentGraph] record failed stream_key=%s seq=%d type=%s err=%v",
			data.StreamKey, data.Seq, data.EventType, err)
		return err
	}
	if data.Seq == 1 || data.Seq%25 == 0 || data.EventType == types.AgentGraphEventRunComplete {
		logger.Infof(ctx, "[AgentGraph] recorded stream_key=%s seq=%d type=%s tenant=%d msg=%s",
			data.StreamKey, data.Seq, data.EventType, tenantID, messageID)
	}
	return nil
}

func (s *agentGraphService) GetSnapshot(
	ctx context.Context,
	tenantID uint64,
	sessionID, messageID string,
	afterSeq int64,
	include map[string]bool,
) (*types.AgentGraphSnapshot, error) {
	if include == nil || len(include) == 0 {
		include = map[string]bool{"nodes": true, "edges": true, "run": true}
	}

	snap := &types.AgentGraphSnapshot{
		Nodes:  []*types.AgentGraphNode{},
		Edges:  []*types.AgentGraphEdge{},
		Events: []*types.AgentGraphEvent{},
		LastSeq: afterSeq,
	}

	run, err := s.repo.GetLatestRunByMessage(ctx, tenantID, sessionID, messageID)
	if err != nil {
		return nil, err
	}
	if include["run"] {
		snap.Run = run
	}
	if run != nil && run.LastMsgSeq > snap.LastSeq {
		snap.LastSeq = run.LastMsgSeq
	}

	if include["nodes"] {
		nodes, err := s.repo.ListNodes(ctx, tenantID, sessionID, messageID, afterSeq)
		if err != nil {
			return nil, err
		}
		if nodes != nil {
			snap.Nodes = nodes
		}
		for _, n := range nodes {
			if n.LastMsgSeq > snap.LastSeq {
				snap.LastSeq = n.LastMsgSeq
			}
		}
	}

	if include["edges"] {
		edges, err := s.repo.ListEdges(ctx, tenantID, sessionID, messageID, afterSeq)
		if err != nil {
			return nil, err
		}
		if edges != nil {
			snap.Edges = edges
		}
		for _, e := range edges {
			if e.LastMsgSeq > snap.LastSeq {
				snap.LastSeq = e.LastMsgSeq
			}
		}
	}

	if include["events"] {
		events, err := s.repo.ListEvents(ctx, tenantID, sessionID, messageID, afterSeq)
		if err != nil {
			return nil, err
		}
		if events != nil {
			snap.Events = events
		}
		for _, e := range events {
			if e.MsgSeq > snap.LastSeq {
				snap.LastSeq = e.MsgSeq
			}
		}
	}

	return snap, nil
}

func (s *agentGraphService) ensureRun(
	ctx context.Context, repo interfaces.AgentGraphRepository,
	tenantID uint64, sessionID, messageID string, data event.AgentGraphData,
) error {
	existing, err := repo.GetRunByStreamKey(ctx, tenantID, data.StreamKey)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	return repo.UpsertRun(ctx, &types.AgentGraphRun{
		ID:         uuid.NewString(),
		TenantID:   tenantID,
		SessionID:  sessionID,
		MessageID:  messageID,
		StreamKey:  data.StreamKey,
		ToolCallID: data.ToolCallID,
		Status:     types.AgentGraphRunStatusRunning,
	})
}

func (s *agentGraphService) handleEntitySearching(
	ctx context.Context, repo interfaces.AgentGraphRepository,
	tenantID uint64, sessionID, messageID string, msgSeq int64, data event.AgentGraphData,
) error {
	name := asString(data.Payload["entity_name"])
	if name == "" {
		return repo.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, nil)
	}
	if err := repo.UpsertNode(ctx, &types.AgentGraphNode{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		SessionID:    sessionID,
		MessageID:    messageID,
		StreamKey:    data.StreamKey,
		EntityName:   name,
		EntityType: normalizeEntityType(firstNonEmpty(
			asString(data.Payload["entity_type"]), asString(data.Payload["entityType"]),
		)),
		Status:       types.AgentGraphNodeStatusSearching,
		SourceKB:     asString(data.Payload["source_kb"]),
		Observations: types.JSON([]byte("[]")),
		FirstSeq:     data.Seq,
		LastSeq:      data.Seq,
		FirstMsgSeq:  msgSeq,
		LastMsgSeq:   msgSeq,
	}, false); err != nil {
		return err
	}
	return repo.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, nil)
}

func (s *agentGraphService) handleEntityConfirmed(
	ctx context.Context, repo interfaces.AgentGraphRepository,
	tenantID uint64, sessionID, messageID string, msgSeq int64, data event.AgentGraphData,
) error {
	name := asString(data.Payload["entity_name"])
	if name == "" {
		return repo.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, nil)
	}
	obs := asStringSlice(data.Payload["observations"])
	if obs == nil {
		obs = []string{}
	}
	obsJSON, _ := json.Marshal(obs)
	if err := repo.UpsertNode(ctx, &types.AgentGraphNode{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		SessionID:    sessionID,
		MessageID:    messageID,
		StreamKey:    data.StreamKey,
		EntityName:   name,
		EntityType:   normalizeEntityType(firstNonEmpty(
			asString(data.Payload["entity_type"]), asString(data.Payload["entityType"]),
		)),
		Status:       types.AgentGraphNodeStatusConfirmed,
		Observations: types.JSON(obsJSON),
		FirstSeq:     data.Seq,
		LastSeq:      data.Seq,
		FirstMsgSeq:  msgSeq,
		LastMsgSeq:   msgSeq,
	}, true); err != nil {
		return err
	}
	count, err := repo.CountConfirmedNodes(ctx, tenantID, messageID)
	if err != nil {
		return err
	}
	return repo.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, map[string]interface{}{
		"entity_count": count,
	})
}

func (s *agentGraphService) handleRelationFound(
	ctx context.Context, repo interfaces.AgentGraphRepository,
	tenantID uint64, sessionID, messageID string, msgSeq int64, data event.AgentGraphData,
) error {
	src := asString(data.Payload["source_entity"])
	tgt := asString(data.Payload["target_entity"])
	if src == "" || tgt == "" {
		return repo.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, nil)
	}
	if err := repo.UpsertEdge(ctx, &types.AgentGraphEdge{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		SessionID:    sessionID,
		MessageID:    messageID,
		StreamKey:    data.StreamKey,
		SourceEntity: src,
		TargetEntity: tgt,
		RelationType: asString(data.Payload["relation_type"]),
		FirstSeq:     data.Seq,
		LastSeq:      data.Seq,
		FirstMsgSeq:  msgSeq,
		LastMsgSeq:   msgSeq,
	}); err != nil {
		return err
	}
	count, err := repo.CountEdges(ctx, tenantID, messageID)
	if err != nil {
		return err
	}
	return repo.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, map[string]interface{}{
		"relation_count": count,
	})
}

func (s *agentGraphService) handleRunComplete(
	ctx context.Context, repo interfaces.AgentGraphRepository,
	tenantID uint64, sessionID, messageID string, msgSeq int64, data event.AgentGraphData,
) error {
	if err := s.reconcileSnapshot(ctx, repo, tenantID, sessionID, messageID, msgSeq, data); err != nil {
		return err
	}

	entityCount, err := repo.CountConfirmedNodes(ctx, tenantID, messageID)
	if err != nil {
		return err
	}
	relationCount, err := repo.CountEdges(ctx, tenantID, messageID)
	if err != nil {
		return err
	}
	now := time.Now()
	extra := map[string]interface{}{
		"status":         types.AgentGraphRunStatusCompleted,
		"completed_at":   now,
		"entity_count":   entityCount,
		"relation_count": relationCount,
	}
	if v, ok := data.Payload["duration_seconds"]; ok {
		if d := asFloat64(v); d > 0 {
			extra["duration_seconds"] = d
		}
	}
	if v, ok := data.Payload["total_steps"]; ok {
		extra["step"] = asInt(v)
	}
	return repo.AdvanceRunSeq(ctx, tenantID, data.StreamKey, data.Seq, msgSeq, extra)
}

func (s *agentGraphService) reconcileSnapshot(
	ctx context.Context, repo interfaces.AgentGraphRepository,
	tenantID uint64, sessionID, messageID string, msgSeq int64, data event.AgentGraphData,
) error {
	entities, _ := data.Payload["entities"].([]any)
	for _, raw := range entities {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := asString(m["entity_name"])
		if name == "" {
			name = asString(m["name"])
		}
		if name == "" {
			continue
		}
		obs := asStringSlice(m["observations"])
		if obs == nil {
			obs = []string{}
		}
		obsJSON, _ := json.Marshal(obs)
		if err := repo.UpsertNodeReconcile(ctx, &types.AgentGraphNode{
			ID:           uuid.NewString(),
			TenantID:     tenantID,
			SessionID:    sessionID,
			MessageID:    messageID,
			StreamKey:    data.StreamKey,
			EntityName:   name,
			// BioDSA memory-graph snapshot uses camelCase entityType / GENE-style values.
			EntityType:   normalizeEntityType(firstNonEmpty(
				asString(m["entity_type"]), asString(m["entityType"]), asString(m["type"]),
			)),
			Status:       types.AgentGraphNodeStatusConfirmed,
			Observations: types.JSON(obsJSON),
			FirstSeq:     data.Seq,
			LastSeq:      data.Seq,
			FirstMsgSeq:  msgSeq,
			LastMsgSeq:   msgSeq,
		}); err != nil {
			return err
		}
	}

	relations, _ := data.Payload["relations"].([]any)
	for _, raw := range relations {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		src := firstNonEmpty(
			asString(m["source_entity"]), asString(m["source"]), asString(m["from"]),
		)
		tgt := firstNonEmpty(
			asString(m["target_entity"]), asString(m["target"]), asString(m["to"]),
		)
		if src == "" || tgt == "" {
			continue
		}
		if err := repo.UpsertEdgeReconcile(ctx, &types.AgentGraphEdge{
			ID:           uuid.NewString(),
			TenantID:     tenantID,
			SessionID:    sessionID,
			MessageID:    messageID,
			StreamKey:    data.StreamKey,
			SourceEntity: src,
			TargetEntity: tgt,
			RelationType: firstNonEmpty(
				asString(m["relation_type"]), asString(m["relationType"]), asString(m["type"]),
			),
			FirstSeq:     data.Seq,
			LastSeq:      data.Seq,
			FirstMsgSeq:  msgSeq,
			LastMsgSeq:   msgSeq,
		}); err != nil {
			return err
		}
	}
	return nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
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
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func asStringSlice(v any) []string {
	switch arr := v.(type) {
	case []string:
		return arr
	case []any:
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// normalizeEntityType mirrors BioDSA ENTITY_TYPE_NORMALIZE so GET nodes and
// SSE Confirmed payloads share one lowercase vocabulary for frontend coloring.
func normalizeEntityType(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	switch strings.ToUpper(raw) {
	case "GENE", "PROTEIN":
		return "gene"
	case "VARIANT":
		return "variant"
	case "DRUG":
		return "drug"
	case "CHEMICAL":
		return "compound"
	case "DISEASE", "PHENOTYPE":
		return "disease"
	case "PATHWAY", "GENE_SET":
		return "pathway"
	case "PAPER":
		return "literature"
	case "FINDING":
		return "finding"
	case "CELL_LINE":
		return "cell_line"
	case "TISSUE":
		return "tissue"
	case "TARGET":
		return "target"
	case "COMPOUND":
		return "compound"
	default:
		return strings.ToLower(raw)
	}
}
