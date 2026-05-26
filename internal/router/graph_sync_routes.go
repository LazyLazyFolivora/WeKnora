package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterGraphSyncRoutes registers batch graph entity and relation upsert routes.
//
// Permission model — these are KB-scoped write operations, matching the
// same "creator-of-the-KB OR Admin+" matrix used by other KB mutations.
//
// Routes:
//
//	POST /api/v1/knowledge-bases/:id/graph/entities:batch-upsert
//	POST /api/v1/knowledge-bases/:id/graph/relations:batch-upsert
func RegisterGraphSyncRoutes(r *gin.RouterGroup, h *handler.GraphSyncHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	r.POST("/knowledge-bases/:id/graph/entities/batch-upsert",
		g.OwnedKBOrAdmin(),
		g.KBAccessWrite("id"),
		h.BatchUpsertEntities,
	)
	r.POST("/knowledge-bases/:id/graph/relations/batch-upsert",
		g.OwnedKBOrAdmin(),
		g.KBAccessWrite("id"),
		h.BatchUpsertRelations,
	)
}
