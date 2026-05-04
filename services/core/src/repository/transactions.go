package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
)

func CreateTransaction(tx *models.Transaction) *models.Transaction {
	DB.Create(tx)
	return tx
}

func GetTransaction(id uuid.UUID) *models.Transaction {
	var tx models.Transaction
	if err := DB.Preload("Events.EventType").First(&tx, id).Error; err != nil {
		return nil
	}
	setOpenStatus(&tx)
	return &tx
}

type TransactionFilter struct {
	GateID string
	Search string
	Open   bool // if true, only return currently-open transactions (Valkey key exists)
	Page   int
	Limit  int
}

func ListTransactions(f TransactionFilter) ([]models.Transaction, int64) {
	var txs []models.Transaction
	var total int64

	q := DB.Model(&models.Transaction{})
	if f.GateID != "" {
		q = q.Where("gate_id = ?", f.GateID)
	}
	if f.Search != "" {
		q = q.Where("code ILIKE ?", "%"+f.Search+"%")
	}

	q.Count(&total)

	page := f.Page
	if page < 1 {
		page = 1
	}
	limit := f.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q.Preload("Events").Offset((page - 1) * limit).Limit(limit).Order("created_at DESC").Find(&txs)
	annotateOpen(txs)

	if f.Open {
		open := txs[:0]
		for _, tx := range txs {
			if tx.IsOpen {
				open = append(open, tx)
			}
		}
		return open, int64(len(open))
	}

	return txs, total
}

func UpdateTransaction(tx *models.Transaction) error {
	return DB.Save(tx).Error
}

func DeleteTransaction(id uuid.UUID) error {
	return DB.Delete(&models.Transaction{}, id).Error
}

// CountEventsForTransaction returns the number of events attached to a transaction.
func CountEventsForTransaction(txID uuid.UUID) int64 {
	var count int64
	DB.Model(&models.Event{}).Where("transaction_id = ?", txID).Count(&count)
	return count
}

// setOpenStatus checks Valkey for the single given transaction.
func setOpenStatus(tx *models.Transaction) {
	key := fmt.Sprintf("tx_active:%s", tx.GateID)
	val, err := RDB.Get(context.Background(), key).Result()
	tx.IsOpen = err == nil && val == tx.ID.String()
}

// annotateOpen batch-checks Valkey for a slice of transactions.
func annotateOpen(txs []models.Transaction) {
	if len(txs) == 0 {
		return
	}
	ctx := context.Background()
	// Collect unique gate IDs
	seen := map[string]string{} // gateID → active txID
	gates := make([]string, 0)
	for _, tx := range txs {
		if _, ok := seen[tx.GateID]; !ok {
			seen[tx.GateID] = ""
			gates = append(gates, tx.GateID)
		}
	}
	// Fetch active tx IDs for each gate in one pass
	for _, gateID := range gates {
		key := fmt.Sprintf("tx_active:%s", gateID)
		if val, err := RDB.Get(ctx, key).Result(); err == nil {
			seen[gateID] = val
		}
	}
	for i := range txs {
		activeTxID := seen[txs[i].GateID]
		txs[i].IsOpen = activeTxID != "" && activeTxID == txs[i].ID.String()
	}
}
