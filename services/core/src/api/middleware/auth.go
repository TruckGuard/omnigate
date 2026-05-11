package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permsHeader := c.GetHeader("X-Permissions")
		if permsHeader == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden: No permissions"})
			return
		}

		perms := strings.Split(permsHeader, ",")
		hasPerm := false
		for _, p := range perms {
			if strings.TrimSpace(p) == permission {
				hasPerm = true
				break
			}
		}

		if !hasPerm {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden: Missing permission"})
			return
		}

		c.Next()
	}
}
