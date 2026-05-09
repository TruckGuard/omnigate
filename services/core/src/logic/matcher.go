package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
)

const defaultTransactionTTL = 30 * time.Minute

func generateTransactionCode(gateID string) string {
	now := time.Now()
	return fmt.Sprintf("TRK-%s-%04d%02d%02d-%06d-%04x",
		gateID,
		now.Year(),
		now.Month(),
		now.Day(),
		now.Unix()%1000000,
		rand.Intn(0xFFFF),
	)
}

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

// ResolveTransaction is the single entry-point for routing an incoming event to a transaction.
//
// Puller path — externalID != nil:
//   - Validates the transaction exists and belongs to gateID.
//   - On success: best-effort TTL refresh, returns the existing ID (even if tx_active has rotated).
//   - On failure: logs a warning and falls back to the normal gate-scoped flow.
//
// Normal path — externalID == nil:
//   - Finds the active transaction for the gate via Valkey, or creates a fresh one.
//   - Enforces max-events-per-transaction: rotates to a new transaction when the limit is hit.
func ResolveTransaction(ctx context.Context, gateID string, externalID *uuid.UUID) uuid.UUID {
	if externalID != nil && *externalID != uuid.Nil {
		tx := repository.GetTransactionRaw(*externalID)
		if tx != nil && tx.GateID == gateID {
			// Best-effort TTL refresh — keeps the gate's active key alive while the Puller lands.
			repository.RDB.Expire(ctx, ActiveTxKey(gateID), GateTTL(gateID))
			return tx.ID
		}
		log.Printf("matchmaker: external tx %s not found or gate mismatch (want %s) — creating new", externalID, gateID)
	}

	return findOrCreateActive(ctx, gateID)
}

// FindOrCreateTransaction is kept for call sites that have no ctx or external ID.
func FindOrCreateTransaction(gateID string) uuid.UUID {
	return findOrCreateActive(context.Background(), gateID)
}

// findOrCreateActive finds or creates the currently active transaction for a gate,
// rotating to a fresh one when the max-events limit is reached.
func findOrCreateActive(ctx context.Context, gateID string) uuid.UUID {
	key := ActiveTxKey(gateID)
	ttl := GateTTL(gateID)

	if val, err := repository.RDB.Get(ctx, key).Result(); err == nil {
		if id, parseErr := uuid.Parse(val); parseErr == nil && id != uuid.Nil {
			// Enforce max events before handing out the same transaction again.
			if max := MaxEventsForGate(gateID); max > 0 {
				if repository.CountEventsForTransaction(id) >= int64(max) {
					repository.RDB.Del(ctx, key)
					return newTransaction(ctx, gateID, key, ttl)
				}
			}
			repository.RDB.Expire(ctx, key, ttl)
			return id
		}
	}

	return newTransaction(ctx, gateID, key, ttl)
}

func newTransaction(ctx context.Context, gateID, key string, ttl time.Duration) uuid.UUID {
	tx := repository.CreateTransaction(&models.Transaction{
		Code:   generateTransactionCode(gateID),
		GateID: gateID,
	})
	repository.RDB.Set(ctx, key, tx.ID.String(), ttl)
	return tx.ID
}
