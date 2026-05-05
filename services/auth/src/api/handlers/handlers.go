package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"log/slog"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/omnigate/services/auth/src/models"
	"github.com/omnigate/services/auth/src/repository"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"
)

func HandleRegister(c *gin.Context) {
	var b struct {
		User string `json:"username"`
		Pass string `json:"password"`
		Role string `json:"role"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	h, _ := bcrypt.GenerateFromPassword([]byte(b.Pass), 10)

	var role models.Role
	roleName := b.Role
	if roleName == "" {
		roleName = "operator"
	}
	repository.DB.WithContext(c.Request.Context()).Where("name = ?", roleName).First(&role)

	u := models.User{Username: b.User, PasswordHash: string(h), RoleID: role.ID}
	if err := repository.DB.WithContext(c.Request.Context()).Preload("Role").Preload("Role.Permissions").Create(&u).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			slog.Warn("Registration failed: user already exists", "username", b.User)
			c.JSON(409, gin.H{"error": "User already exists"})
			return
		}
		slog.Error("Registration failed", "username", b.User, "error", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	slog.Info("User registered", "username", u.Username, "role", roleName)
	c.JSON(201, u)
}

func HandleLogin(c *gin.Context) {
	var b struct {
		User string `json:"username"`
		Pass string `json:"password"`
		IP   string `json:"ip"`
		UA   string `json:"user_agent"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	var u models.User
	if err := repository.DB.WithContext(c.Request.Context()).Preload("Role.Permissions").Where("username = ?", b.User).First(&u).Error; err != nil {
		slog.Warn("Login failed: user not found", "username", b.User)
		c.Status(401)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(b.Pass)); err != nil {
		slog.Warn("Login failed: invalid password", "username", b.User)
		c.Status(401)
		return
	}

	now := time.Now()
	u.LastLogin = &now
	repository.DB.WithContext(c.Request.Context()).Save(&u)

	// Use forwarded IP/UA if provided, otherwise fallback to request headers
	ip := b.IP
	if ip == "" {
		ip = c.ClientIP()
	}
	ua := b.UA
	if ua == "" {
		ua = c.GetHeader("User-Agent")
	}

	sessionID, err := repository.CreateSession(u.ID, u.Username, u.Role.Name, ip, ua)
	if err != nil {
		slog.Error("Session creation failed", "error", err)
		c.Status(500)
		return
	}

	slog.Info("User logged in", "username", u.Username, "user_id", u.ID)
	c.JSON(200, gin.H{"session_id": sessionID})
}

func HandleValidate(c *gin.Context) {
	origURI := c.GetHeader("X-Original-URI")
	origMethod := c.GetHeader("X-Original-Method")

	var perms []string
	var userID string
	var sessionID string
	var sourceID string
	var sourceName string
	var username string
	var role string
	var gateID string

	// 1. Check API Key
	k := c.GetHeader("X-API-Key")

	// 1.1 Support API Key in Query Param (from X-Original-URI)
	if k == "" && origURI != "" {
		if u, err := url.Parse(origURI); err == nil {
			k = u.Query().Get("key")
		}
	}

	// 1.2 Support API Key in Basic Auth Username
	if k == "" {
		if authHeader := c.GetHeader("Authorization"); strings.HasPrefix(authHeader, "Basic ") {
			if user, _, ok := c.Request.BasicAuth(); ok {
				k = user
			}
		}
	}

	if k != "" {
		if meta, valid := repository.ValidateKeyAndGetMetadata(k); valid {
			sourceID = meta.ID
			sourceName = meta.Name
			gateID = meta.GateID
			perms = meta.Permissions
		} else {
			slog.Warn("Invalid API Key attempted", "key_prefix", k[:4]+"...")
			c.Status(401)
			return
		}
	} else {
		// 2. Check Session ID
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			sessionID = strings.TrimPrefix(authHeader, "Bearer ")
		}
		if sessionID == "" {
			if cookie, err := c.Cookie("session"); err == nil {
				sessionID = cookie
			}
		}

		if sessionID != "" {
			sessionData, err := repository.GetSession(sessionID)
			if err == nil && sessionData != nil {
				userIDFloat, ok := sessionData["user_id"].(float64)
				if ok {
					uID := uint(userIDFloat)
					perms = repository.GetUserPermissions(uID)
					userID = fmt.Sprintf("%d", uID)
					username, _ = sessionData["username"].(string)
					role, _ = sessionData["role"].(string)
				}
			}
		}
	}

	// 3. Authenticated?
	if perms == nil && userID == "" && sourceID == "" {
		slog.Warn("Validation failed: no identity provided", "method", origMethod, "uri", origURI)
		c.Status(401)
		return
	}

	// 4. Centralized Authorization Check
	if origURI != "" {
		allowed, err := repository.EvaluateAccess(origMethod, origURI, perms)
		if err != nil {
			slog.Error("Policy evaluation failed", "error", err)
			c.Status(500)
			return
		}
		if !allowed {
			slog.Warn("Centralized access denied", "method", origMethod, "uri", origURI, "user_id", userID, "source_id", sourceID)
			c.Status(403)
			return
		}
	}

	// 5. Success - Set Headers and Body
	span := trace.SpanFromContext(c.Request.Context())
	
	c.Header("X-Permissions", strings.Join(repository.ExpandPermissions(perms), ","))
	if userID != "" {
		c.Header("X-User-ID", userID)
		c.Header("X-Username", username)
		span.SetAttributes(attribute.String("truckguard.user_id", userID))
	}
	if sourceID != "" {
		c.Header("X-Source-ID", sourceID)
		c.Header("X-Source-Name", sourceName)
		span.SetAttributes(attribute.String("truckguard.source_id", sourceID))
	}
	if gateID != "" {
		c.Header("X-Gate-ID", gateID)
		span.SetAttributes(attribute.String("truckguard.gate_id", gateID))
	}

	// Return full user data for frontend
	if userID != "" {
		c.JSON(200, gin.H{
			"id":          userID,
			"username":    username,
			"role":        role,
			"permissions": perms,
			"session_id":  sessionID,
		})
	} else {
		c.Status(200)
	}
}

func HandleListSessions(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.Status(401)
		return
	}

	var userID uint
	fmt.Sscanf(userIDStr, "%d", &userID)

	sessions, err := repository.ListSessions(userID)
	if err != nil {
		slog.Error("Failed to list sessions", "user_id", userID, "error", err)
		c.Status(500)
		return
	}

	c.JSON(200, sessions)
}

func HandleRevokeSession(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.Status(401)
		return
	}

	var userID uint
	fmt.Sscanf(userIDStr, "%d", &userID)

	var b struct {
		SessionID string `json:"session_id"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	// Verify session belongs to user
	sessionData, err := repository.GetSession(b.SessionID)
	if err != nil {
		c.Status(404)
		return
	}

	sessionUserIDFloat, ok := sessionData["user_id"].(float64)
	if !ok || uint(sessionUserIDFloat) != userID {
		slog.Warn("Attempted to revoke session of another user", "actor_id", userID, "target_session", b.SessionID)
		c.Status(403)
		return
	}

	if err := repository.RevokeSession(userID, b.SessionID); err != nil {
		c.Status(500)
		return
	}

	c.Status(204)
}

func HandleRevokeAllSessions(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.Status(401)
		return
	}

	var userID uint
	fmt.Sscanf(userIDStr, "%d", &userID)

	if err := repository.RevokeAllSessions(userID); err != nil {
		slog.Error("Failed to revoke all sessions", "user_id", userID, "error", err)
		c.Status(500)
		return
	}

	c.Status(204)
}

func HandleLogout(c *gin.Context) {
	var sessionID string
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		sessionID = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if sessionID == "" {
		if cookie, err := c.Cookie("session"); err == nil {
			sessionID = cookie
		}
	}

	if sessionID != "" {
		repository.DeleteSession(sessionID)
	}

	c.SetCookie("session", "", -1, "/", "", false, true)
	c.Status(204)
}

func HandleListPermissions(c *gin.Context) {
	var perms []models.Permission
	repository.DB.WithContext(c.Request.Context()).Find(&perms)
	c.JSON(200, perms)
}

func HandleCreateRole(c *gin.Context) {
	var b struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	role := models.Role{Name: b.Name, Description: b.Description}
	if err := repository.DB.WithContext(c.Request.Context()).Create(&role).Error; err != nil {
		c.JSON(500, gin.H{"error": "Role already exists or DB error"})
		return
	}
	c.JSON(201, role)
}

func HandleAssignPermissionsToRole(c *gin.Context) {
	roleID := c.Param("id")
	var b struct {
		PermissionIDs []string `json:"permission_ids"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	var role models.Role
	if err := repository.DB.WithContext(c.Request.Context()).First(&role, roleID).Error; err != nil {
		c.JSON(404, gin.H{"error": "Role not found"})
		return
	}

	var perms []models.Permission
	repository.DB.WithContext(c.Request.Context()).Where("id IN ?", b.PermissionIDs).Find(&perms)

	// Permission escalation prevention
	actorPerms := strings.Split(c.GetHeader("X-Permissions"), ",")
	targetPermIDs := make([]string, len(perms))
	for i, p := range perms {
		targetPermIDs[i] = p.ID
	}
	if err := repository.ValidatePermissions(actorPerms, targetPermIDs); err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	if err := repository.DB.WithContext(c.Request.Context()).Model(&role).Association("Permissions").Replace(perms); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var userIDs []uint
	repository.DB.WithContext(c.Request.Context()).Model(&models.User{}).Where("role_id = ?", role.ID).Pluck("id", &userIDs)
	for _, id := range userIDs {
		repository.InvalidateUserCache(id)
	}

	c.JSON(200, gin.H{"message": "Permissions updated for role " + role.Name})
}

func HandleUpdateUserRole(c *gin.Context) {
	userIDStr := c.Param("id")
	var b struct {
		RoleID uint `json:"role_id"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	repository.DB.WithContext(c.Request.Context()).Model(&models.User{}).Where("id = ?", userIDStr).Update("role_id", b.RoleID)

	var id uint
	fmt.Sscanf(userIDStr, "%d", &id)
	repository.InvalidateUserCache(id)

	c.Status(200)
}

func HandleListKeys(c *gin.Context) {
	var keys []models.APIKey
	repository.DB.WithContext(c.Request.Context()).Preload("Permissions").Find(&keys)
	c.JSON(200, keys)
}

func HandleCreateKeyWithPerms(c *gin.Context) {
	var b struct {
		Name          string   `json:"name"`
		GateID        string   `json:"gate_id"`
		PermissionIDs []string `json:"permission_ids"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	rb := make([]byte, 16)
	rand.Read(rb)
	rk := hex.EncodeToString(rb)

	var perms []models.Permission
	repository.DB.WithContext(c.Request.Context()).Where("id IN ?", b.PermissionIDs).Find(&perms)

	// Permission escalation prevention
	actorPerms := strings.Split(c.GetHeader("X-Permissions"), ",")
	targetPermIDs := make([]string, len(perms))
	for i, p := range perms {
		targetPermIDs[i] = p.ID
	}
	if err := repository.ValidatePermissions(actorPerms, targetPermIDs); err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	key := models.APIKey{
		KeyHash:     repository.HashKey(rk),
		OwnerName:   b.Name,
		GateID:      b.GateID,
		Permissions: perms,
	}

	if err := repository.DB.WithContext(c.Request.Context()).Create(&key).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"api_key": rk, "id": key.ID})
}

func HandleUpdateKeyStatus(c *gin.Context) {
	id := c.Param("id")
	var b struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	var key models.APIKey
	if err := repository.DB.WithContext(c.Request.Context()).First(&key, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Key not found"})
		return
	}

	repository.DB.WithContext(c.Request.Context()).Model(&key).Update("is_active", b.IsActive)

	repository.RDB.Del(context.Background(), "auth:"+key.KeyHash)

	c.Status(200)
}

func HandleAssignPermissionsToKey(c *gin.Context) {
	id := c.Param("id")
	var b struct {
		PermissionIDs []string `json:"permission_ids"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	var key models.APIKey
	if err := repository.DB.WithContext(c.Request.Context()).First(&key, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Key not found"})
		return
	}

	var perms []models.Permission
	repository.DB.WithContext(c.Request.Context()).Where("id IN ?", b.PermissionIDs).Find(&perms)

	// Permission escalation prevention
	actorPerms := strings.Split(c.GetHeader("X-Permissions"), ",")
	targetPermIDs := make([]string, len(perms))
	for i, p := range perms {
		targetPermIDs[i] = p.ID
	}
	if err := repository.ValidatePermissions(actorPerms, targetPermIDs); err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	repository.DB.WithContext(c.Request.Context()).Model(&key).Association("Permissions").Replace(perms)

	repository.RDB.Del(context.Background(), "auth:"+key.KeyHash)

	c.JSON(200, gin.H{"message": "Permissions updated for key " + key.OwnerName})
}

func HandleUpdateKey(c *gin.Context) {
	id := c.Param("id")
	var b struct {
		OwnerName string `json:"owner_name"`
		IsActive  bool   `json:"is_active"`
		GateID    string `json:"gate_id"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}

	var key models.APIKey
	if err := repository.DB.WithContext(c.Request.Context()).First(&key, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Key not found"})
		return
	}

	repository.DB.WithContext(c.Request.Context()).Model(&key).Updates(map[string]interface{}{
		"owner_name": b.OwnerName,
		"is_active":  b.IsActive,
		"gate_id":    b.GateID,
	})

	repository.RDB.Del(context.Background(), "auth:"+key.KeyHash)

	c.JSON(200, key)
}

func HandleDeleteKey(c *gin.Context) {
	id := c.Param("id")
	var key models.APIKey
	if err := repository.DB.WithContext(c.Request.Context()).First(&key, id).Error; err == nil {
		repository.RDB.Del(context.Background(), "auth:"+key.KeyHash)

		if err := repository.DB.WithContext(c.Request.Context()).Model(&key).Association("Permissions").Clear(); err != nil {
			c.JSON(500, gin.H{"error": "Failed to clear permissions: " + err.Error()})
			return
		}

		if err := repository.DB.WithContext(c.Request.Context()).Delete(&key).Error; err != nil {
			c.JSON(500, gin.H{"error": "Failed to delete key: " + err.Error()})
			return
		}
	} else {
		c.JSON(404, gin.H{"error": "Key not found"})
		return
	}
	c.Status(204)
}

func HandleListUsers(c *gin.Context) {
	var users []models.User
	repository.DB.WithContext(c.Request.Context()).Preload("Role").Find(&users)
	c.JSON(200, users)
}

func HandleDeleteUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := repository.DB.WithContext(c.Request.Context()).First(&user, id).Error; err == nil {
		repository.InvalidateUserCache(user.ID)
		repository.DB.WithContext(c.Request.Context()).Delete(&user)
		c.Status(204)
		return
	}
	c.JSON(404, gin.H{"error": "User not found"})
}

func HandleGetUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := repository.DB.WithContext(c.Request.Context()).Preload("Role").First(&user, id).Error; err == nil {
		c.JSON(200, user)
		return
	}
	c.JSON(404, gin.H{"error": "User not found"})
}

func HandleListRoles(c *gin.Context) {
	var roles []models.Role
	repository.DB.WithContext(c.Request.Context()).Preload("Permissions").Find(&roles)
	c.JSON(200, roles)
}

func HandleUpdateRole(c *gin.Context) {
	id := c.Param("id")
	var b struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.Status(400)
		return
	}
	repository.DB.WithContext(c.Request.Context()).Model(&models.Role{}).Where("id = ?", id).Updates(models.Role{Name: b.Name, Description: b.Description})
	c.Status(200)
}

func HandleDeleteRole(c *gin.Context) {
	id := c.Param("id")
	var count int64
	repository.DB.WithContext(c.Request.Context()).Model(&models.User{}).Where("role_id = ?", id).Count(&count)

	if count > 0 {
		c.JSON(400, gin.H{"error": "Cannot delete role: users are still assigned to it"})
		return
	}

	repository.DB.WithContext(c.Request.Context()).Delete(&models.Role{}, id)
	c.Status(204)
}

func HandleChangePassword(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.Status(401)
		return
	}

	var b struct {
		CurrentPass string `json:"current_password" binding:"required"`
		NewPass     string `json:"new_password" binding:"required"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var u models.User
	if err := repository.DB.WithContext(c.Request.Context()).Where("id = ?", userIDStr).First(&u).Error; err != nil {
		c.Status(404)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(b.CurrentPass)); err != nil {
		c.JSON(401, gin.H{"error": "Invalid current password"})
		return
	}

	h, err := bcrypt.GenerateFromPassword([]byte(b.NewPass), 10)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate password hash"})
		return
	}

	u.PasswordHash = string(h)
	if err := repository.DB.WithContext(c.Request.Context()).Save(&u).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update password"})
		return
	}

	slog.Info("Password changed", "user_id", u.ID, "username", u.Username)
	c.JSON(200, gin.H{"message": "Password changed successfully"})
}

func HandleAdminResetPassword(c *gin.Context) {
	targetUserID := c.Param("id")
	var b struct {
		NewPass string `json:"new_password" binding:"required"`
	}
	if err := c.BindJSON(&b); err != nil {
		c.JSON(400, gin.H{"error": "New password is required"})
		return
	}

	h, err := bcrypt.GenerateFromPassword([]byte(b.NewPass), 10)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate password hash"})
		return
	}

	var u models.User
	if err := repository.DB.WithContext(c.Request.Context()).Where("id = ?", targetUserID).First(&u).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	u.PasswordHash = string(h)
	if err := repository.DB.WithContext(c.Request.Context()).Save(&u).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update password"})
		return
	}

	slog.Info("Admin reset password", "admin_id", c.GetHeader("X-User-ID"), "target_user_id", u.ID, "username", u.Username)
	c.JSON(200, gin.H{"message": "Password reset successfully"})
}

func HandleGetPermissionHierarchy(c *gin.Context) {
	repository.LoadPermissionHierarchy()
	c.JSON(200, repository.PermissionHierarchy)
}
