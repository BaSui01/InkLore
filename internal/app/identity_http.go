package app

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

func (a *App) registerIdentityRoutes(engine *gin.Engine) {
	service := a.platform.Identity
	engine.GET("/v1/auth/captcha", func(c *gin.Context) {
		if a.platform.Captcha == nil {
			response.Fail(c, platformerr.Contract(503, "platform.dependency_unavailable", nil))
			return
		}
		id, svg, err := a.platform.Captcha.Generate()
		if err != nil {
			response.Fail(c, platformerr.Contract(500, "platform.internal_error", nil))
			return
		}
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		response.OK(c, 200, gin.H{
			"captcha_id": id,
			"svg":        svg,
			"expires_in": 180,
		})
	})
	engine.POST("/v1/auth/captcha/verify", func(c *gin.Context) {
		var input struct {
			CaptchaID   string `json:"captcha_id"`
			CaptchaCode string `json:"captcha_code"`
		}
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		if a.platform.Captcha == nil || !a.platform.Captcha.Verify(input.CaptchaID, input.CaptchaCode) {
			response.Fail(c, platformerr.Contract(400, "platform.validation_failed", map[string]any{
				"field":  "captcha_code",
				"reason": "invalid_or_expired",
			}))
			return
		}
		response.OK(c, 200, gin.H{"valid": true})
	})
	engine.POST("/v1/auth/login", func(c *gin.Context) {
		var input struct {
			identity.LoginInput
			CaptchaID   string `json:"captcha_id"`
			CaptchaCode string `json:"captcha_code"`
		}
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		if input.CaptchaID != "" || input.CaptchaCode != "" {
			if a.platform.Captcha == nil || !a.platform.Captcha.Verify(input.CaptchaID, input.CaptchaCode) {
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", map[string]any{
					"field":  "captcha_code",
					"reason": "invalid_or_expired",
				}))
				return
			}
		}
		if c.GetHeader("Origin") != "" && !a.sessionOrigin(c, input.Audience, false) {
			return
		}
		input.ClientIP = c.ClientIP()
		result, err := service.Login(c.Request.Context(), input.LoginInput)
		if err != nil {
			response.Fail(c, err)
			return
		}
		a.setRefreshCookie(c, result.Principal.Audience, result.RefreshToken, result.RefreshExpiresAt)
		response.OK(c, 200, result)
	})
	engine.POST("/v1/auth/switch-tenant", func(c *gin.Context) {
		var input struct {
			TenantID string `json:"tenant_id"`
		}
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		if !a.sessionOrigin(c, "admin", true) {
			return
		}
		csrf := c.GetHeader("X-CSRF-Token")
		if csrf == "" || len(csrf) > 128 {
			response.Fail(c, platformerr.Contract(403, "auth.forbidden", nil))
			return
		}
		principal, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		result, err := service.SwitchTenant(c.Request.Context(), principal, csrf, input.TenantID)
		if err != nil {
			response.Fail(c, err)
			return
		}
		a.setRefreshCookie(c, "admin", result.RefreshToken, result.RefreshExpiresAt)
		response.OK(c, 200, result)
	})
	engine.POST("/v1/auth/register", func(c *gin.Context) {
		var input struct {
			identity.RegisterInput
			CaptchaID   string `json:"captcha_id"`
			CaptchaCode string `json:"captcha_code"`
		}
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		if input.CaptchaID != "" || input.CaptchaCode != "" {
			if a.platform.Captcha == nil || !a.platform.Captcha.Verify(input.CaptchaID, input.CaptchaCode) {
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", map[string]any{
					"field":  "captcha_code",
					"reason": "invalid_or_expired",
				}))
				return
			}
		}
		if c.GetHeader("Origin") != "" && !a.sessionOrigin(c, "creator", false) {
			return
		}
		input.ClientIP = c.ClientIP()
		result, err := service.RegisterCreator(c.Request.Context(), input.RegisterInput)
		if err != nil {
			response.Fail(c, err)
			return
		}
		a.setRefreshCookie(c, "creator", result.RefreshToken, result.RefreshExpiresAt)
		response.OK(c, 200, result)
	})
	engine.POST("/v1/auth/forgot-password", func(c *gin.Context) {
		var input struct {
			Email       string `json:"email"`
			CaptchaID   string `json:"captcha_id"`
			CaptchaCode string `json:"captcha_code"`
		}
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		if input.CaptchaID != "" || input.CaptchaCode != "" {
			if a.platform.Captcha == nil || !a.platform.Captcha.Verify(input.CaptchaID, input.CaptchaCode) {
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", map[string]any{
					"field":  "captcha_code",
					"reason": "invalid_or_expired",
				}))
				return
			}
		}
		if !a.config.IsDevelopment() {
			response.Fail(c, platformerr.Contract(503, "platform.dependency_unavailable", nil))
			return
		}
		c.Header("Cache-Control", "no-store")
		token, err := service.ForgotPassword(c.Request.Context(), input.Email)
		if err != nil {
			response.Fail(c, err)
			return
		}
		out := map[string]any{
			"sent":       false,
			"expires_in": 900,
		}
		if a.config.IsDevelopment() && token != "" {
			out["dev_token"] = token
		}
		response.OK(c, 200, out)
	})
	engine.POST("/v1/auth/reset-password", func(c *gin.Context) {
		var input struct {
			Token       string `json:"token"`
			NewPassword string `json:"new_password"`
		}
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		if err := service.ResetPassword(c.Request.Context(), input.Token, input.NewPassword); err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, map[string]any{"reset": true})
	})
	engine.GET("/v1/auth/session-csrf", func(c *gin.Context) {
		values := c.Request.URL.Query()["audience"]
		if len(values) != 1 || !a.sessionOrigin(c, values[0], true) {
			if !c.IsAborted() {
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
			}
			return
		}
		cookie, err := c.Cookie(refreshCookieName(values[0]))
		if err != nil {
			response.Fail(c, platformerr.Contract(401, "auth.unauthenticated", nil))
			return
		}
		result, err := service.SessionCSRF(c.Request.Context(), cookie, values[0])
		if err != nil {
			response.Fail(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		response.OK(c, 200, result)
	})
	for _, operation := range []string{"refresh", "logout"} {
		operation := operation
		engine.POST("/v1/auth/"+operation, func(c *gin.Context) {
			var input struct {
				Audience string `json:"audience"`
			}
			if err := httpguard.DecodeJSON(c, &input); err != nil {
				response.Fail(c, err)
				return
			}
			if !a.sessionOrigin(c, input.Audience, true) {
				return
			}
			csrf := c.GetHeader("X-CSRF-Token")
			if csrf == "" || len(csrf) > 128 {
				response.Fail(c, platformerr.Contract(403, "auth.forbidden", nil))
				return
			}
			cookie, err := c.Cookie(refreshCookieName(input.Audience))
			if err != nil && operation != "logout" {
				response.Fail(c, platformerr.Contract(401, "auth.unauthenticated", nil))
				return
			}
			if operation == "logout" {
				result, err := service.Logout(c.Request.Context(), cookie, input.Audience, csrf)
				if err != nil {
					response.Fail(c, err)
					return
				}
				a.setRefreshCookie(c, input.Audience, "", time.Time{})
				response.OK(c, 200, result)
				return
			}
			result, err := service.Refresh(c.Request.Context(), cookie, input.Audience, csrf)
			if err != nil {
				a.setRefreshCookie(c, input.Audience, "", time.Time{})
				response.Fail(c, err)
				return
			}
			a.setRefreshCookie(c, input.Audience, result.RefreshToken, result.RefreshExpiresAt)
			response.OK(c, 200, result)
		})
	}
	engine.GET("/v1/auth/me", func(c *gin.Context) {
		p, ok := a.authenticate(c, "either")
		if !ok {
			return
		}
		response.OK(c, 200, p)
	})
	engine.POST("/v1/auth/ws-ticket", func(c *gin.Context) {
		var input struct {
			Audience string `json:"audience"`
		}
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		if input.Audience != "creator" && input.Audience != "admin" {
			response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
			return
		}
		p, ok := a.authenticate(c, input.Audience)
		if !ok {
			return
		}
		ticket, err := service.WSTicket(c.Request.Context(), p)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, ticket)
	})
}
func (a *App) authenticate(c *gin.Context, audience string) (identity.Principal, bool) {
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		response.Fail(c, platformerr.Contract(401, "auth.unauthenticated", nil))
		return identity.Principal{}, false
	}
	p, err := a.platform.Identity.Authenticate(c.Request.Context(), parts[1], audience)
	if err != nil {
		response.Fail(c, err)
		return identity.Principal{}, false
	}
	c.Header("Cache-Control", "no-store")
	// The tenant comes only from a signed token verified against the live session
	// and membership, never from request input. The HTTP logger reads this after
	// the handler finishes to associate safe operational metadata with the tenant.
	c.Request = c.Request.WithContext(logger.WithVerifiedTenant(c.Request.Context(), p.TenantID))
	return p, true
}
func refreshCookieName(audience string) string {
	if audience == "admin" {
		return "inklore_admin_refresh"
	}
	return "inklore_creator_refresh"
}
func (a *App) setRefreshCookie(c *gin.Context, audience, token string, expires time.Time) {
	cookie := &http.Cookie{Name: refreshCookieName(audience), Value: token, Path: "/v1/auth", HttpOnly: true, Secure: a.config.IsProduction(), SameSite: http.SameSiteStrictMode, Expires: expires.UTC()}
	if token == "" {
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0).UTC()
	}
	http.SetCookie(c.Writer, cookie)
	c.Header("Cache-Control", "no-store")
}
func (a *App) sessionOrigin(c *gin.Context, audience string, required bool) bool {
	origin := ""
	switch audience {
	case "creator":
		origin = a.platform.CreatorOrigin
	case "admin":
		origin = a.platform.AdminOrigin
	default:
		response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
		return false
	}
	actual := c.GetHeader("Origin")
	fetchSite := c.GetHeader("Sec-Fetch-Site")
	// Same-origin browser GETs often omit Origin. Bind recovery to the selected
	// application's Host or browser Referer as well as protected Fetch Metadata.
	if required && actual == "" && c.Request.Method == http.MethodGet && fetchSite == "same-origin" {
		expected, _ := url.Parse(origin)
		referer, _ := url.Parse(c.GetHeader("Referer"))
		hostMatches := expected != nil && isHostEquivalent(c.Request.Host, expected.Host)
		refererMatches := referer != nil && isOriginEquivalent(referer.Scheme+"://"+referer.Host, origin)
		if hostMatches || refererMatches {
			return true
		}
	}
	if (required && actual == "") || (actual != "" && !isOriginEquivalent(actual, origin)) || (fetchSite != "" && fetchSite != "same-origin" && fetchSite != "same-site") {
		response.Fail(c, platformerr.Contract(403, "auth.forbidden", nil))
		return false
	}
	return true
}

func isHostEquivalent(actual, expected string) bool {
	if actual == expected {
		return true
	}
	actH, actP, err1 := net.SplitHostPort(actual)
	if err1 != nil {
		actH = actual
		actP = ""
	}
	expH, expP, err2 := net.SplitHostPort(expected)
	if err2 != nil {
		expH = expected
		expP = ""
	}
	if actP != expP {
		return false
	}
	isLoopback := func(h string) bool {
		return h == "localhost" || h == "127.0.0.1" || h == "[::1]" || h == "::1"
	}
	return isLoopback(actH) && isLoopback(expH)
}

func isOriginEquivalent(actual, expected string) bool {
	if actual == expected {
		return true
	}
	uAct, err1 := url.Parse(actual)
	uExp, err2 := url.Parse(expected)
	if err1 != nil || err2 != nil {
		return false
	}
	if uAct.Scheme != uExp.Scheme {
		return false
	}
	return isHostEquivalent(uAct.Host, uExp.Host)
}
