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

const defaultTransactionTTL = 1 * time.Minute

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

func AwaitTxKey(gateID, sourceID string) string {
	return fmt.Sprintf("tx_await:%s:%s", gateID, sourceID)
}

type gateSettings struct {
	TTLSeconds *int `json:"transaction_ttl_seconds"`
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
	if s.TTLSeconds != nil && *s.TTLSeconds > 0 {
		return time.Duration(*s.TTLSeconds) * time.Second
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
//   - Checks if another device registered an await key for this sourceID (GETDEL — atomic).
//   - If awaited: attaches this event to the awaiting device's transaction.
//   - Otherwise: finds the active transaction for the gate via Valkey, or creates a fresh one.
//   - Enforces max-events-per-transaction: rotates to a new transaction when the limit is hit.
func ResolveTransaction(ctx context.Context, gateID, sourceID string, externalID *uuid.UUID) uuid.UUID {
	if externalID != nil && *externalID != uuid.Nil {
		tx := repository.GetTransactionRaw(*externalID)
		if tx != nil && tx.GateID == gateID {
			// Best-effort TTL refresh — keeps the gate's active key alive while the Puller lands.
			repository.RDB.Expire(ctx, ActiveTxKey(gateID), GateTTL(gateID))
			return tx.ID
		}
		log.Printf("matchmaker: external tx %s not found or gate mismatch (want %s) — creating new", externalID, gateID)
	}

	// Await path: atomically claim the reservation if another device is waiting for this sourceID.
	if val, err := repository.RDB.GetDel(ctx, AwaitTxKey(gateID, sourceID)).Result(); err == nil {
		if id, parseErr := uuid.Parse(val); parseErr == nil && id != uuid.Nil {
			log.Printf("matchmaker: source %s joined awaited tx %s", sourceID, id)
			return id
		}
	}

	return findOrCreateActive(ctx, gateID)
}

// RegisterAwaits sets a tx_await key in Valkey for each device listed in the source's
// AwaitSourceIDs config. Called after the transaction is resolved so the key carries
// the correct transaction UUID.
func RegisterAwaits(ctx context.Context, gateID, sourceID string, txID uuid.UUID) {
	config := repository.GetDeviceConfigBySourceIDCached(ctx, sourceID)
	if config == nil {
		return
	}
	var awaitIDs []string
	if err := json.Unmarshal(config.AwaitSourceIDs, &awaitIDs); err != nil || len(awaitIDs) == 0 {
		return
	}
	ttl := time.Duration(config.AwaitTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = GateTTL(gateID)
	}
	// SetNX: only claim the slot if no other transaction is already waiting for that device.
	// This ensures the first truck's transaction keeps priority when a second truck arrives
	// before the awaited device (e.g. an exit camera) has fired.
	pipe := repository.RDB.Pipeline()
	for _, awaitID := range awaitIDs {
		if awaitID != "" {
			pipe.SetNX(ctx, AwaitTxKey(gateID, awaitID), txID.String(), ttl)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("matchmaker: failed to register awaits for %s: %v", sourceID, err)
	}
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
