package handler

import (
	"net/http"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type GraphImportHandler struct {
	service interfaces.GraphImportService
}

func NewGraphImportHandler(service interfaces.GraphImportService) *GraphImportHandler {
	return &GraphImportHandler{service: service}
}

// ImportGraph godoc
// @Summary      手动导入知识图谱数据
// @Description  将自定义实体和关系数据直接写入指定知识库的 Neo4j 图谱，绕过 LLM 提取流程。
// @Description  节点和关系均为幂等写入（MERGE 语义），重复调用不会产生重复节点。
// @Tags         知识图谱
// @Accept       json
// @Produce      json
// @Param        id       path      string                   true  "知识库 ID"
// @Param        request  body      types.GraphImportRequest true  "要导入的节点和关系"
// @Success      200      {object}  map[string]interface{}   "导入成功"
// @Failure      400      {object}  map[string]interface{}   "请求参数错误"
// @Failure      403      {object}  map[string]interface{}   "无权限"
// @Failure      404      {object}  map[string]interface{}   "知识库不存在"
// @Failure      503      {object}  map[string]interface{}   "图数据库不可用"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/graph/import [post]
func (h *GraphImportHandler) ImportGraph(c *gin.Context) {
	var req types.GraphImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("请求参数解析失败").WithDetails(err.Error()))
		return
	}

	result, err := h.service.ImportGraph(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
