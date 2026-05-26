package handler

import (
	"net/http"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// GraphSyncHandler handles batch graph entity and relation upsert requests.
type GraphSyncHandler struct {
	service interfaces.GraphSyncService
}

// NewGraphSyncHandler creates a new GraphSyncHandler.
func NewGraphSyncHandler(service interfaces.GraphSyncService) *GraphSyncHandler {
	return &GraphSyncHandler{service: service}
}

// BatchUpsertEntities handles batch entity upsert.
// @Summary      批量导入知识图谱实体
// @Description  将自定义实体数据批量写入指定知识库，写入后 sync_status = pending，需另行触发投影。
// @Tags         知识图谱
// @Accept       json
// @Produce      json
// @Param        id       path      string                            true  "知识库 ID"
// @Param        request  body      types.GraphEntityBatchUpsertRequest true  "实体列表"
// @Success      200      {object}  map[string]interface{}            "导入成功"
// @Failure      400      {object}  map[string]interface{}            "请求参数错误"
// @Failure      404      {object}  map[string]interface{}            "知识库不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/graph/entities/batch-upsert [post]
func (h *GraphSyncHandler) BatchUpsertEntities(c *gin.Context) {
	var req types.GraphEntityBatchUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("请求参数解析失败").WithDetails(err.Error()))
		return
	}

	result, err := h.service.BatchUpsertEntities(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// BatchUpsertRelations handles batch relation upsert.
// @Summary      批量导入知识图谱关系
// @Description  将自定义关系数据批量写入指定知识库，写入后 sync_status = pending，需另行触发投影。
// @Tags         知识图谱
// @Accept       json
// @Produce      json
// @Param        id       path      string                               true  "知识库 ID"
// @Param        request  body      types.GraphRelationBatchUpsertRequest true  "关系列表"
// @Success      200      {object}  map[string]interface{}               "导入成功"
// @Failure      400      {object}  map[string]interface{}               "请求参数错误"
// @Failure      404      {object}  map[string]interface{}               "知识库不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/graph/relations/batch-upsert [post]
func (h *GraphSyncHandler) BatchUpsertRelations(c *gin.Context) {
	var req types.GraphRelationBatchUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("请求参数解析失败").WithDetails(err.Error()))
		return
	}

	result, err := h.service.BatchUpsertRelations(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
