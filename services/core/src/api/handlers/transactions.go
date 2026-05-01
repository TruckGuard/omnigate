package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/logic"
	"github.com/omnigate/services/core/src/repository"
)

func HandleListTransactions(c *gin.Context) {
	txs := repository.ListTransactions()
	c.JSON(http.StatusOK, txs)
}

func HandleGetTransaction(c *gin.Context) {
	id := c.Param("id")
	txID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}

	tx := repository.GetTransaction(txID)
	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	c.JSON(http.StatusOK, tx)
}

func HandleCreateTransaction(c *gin.Context) {
	var req struct {
		GateID string `json:"gate_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// This is a manual creation, logic.FindOrCreateTransaction handles code gen
	txID := logic.FindOrCreateTransaction(req.GateID)
	tx := repository.GetTransaction(txID)

	c.JSON(http.StatusCreated, tx)
}

func HandleUpdateTransaction(c *gin.Context) {
	id := c.Param("id")
	txID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}

	tx := repository.GetTransaction(txID)
	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	var req struct {
		Status       string  `json:"status"`
		VehiclePlate *string `json:"vehicle_plate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Status != "" {
		tx.Status = req.Status
		if req.Status == "completed" || req.Status == "cancelled" {
			now := time.Now()
			tx.CompletedAt = &now
			repository.RDB.Del(context.Background(), logic.ActiveTxKey(tx.GateID))
		}
	}

	tx.UpdatedAt = time.Now()
	if err := repository.UpdateTransaction(tx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tx)
}

func HandleDeleteTransaction(c *gin.Context) {
	id := c.Param("id")
	txID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}

	if err := repository.DeleteTransaction(txID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transaction deleted"})
}

