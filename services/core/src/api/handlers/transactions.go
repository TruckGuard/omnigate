package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/logic"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func HandleListTransactions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	txs, total := repository.ListTransactions(repository.TransactionFilter{
		GateID: c.Query("gate_id"),
		Search: c.Query("search"),
		Open:   c.Query("open") == "true",
		Page:   page,
		Limit:  limit,
	})

	c.JSON(http.StatusOK, gin.H{
		"data":  txs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
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

	// Enrich span with business context
	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		attribute.String("truckguard.gate_id", req.GateID),
	)

	txID := logic.FindOrCreateTransaction(req.GateID)
	
	span.SetAttributes(
		attribute.String("truckguard.transaction_id", txID.String()),
	)
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
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx.Note = req.Note
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

// HandleVehicleHistory повертає список закритих транзакцій, в яких
// зафіксований номер авто нечітко збігається з query-параметром ?plate=.
// Пошук делегується repository.FindPastTransactionsFuzzy (pg_trgm + levenshtein).
func HandleVehicleHistory(c *gin.Context) {
	plate := c.Query("plate")
	if plate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plate query parameter is required"})
		return
	}

	txs, err := repository.FindPastTransactionsFuzzy(
		repository.DB.WithContext(c.Request.Context()),
		plate,
		10,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if txs == nil {
		txs = []models.Transaction{}
	}

	c.JSON(http.StatusOK, gin.H{
		"plate": plate,
		"data":  txs,
	})
}
