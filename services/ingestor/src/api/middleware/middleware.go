package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func RequirePermission(requiredPerm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permsHeader := c.GetHeader("X-Permissions")
		if permsHeader == "" {
			c.AbortWithStatusJSON(403, gin.H{"error": "No permissions provided"})
			return
		}

		perms := strings.Split(permsHeader, ",")
		for _, p := range perms {
			if p == requiredPerm {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(403, gin.H{"error": "Missing permission: " + requiredPerm})
	}
}
