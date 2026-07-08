package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterGraphImportRoutes 注册手动导入知识图谱数据的路由。
//
// 路由：POST /api/v1/graph/import
func RegisterGraphImportRoutes(r *gin.RouterGroup, h *handler.GraphImportHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	r.POST("/graph/import",
		g.Contributor(),
		h.ImportGraph,
	)
}
