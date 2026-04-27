package middleware

import (
	"strings"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

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
	secure := c.Request.TLS != nil
	c.SetCookie("token", token, 86400*7, "/", "", secure, true)
}

func ClearAuthCookie(c *gin.Context) {
	secure := c.Request.TLS != nil
	c.SetCookie("token", "", -1, "/", "", secure, true)
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
