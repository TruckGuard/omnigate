package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
)

const devCfgCacheTTL = 5 * time.Minute

func devCfgCacheKey(sourceID string) string {
	return "cfg:core:" + sourceID
}

func ListDeviceConfigs() []models.DeviceConfig {
	var configs []models.DeviceConfig
	DB.Preload("EventType").Order("created_at desc").Find(&configs)
	return configs
}

func CreateDeviceConfig(config *models.DeviceConfig) *models.DeviceConfig {
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}
	DB.Create(config)
	return config
}

func GetDeviceConfigByID(id uuid.UUID) *models.DeviceConfig {
	var config models.DeviceConfig
	if err := DB.Preload("EventType").First(&config, "id = ?", id).Error; err != nil {
		return nil
	}
	return &config
}

func GetDeviceConfigBySourceID(sourceID string) *models.DeviceConfig {
	var config models.DeviceConfig
	if err := DB.Preload("EventType").Where("source_id = ? AND enabled = ?", sourceID, true).First(&config).Error; err != nil {
		return nil
	}
	return &config
}

// GetDeviceConfigBySourceIDCached is the hot-path variant used by the event
// pipeline. It tries Valkey first and only hits PostgreSQL on a cache miss.
func GetDeviceConfigBySourceIDCached(ctx context.Context, sourceID string) *models.DeviceConfig {
	key := devCfgCacheKey(sourceID)
	if raw, err := RDB.Get(ctx, key).Bytes(); err == nil {
		var cfg models.DeviceConfig
		if json.Unmarshal(raw, &cfg) == nil {
			return &cfg
		}
	}
	cfg := GetDeviceConfigBySourceID(sourceID)
	if cfg != nil {
		if b, err := json.Marshal(cfg); err == nil {
			RDB.Set(ctx, key, b, devCfgCacheTTL) //nolint:errcheck
		}
	}
	return cfg
}

// InvalidateDeviceConfigCache drops the Valkey entry for a source so the next
// read fetches a fresh copy from PostgreSQL.
func InvalidateDeviceConfigCache(sourceID string) {
	RDB.Del(context.Background(), devCfgCacheKey(sourceID)) //nolint:errcheck
}

func UpdateDeviceConfig(config *models.DeviceConfig) error {
	InvalidateDeviceConfigCache(config.SourceID)
	return DB.Save(config).Error
}

func DeleteDeviceConfig(id uuid.UUID) error {
	var cfg models.DeviceConfig
	if err := DB.Select("source_id").First(&cfg, id).Error; err == nil {
		InvalidateDeviceConfigCache(cfg.SourceID)
	}
	return DB.Delete(&models.DeviceConfig{}, id).Error
}
