package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterEntityDictRoutes registers entity_dict sync and init routes.
//
// Routes:
//
//	POST /api/v1/entity-dict/batch-upsert      — sync entity_dict rows from ZK
//	POST /api/v1/entity-dict/init-copies        — init PrimeKG copies into graph_entities
func RegisterEntityDictRoutes(r *gin.RouterGroup, h *handler.EntityDictHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	r.POST("/entity-dict/batch-upsert", g.Admin(), h.BatchUpsert)
	r.POST("/entity-dict/init-copies", g.Admin(), h.InitCopies)
}
