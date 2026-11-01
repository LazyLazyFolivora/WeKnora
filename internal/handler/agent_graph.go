package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AgentGraphHandler serves incremental knowledge-graph snapshots for one assistant turn.
type AgentGraphHandler struct {
	service        interfaces.AgentGraphService
	messageService interfaces.MessageService
}

// NewAgentGraphHandler creates an AgentGraphHandler.
func NewAgentGraphHandler(
	service interfaces.AgentGraphService,
	messageService interfaces.MessageService,
) *AgentGraphHandler {
	return &AgentGraphHandler{
		service:        service,
		messageService: messageService,
	}
}

// GetGraph returns the incremental knowledge graph for one assistant message.
// @Summary      获取 Agent 流式知识图谱
// @Description  返回 DeepEvidence 等 MCP 流式工具在本轮回答中构建的知识图谱；运行中与运行后共用此接口。
// @Tags         会话
// @Produce      json
// @Param        id          path   string  true   "会话 ID"
// @Param        message_id  path   string  true   "助手消息 ID"
// @Param        after_seq   query  int64   false  "消息级增量游标（msg_seq），只返回更大的数据"
// @Param        include     query  string  false  "逗号分隔：run,nodes,edges,events；默认 run,nodes,edges"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id}/messages/{message_id}/graph [get]
func (h *AgentGraphHandler) GetGraph(c *gin.Context) {
	if h == nil || h.service == nil {
		c.Error(apperrors.NewInternalServerError("agent graph service unavailable"))
		return
	}

	sessionID := secutils.SanitizeForLog(c.Param("id"))
	messageID := secutils.SanitizeForLog(c.Param("message_id"))
	if sessionID == "" || messageID == "" {
		c.Error(apperrors.NewBadRequestError("session id and message id are required"))
		return
	}

	ctx := c.Request.Context()
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		c.Error(apperrors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// GetMessage enforces session ownership via loadSessionForRead.
	if _, err := h.messageService.GetMessage(ctx, sessionID, messageID); err != nil {
		if errors.Is(err, apperrors.ErrSessionNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.Error(apperrors.NewNotFoundError("session or message not found"))
			return
		}
		c.Error(err)
		return
	}

	var afterSeq int64
	if raw := c.Query("after_seq"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			c.Error(apperrors.NewBadRequestError("invalid after_seq"))
			return
		}
		afterSeq = n
	}

	include := parseInclude(c.Query("include"))
	snap, err := h.service.GetSnapshot(ctx, tenantID, sessionID, messageID, afterSeq, include)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    snap,
	})
}

func parseInclude(raw string) map[string]bool {
	if strings.TrimSpace(raw) == "" {
		return map[string]bool{"run": true, "nodes": true, "edges": true}
	}
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			out[part] = true
		}
	}
	return out
}
