package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

func (s *userService) getCASSettings() (*types.CASSettings, error) {
	if s.config == nil || s.config.CASAuth == nil || !s.config.CASAuth.Enable {
		return nil, fmt.Errorf("CAS authentication is not enabled")
	}
	cfg := s.config.CASAuth
	serverURL := strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	if serverURL == "" {
		return nil, fmt.Errorf("CAS server URL is not configured")
	}
	callbackURL := strings.TrimSpace(cfg.CallbackURL)
	if callbackURL == "" {
		return nil, fmt.Errorf("CAS callback URL is not configured")
	}
	displayName := strings.TrimSpace(cfg.ProviderDisplayName)
	if displayName == "" {
		displayName = "CAS"
	}
	return &types.CASSettings{
		Enabled:              cfg.Enable,
		ServerURL:            serverURL,
		ProviderDisplayName:  displayName,
		CallbackURL:          callbackURL,
		UsernameAttribute:    strings.TrimSpace(cfg.UsernameAttribute),
		EmailAttribute:       strings.TrimSpace(cfg.EmailAttribute),
		DisplayNameAttribute: strings.TrimSpace(cfg.DisplayNameAttribute),
	}, nil
}

// LoginWithCAS validates a CAS ticket and completes the login flow.
// On success it returns the same response shape as LoginWithOIDC so the
// frontend callback handler can process both identically.
func (s *userService) LoginWithCAS(ctx context.Context, ticket, redirectURI string) (*types.OIDCCallbackResponse, error) {
	if strings.TrimSpace(ticket) == "" {
		return nil, fmt.Errorf("ticket is required")
	}
	if strings.TrimSpace(redirectURI) == "" {
		return nil, fmt.Errorf("redirect_uri is required")
	}

	settings, err := s.getCASSettings()
	if err != nil {
		return nil, err
	}

	info, err := s.validateCASTicket(ctx, settings, ticket, redirectURI)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(info.Username) == "" {
		return nil, fmt.Errorf("CAS provider did not return a username")
	}

	user, err := s.userRepo.GetUserByUsername(ctx, info.Username)
	if err != nil && !isUserLookupNotFound(err) {
		return nil, fmt.Errorf("failed to query user by username: %w", err)
	}
	isNewUser := false
	if isUserLookupNotFound(err) || user == nil {
		user, err = s.provisionCASUser(ctx, info)
		if err != nil {
			return nil, err
		}
		isNewUser = true
	}

	if !user.IsActive {
		return &types.OIDCCallbackResponse{Success: false, Message: "Account is disabled"}, nil
	}

	resolvedTenantID := s.resolveLoginTenantID(ctx, user)
	accessToken, refreshToken, err := s.generateTokensForTenant(ctx, user, resolvedTenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate local tokens: %w", err)
	}

	var tenant *types.Tenant
	if resolvedTenantID > 0 {
		if t, terr := s.tenantService.GetTenantByID(ctx, resolvedTenantID); terr == nil {
			tenant = t
		} else {
			logger.Warnf(ctx, "CAS login: failed to load tenant %d for user %s: %v",
				resolvedTenantID, user.ID, terr)
		}
	}
	memberships := s.buildMembershipsForUser(ctx, user, tenant)

	return &types.OIDCCallbackResponse{
		Success:      true,
		Message:      "登录成功",
		User:         user,
		Tenant:       tenant,
		Memberships:  memberships,
		Token:        accessToken,
		RefreshToken: refreshToken,
		IsNewUser:    isNewUser,
	}, nil
}

// validateCASTicket sends a serviceValidate request to the CAS server and
// returns the parsed user info.
func (s *userService) validateCASTicket(ctx context.Context, settings *types.CASSettings, ticket, serviceURL string) (*types.CASUserInfo, error) {
	validateURL := settings.ServerURL + "/p3/serviceValidate?" + url.Values{
		"service": {serviceURL},
		"ticket":  {ticket},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create CAS validate request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate CAS ticket: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB
	if err != nil {
		return nil, fmt.Errorf("failed to read CAS response: %w", err)
	}

	info, failure := types.ParseCASServiceResponse(body,
		settings.UsernameAttribute,
		settings.DisplayNameAttribute,
		settings.EmailAttribute,
	)
	if info == nil {
		return nil, fmt.Errorf("CAS authentication failed: %s", failure)
	}

	return info, nil
}

// provisionCASUser creates a new user from CAS user info.
func (s *userService) provisionCASUser(ctx context.Context, info *types.CASUserInfo) (*types.User, error) {
	username := s.generateCASUsername(ctx, info)
	randomPassword, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password for CAS user: %w", err)
	}

	email := info.Email
	if email == "" {
		email = fmt.Sprintf("%s@cas.school", username)
	}

	user, err := s.Register(ctx, &types.RegisterRequest{
		Username: username,
		Email:    email,
		Password: randomPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-provision CAS user: %w", err)
	}
	return user, nil
}

func (s *userService) generateCASUsername(ctx context.Context, info *types.CASUserInfo) string {
	base := sanitizeUsernameCandidate(info.Username)
	if base == "" {
		base = "cas-user"
	}

	candidate := base
	for i := 0; i < 20; i++ {
		existing, err := s.userRepo.GetUserByUsername(ctx, candidate)
		if isUserLookupNotFound(err) || (err == nil && existing == nil) {
			return candidate
		}
		if err != nil && !isUserLookupNotFound(err) {
			logger.Warnf(ctx, "Failed to check existing CAS username %q: %v", candidate, err)
		}
		candidate = fmt.Sprintf("%s-%d", base, i+1)
	}
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}
