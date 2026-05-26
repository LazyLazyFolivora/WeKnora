package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterGraphImportRoutes 注册手动导入知识图谱数据的路由。
//
// 权限设计：
//   - OwnedKBOrAdmin：只有 KB 创建者本人或 Admin+ 才能向图谱写入数据，
//     与其他 KB 写操作（上传文档、删除内容等）保持一致。
//   - KBAccessWrite：确保共享 KB 场景下也能正确解析有效租户上下文。
//
// 路由：POST /api/v1/knowledge-bases/:id/graph/import
func RegisterGraphImportRoutes(r *gin.RouterGroup, h *handler.GraphImportHandler, g *rbacGuards) {
	if h == nil {
		return
	}
	r.POST("/knowledge-bases/:id/graph/import",
		g.OwnedKBOrAdmin(),
		g.KBAccessWrite("id"),
		h.ImportGraph,
	)
}
