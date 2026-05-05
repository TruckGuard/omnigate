package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
)

func HandleListUserProfiles(c *gin.Context) {
	// Optional: look up by auth_id query param
	if authIDStr := c.Query("auth_id"); authIDStr != "" {
		authID, err := strconv.ParseUint(authIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid auth_id"})
			return
		}
		profile := repository.GetUserProfileByAuthID(uint(authID))
		if profile == nil {
			c.JSON(http.StatusOK, []models.UserProfile{}) // Повертаємо порожній масив замість 404
			return
		}
		c.JSON(http.StatusOK, []models.UserProfile{*profile}) // Повертаємо масив з одним профілем
		return
	}
	c.JSON(http.StatusOK, repository.ListUserProfiles())
}

func HandleGetUserProfile(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid profile ID"})
		return
	}
	profile := repository.GetUserProfile(id)
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User profile not found"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func HandleCreateUserProfile(c *gin.Context) {
	var req struct {
		AuthID    uint   `json:"auth_id" binding:"required"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
		GateID    string `json:"gate_id"`
		Notes     string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile := repository.CreateUserProfile(&models.UserProfile{
		AuthID:    req.AuthID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		GateID:    req.GateID,
		Notes:     req.Notes,
	})
	c.JSON(http.StatusCreated, profile)
}

func HandleUpdateUserProfile(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid profile ID"})
		return
	}
	profile := repository.GetUserProfile(id)
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User profile not found"})
		return
	}

	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
		GateID    string `json:"gate_id"`
		Notes     string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.FirstName != "" {
		profile.FirstName = req.FirstName
	}
	if req.LastName != "" {
		profile.LastName = req.LastName
	}
	if req.Phone != "" {
		profile.Phone = req.Phone
	}
	if req.GateID != "" {
		profile.GateID = req.GateID
	}
	profile.Notes = req.Notes

	if err := repository.UpdateUserProfile(profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func HandleDeleteUserProfile(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid profile ID"})
		return
	}
	if err := repository.DeleteUserProfile(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User profile deleted"})
}
