package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
)

const defaultTransactionTTL = 30 * time.Minute

func ActiveTxKey(gateID string) string {
	return fmt.Sprintf("tx_active:%s", gateID)
}

type gateSettings struct {
	TTLMinutes *int `json:"transaction_ttl_minutes"`
	MaxEvents  *int `json:"max_events_per_transaction"`
}

func getGateSettings(gateID string) gateSettings {
	gate := repository.GetGateByGateID(gateID)
	if gate == nil {
		return gateSettings{}
	}
	var s gateSettings
	json.Unmarshal(gate.Settings, &s) //nolint:errcheck
	return s
}

func GateTTL(gateID string) time.Duration {
	s := getGateSettings(gateID)
	if s.TTLMinutes != nil && *s.TTLMinutes > 0 {
		return time.Duration(*s.TTLMinutes) * time.Minute
	}
	return defaultTransactionTTL
}

// MaxEventsForGate returns the max events per transaction (0 = unlimited).
func MaxEventsForGate(gateID string) int {
	s := getGateSettings(gateID)
	if s.MaxEvents != nil && *s.MaxEvents > 0 {
		return *s.MaxEvents
	}
	return 0
}

// FindOrCreateTransaction checks Valkey for an active transaction for the gate.
// If found, refreshes the TTL and returns the ID.
// If the key has expired a new transaction is simply created — no DB status changes needed.
func FindOrCreateTransaction(gateID string) uuid.UUID {
	ctx := context.Background()
	key := ActiveTxKey(gateID)
	ttl := GateTTL(gateID)

	if val, err := repository.RDB.Get(ctx, key).Result(); err == nil {
		repository.RDB.Expire(ctx, key, ttl)
		if id, parseErr := uuid.Parse(val); parseErr == nil {
			return id
		}
	}

	code := generateTransactionCode(gateID)
	newTx := &models.Transaction{
		Code:   code,
		GateID: gateID,
	}
	savedTx := repository.CreateTransaction(newTx)
	repository.RDB.Set(ctx, key, savedTx.ID.String(), ttl)
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
