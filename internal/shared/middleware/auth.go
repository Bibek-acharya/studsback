package middleware

import (
	"strings"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

func cookieDomain() string {
	if domain := strings.TrimSpace(config.AppConfig.CookieDomain); domain != "" {
		domain = strings.TrimPrefix(domain, ".")
		if strings.Contains(domain, ":") {
			// Drop any accidental port component so we never emit an invalid cookie domain.
			if host, _, found := strings.Cut(domain, ":"); found && host != "" {
				domain = host
			}
		}
		if domain == "localhost" || domain == "127.0.0.1" {
			return ""
		}
		return domain
	}
	return ""
}

func cookieSecure(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
		return true
	}
	return false
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}

		if token == "" {
			cookieToken, err := c.Cookie("token")
			if err == nil && cookieToken != "" {
				token = cookieToken
			}
		}

		if token == "" {
			response.Error(c, 401, "Authentication required")
			c.Abort()
			return
		}

		claims, err := utils.ValidateToken(token)
		if err != nil {
			response.Error(c, 401, "Invalid or expired token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)

		c.Next()
	}
}

func SetAuthCookie(c *gin.Context, token string) {
	secure := cookieSecure(c)

	c.SetCookie("token", token, 86400*7, "/", cookieDomain(), secure, true)
}

func ClearAuthCookie(c *gin.Context) {
	secure := cookieSecure(c)

	c.SetCookie("token", "", -1, "/", cookieDomain(), secure, true)
}

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			response.Error(c, 403, "Unauthorized access")
			c.Abort()
			return
		}

		role := userRole.(string)
		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				c.Next()
				return
			}
		}

		response.Error(c, 403, "Insufficient permissions")
		c.Abort()
	}
}
