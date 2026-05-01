package repository

import (
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
	return &tx
}

type TransactionFilter struct {
	GateID string
	Status string
	Search string
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
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
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

	return txs, total
}

func UpdateTransaction(tx *models.Transaction) error {
	return DB.Save(tx).Error
}

func DeleteTransaction(id uuid.UUID) error {
	return DB.Delete(&models.Transaction{}, id).Error
}
