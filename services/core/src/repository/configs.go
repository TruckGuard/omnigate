package repository

import (
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
)

func CreateDeviceConfig(config *models.DeviceConfig) *models.DeviceConfig {
	DB.Create(config)
	return config
}

func GetDeviceConfigBySourceID(sourceID string) *models.DeviceConfig {
	var config models.DeviceConfig
	if err := DB.Preload("EventType").Where("source_id = ? AND enabled = ?", sourceID, true).First(&config).Error; err != nil {
		return nil
	}
	return &config
}

func UpdateDeviceConfig(config *models.DeviceConfig) error {
	return DB.Save(config).Error
}

func DeleteDeviceConfig(id uuid.UUID) error {
	return DB.Delete(&models.DeviceConfig{}, id).Error
}
