package logic

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
)

// FindOrCreateTransaction finds an active transaction for the gate or creates a new one
func FindOrCreateTransaction(gateID string) uuid.UUID {
	tx := repository.FindActiveTransaction(gateID)

	if tx != nil {
		// Touch updated_at
		tx.UpdatedAt = time.Now()
		repository.UpdateTransaction(tx)
		return tx.ID
	}

	code := generateTransactionCode(gateID)
	newTx := &models.Transaction{
		Code:   code,
		GateID: gateID,
		Status: "active",
	}

	savedTx := repository.CreateTransaction(newTx)
	return savedTx.ID
}

func generateTransactionCode(gateID string) string {
	now := time.Now()
	return fmt.Sprintf("TRK-%s-%04d%02d%02d-%06d",
		gateID,
		now.Year(),
		now.Month(),
		now.Day(),
		now.Unix()%1000000,
	)
}

func CleanupStaleTransactions() {
	// Close transactions inactive for 5 minutes
	staleTxs := repository.GetStaleActiveTransactions(5 * time.Minute)
	
	now := time.Now()
	for _, tx := range staleTxs {
		tx.Status = "completed"
		tx.CompletedAt = &now
		tx.UpdatedAt = now
		repository.UpdateTransaction(&tx)
	}
}
