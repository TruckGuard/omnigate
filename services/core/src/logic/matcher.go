package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
)

const TransactionTTL = 5 * time.Minute

func ActiveTxKey(gateID string) string {
	return fmt.Sprintf("tx_active:%s", gateID)
}

// FindOrCreateTransaction checks Valkey for an active transaction for the gate.
// If found, refreshes TTL and returns the ID.
// If not found, creates a new transaction in DB and registers it in Valkey.
func FindOrCreateTransaction(gateID string) uuid.UUID {
	ctx := context.Background()
	key := ActiveTxKey(gateID)

	val, err := repository.RDB.Get(ctx, key).Result()
	if err == nil {
		repository.RDB.Expire(ctx, key, TransactionTTL)
		if id, parseErr := uuid.Parse(val); parseErr == nil {
			return id
		}
	}

	code := generateTransactionCode(gateID)
	newTx := &models.Transaction{
		Code:   code,
		GateID: gateID,
		Status: "active",
	}
	savedTx := repository.CreateTransaction(newTx)

	repository.RDB.Set(ctx, key, savedTx.ID.String(), TransactionTTL)

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
