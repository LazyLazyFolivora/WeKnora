package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAgentGraphRoutes exposes the incremental knowledge graph built by
// streaming MCP agents during one assistant turn.
//
// Route:
//
//	GET /api/v1/sessions/:id/messages/:message_id/graph
func RegisterAgentGraphRoutes(r *gin.RouterGroup, h *handler.AgentGraphHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	// Wildcard names must be :id and :message_id — gin requires identical
	// names within the same HTTP-method radix tree (see routes_chat.go suggestions).
	// Declare chat/full-access so API keys are not fail-closed on this path.
	sessions := g.apiKeyGroup(r.Group("/sessions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	sessions.GET("/:id/messages/:message_id/graph", h.GetGraph)
}
