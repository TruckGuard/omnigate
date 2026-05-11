package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"log/slog"

	"github.com/omnigate/services/auth/src/models"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
	ctx = context.Background()
)

func InitDB(dsn string) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to Auth Database")
	}

	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		panic(err)
	}

	DB = db
	DB.AutoMigrate(&models.Permission{}, &models.Role{}, &models.User{}, &models.APIKey{}, &models.PolicyRule{}, &models.PermissionHierarchy{})
	LoadPermissionHierarchy()
}

func InitRedis(addr string) {
	RDB = redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
}

func HashKey(key string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
}

func CreateSession(userID uint, username, role, ip, userAgent string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	sessionID := hex.EncodeToString(b)

	data := map[string]interface{}{
		"user_id":    userID,
		"username":   username,
		"role":       role,
		"ip":         ip,
		"user_agent": userAgent,
		"created_at": time.Now().Format(time.RFC3339),
	}

	val, _ := json.Marshal(data)

	pipeline := RDB.Pipeline()
	pipeline.Set(ctx, "session:"+sessionID, val, 24*time.Hour)
	pipeline.SAdd(ctx, fmt.Sprintf("user_sessions:%d", userID), sessionID)
	_, err := pipeline.Exec(ctx)

	return sessionID, err
}

func GetSession(sessionID string) (map[string]interface{}, error) {
	key := "session:" + sessionID
	val, err := RDB.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// Sliding expiration: кожен активний запит подовжує TTL сесії.
	RDB.Expire(ctx, key, 24*time.Hour)

	var data map[string]interface{}
	json.Unmarshal([]byte(val), &data)
	return data, nil
}

func ListSessions(userID uint) ([]map[string]interface{}, error) {
	key := fmt.Sprintf("user_sessions:%d", userID)
	sessionIDs, err := RDB.SMembers(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(sessionIDs) == 0 {
		return nil, nil
	}

	// Batch-fetch all session values in one round-trip instead of N individual GETs.
	redisKeys := make([]string, len(sessionIDs))
	for i, sid := range sessionIDs {
		redisKeys[i] = "session:" + sid
	}
	vals, err := RDB.MGet(ctx, redisKeys...).Result()
	if err != nil {
		return nil, err
	}

	var expired []interface{}
	var activeSessions []map[string]interface{}
	for i, v := range vals {
		if v == nil {
			// Session expired — schedule removal from the set.
			expired = append(expired, sessionIDs[i])
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(v.(string)), &data); err != nil {
			expired = append(expired, sessionIDs[i])
			continue
		}
		data["session_id"] = sessionIDs[i]
		activeSessions = append(activeSessions, data)
	}

	if len(expired) > 0 {
		RDB.SRem(ctx, key, expired...)
	}

	return activeSessions, nil
}

func DeleteSession(sessionID string) error {
	data, err := GetSession(sessionID)
	if err == nil {
		userIDFloat, ok := data["user_id"].(float64)
		if ok {
			RDB.SRem(ctx, fmt.Sprintf("user_sessions:%d", uint(userIDFloat)), sessionID)
		}
	}
	return RDB.Del(ctx, "session:"+sessionID).Err()
}

func RevokeSession(userID uint, sessionID string) error {
	RDB.SRem(ctx, fmt.Sprintf("user_sessions:%d", userID), sessionID)
	return RDB.Del(ctx, "session:"+sessionID).Err()
}
func RevokeAllSessions(userID uint) error {
	key := fmt.Sprintf("user_sessions:%d", userID)
	sessionIDs, _ := RDB.SMembers(ctx, key).Result()

	pipeline := RDB.Pipeline()
	for _, sid := range sessionIDs {
		pipeline.Del(ctx, "session:"+sid)
	}
	pipeline.Del(ctx, key)
	_, err := pipeline.Exec(ctx)
	return err
}

func ValidateKeyAndGetMetadata(key string) (models.SourceMetadata, bool) {
	h := HashKey(key)

	if v, _ := RDB.Get(ctx, "auth:"+h).Result(); v != "" {
		slog.Debug("API Key found in cache", "hash", h[:8])
		var meta models.SourceMetadata
		json.Unmarshal([]byte(v), &meta)
		return meta, true
	}

	var ak models.APIKey
	if err := DB.Preload("Permissions").Where("key_hash = ? AND is_active = ?", h, true).First(&ak).Error; err == nil {
		slog.Debug("API Key validated against DB", "owner", ak.OwnerName)
		var perms []string
		for _, p := range ak.Permissions {
			perms = append(perms, p.ID)
		}

		meta := models.SourceMetadata{
			ID:          fmt.Sprintf("%d", ak.ID),
			Name:        ak.OwnerName,
			GateID:      ak.GateID,
			Permissions: perms,
		}

		val, _ := json.Marshal(meta)
		RDB.Set(ctx, "auth:"+h, val, 15*time.Minute)
		return meta, true
	}

	return models.SourceMetadata{}, false
}

func GetUserPermissions(userID uint) []string {
	key := fmt.Sprintf("user_perms:%d", userID)

	val, _ := RDB.Get(ctx, key).Result()
	if val != "" {
		var perms []string
		json.Unmarshal([]byte(val), &perms)
		return perms
	}

	var u models.User
	DB.Preload("Role.Permissions").First(&u, userID)

	var perms []string
	for _, p := range u.Role.Permissions {
		perms = append(perms, p.ID)
	}

	valJSON, _ := json.Marshal(perms)
	RDB.Set(ctx, key, valJSON, time.Hour)

	return perms
}

func InvalidateUserCache(userID uint) {
	slog.Debug("Invalidating user cache", "user_id", userID)
	key := fmt.Sprintf("user_perms:%d", userID)
	RDB.Del(ctx, key)
}
