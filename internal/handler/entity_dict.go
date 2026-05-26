package handler

import (
	"net/http"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// EntityDictHandler handles entity_dict sync and PrimeKG copy init.
type EntityDictHandler struct {
	svc interfaces.EntityDictService
}

// NewEntityDictHandler creates a new EntityDictHandler.
func NewEntityDictHandler(svc interfaces.EntityDictService) *EntityDictHandler {
	return &EntityDictHandler{svc: svc}
}

// BatchUpsert syncs entity_dict rows from ZK into WeKnora's mirror table.
// @Summary      批量同步 entity_dict 行
// @Description  ZK 侧将 entity_dict 的数据推送到 WeKnora 镜像表
// @Tags         知识图谱
// @Accept       json
// @Produce      json
// @Param        request  body      types.EntityDictBatchUpsertRequest  true  "entity_dict 行列表"
// @Success      200      {object}  map[string]interface{}              "同步成功"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /entity-dict/batch-upsert [post]
func (h *EntityDictHandler) BatchUpsert(c *gin.Context) {
	var req types.EntityDictBatchUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("请求参数解析失败").WithDetails(err.Error()))
		return
	}

	n, err := h.svc.BatchUpsert(c.Request.Context(), req.Rows)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"upserted": n},
	})
}

// InitCopies reads entity_dict and creates graph_entities rows for PrimeKG copies.
// @Summary      初始化 PrimeKG 副本实体
// @Description  从 entity_dict 读取 canonical_source=primekg 的行，写入 graph_entities 并设 source_site=primekg
// @Tags         知识图谱
// @Produce      json
// @Param        kbId  path      string  true  "目标知识库 ID"
// @Success      200    {object}  map[string]interface{}  "初始化完成"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /entity-dict/{kbId}/init-copies [post]
func (h *EntityDictHandler) InitCopies(c *gin.Context) {
	kbID := c.Param("kbId")
	tenantID := c.GetUint64("tenant_id")

	n, err := h.svc.InitCopies(c.Request.Context(), kbID, tenantID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"upserted": n},
	})
}
