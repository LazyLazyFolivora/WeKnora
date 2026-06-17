package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// GetCASConfig godoc
// @Summary      获取CAS登录配置
// @Description  返回CAS是否启用以及provider展示名称，供前端决定是否展示CAS登录入口
// @Tags         认证
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /auth/cas/config [get]
func (h *AuthHandler) GetCASConfig(c *gin.Context) {
	providerDisplayName := ""
	enabled := false
	if h.configInfo != nil && h.configInfo.CASAuth != nil {
		enabled = h.configInfo.CASAuth.Enable
		providerDisplayName = strings.TrimSpace(h.configInfo.CASAuth.ProviderDisplayName)
	}
	if providerDisplayName == "" {
		providerDisplayName = "CAS"
	}
	c.JSON(http.StatusOK, &types.OIDCConfigResponse{
		Success:             true,
		Enabled:             enabled,
		ProviderDisplayName: providerDisplayName,
	})
}

// GetCASLoginURL godoc
// @Summary      获取CAS登录跳转地址
// @Description  根据后端CAS配置生成CAS登录页面跳转地址
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        redirect_uri  query     string  true  "CAS回调地址"
// @Success      200           {object}  types.CASAuthURLResponse
// @Failure      400           {object}  errors.AppError  "请求参数错误"
// @Failure      403           {object}  errors.AppError  "CAS未启用"
// @Router       /auth/cas/url [get]
func (h *AuthHandler) GetCASLoginURL(c *gin.Context) {
	ctx := c.Request.Context()
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	if redirectURI == "" {
		appErr := errors.NewValidationError("redirect_uri is required")
		c.Error(appErr)
		return
	}

	if h.configInfo == nil || h.configInfo.CASAuth == nil || !h.configInfo.CASAuth.Enable {
		appErr := errors.NewForbiddenError("CAS authentication is not enabled")
		c.Error(appErr)
		return
	}

	cfg := h.configInfo.CASAuth
	serverURL := strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	if serverURL == "" {
		appErr := errors.NewForbiddenError("CAS server URL is not configured")
		c.Error(appErr)
		return
	}

	displayName := strings.TrimSpace(cfg.ProviderDisplayName)
	if displayName == "" {
		displayName = "CAS"
	}

	loginURL := fmt.Sprintf("%s/login?service=%s", serverURL, urlQueryEscape(redirectURI))

	logger.Infof(ctx, "[CAS] Built login URL: %s", loginURL)

	c.JSON(http.StatusOK, &types.CASAuthURLResponse{
		Success:             true,
		ProviderDisplayName: displayName,
		AuthorizationURL:    loginURL,
	})
}

// CASCallback godoc
// @Summary      CAS登录回调
// @Description  接收CAS服务器回调，验证ticket并完成登录，随后重定向回前端登录页
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        ticket  query string false "CAS ticket"
// @Success      302
// @Router       /auth/cas/callback [get]
func (h *AuthHandler) CASCallback(c *gin.Context) {
	ctx := c.Request.Context()
	frontendRedirectURI := "/"

	ticket := strings.TrimSpace(c.Query("ticket"))
	if ticket == "" {
		c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_error="+urlQueryEscape("missing_ticket"))
		return
	}

	// Build the service URL that CAS validated against — this is the
	// callback URL as the CAS server sees it (the same URL minus the
	// ticket parameter).
	callbackURL := c.Request.URL
	callbackURL.RawQuery = ""
	serviceURL := callbackURL.String()

	resp, err := h.userService.LoginWithCAS(ctx, ticket, serviceURL)
	if err != nil {
		logger.Errorf(ctx, "Failed to complete CAS login via callback: %v", err)
		c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_error="+urlQueryEscape("login_failed")+"&oidc_error_description="+urlQueryEscape(err.Error()))
		return
	}
	if !resp.Success {
		c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_error="+urlQueryEscape("login_failed")+"&oidc_error_description="+urlQueryEscape(resp.Message))
		return
	}

	payload, err := encodeOIDCCallbackPayload(resp)
	if err != nil {
		logger.Errorf(ctx, "Failed to encode CAS callback payload: %v", err)
		c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_error="+urlQueryEscape("payload_encode_failed"))
		return
	}

	c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_result="+urlQueryEscape(payload))
}
