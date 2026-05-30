package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterEntityDictRoutes registers entity_dict sync and init routes.
//
// Routes:
//
//	POST /api/v1/entity-dict:batch-upsert      — sync entity_dict rows from ZK
//	POST /api/v1/entity-dict/:kbId/init-copies  — init PrimeKG copies into graph_entities
func RegisterEntityDictRoutes(r *gin.RouterGroup, h *handler.EntityDictHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	// entity-dict:batch-upsert has no KB in the URL — it syncs the tenant-wide
	// entity_dict mirror table. Require Admin+ (tenant infrastructure gate).
	r.POST("/entity-dict:batch-upsert", g.Admin(), h.BatchUpsert)
	// init-copies writes PrimeKG copies into a specific KB's graph_entities.
	// The :kbId path param identifies the KB; ownership-or-Admin gate applies.
	r.POST("/entity-dict/:kbId/init-copies", g.OwnedKBOrAdminFromKbIDParam(), h.InitCopies)
}
