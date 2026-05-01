package repository

import (
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
)

func ListUserProfiles() []models.UserProfile {
	var profiles []models.UserProfile
	DB.Order("last_name ASC, first_name ASC").Find(&profiles)
	return profiles
}

func GetUserProfile(id uuid.UUID) *models.UserProfile {
	var profile models.UserProfile
	if err := DB.First(&profile, id).Error; err != nil {
		return nil
	}
	return &profile
}

func GetUserProfileByAuthID(authID uint) *models.UserProfile {
	var profile models.UserProfile
	if err := DB.Where("auth_id = ?", authID).First(&profile).Error; err != nil {
		return nil
	}
	return &profile
}

func CreateUserProfile(profile *models.UserProfile) *models.UserProfile {
	DB.Create(profile)
	return profile
}

func UpdateUserProfile(profile *models.UserProfile) error {
	return DB.Save(profile).Error
}

func DeleteUserProfile(id uuid.UUID) error {
	return DB.Delete(&models.UserProfile{}, id).Error
}
