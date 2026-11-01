package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterGraphSyncRoutes registers batch graph entity and relation upsert routes.
//
// Routes:
//
//	POST /api/v1/graph/entities/batch-upsert
//	POST /api/v1/graph/relations/batch-upsert
func RegisterGraphSyncRoutes(r *gin.RouterGroup, h *handler.GraphSyncHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	r.POST("/graph/entities/batch-upsert",
		g.Contributor(),
		h.BatchUpsertEntities,
	)
	r.POST("/graph/relations/batch-upsert",
		g.Contributor(),
		h.BatchUpsertRelations,
	)
}
