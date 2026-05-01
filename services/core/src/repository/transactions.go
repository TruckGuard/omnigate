package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
)

func CreateTransaction(tx *models.Transaction) *models.Transaction {
	DB.Create(tx)
	return tx
}

func FindActiveTransaction(gateID string) *models.Transaction {
	var tx models.Transaction
	if err := DB.Where("gate_id = ? AND status = 'active'", gateID).Order("created_at desc").First(&tx).Error; err != nil {
		return nil
	}
	return &tx
}

func GetTransaction(id uuid.UUID) *models.Transaction {
	var tx models.Transaction
	if err := DB.Preload("Events").First(&tx, id).Error; err != nil {
		return nil
	}
	return &tx
}

func ListTransactions() []models.Transaction {
	var txs []models.Transaction
	DB.Find(&txs)
	return txs
}

func UpdateTransaction(tx *models.Transaction) error {
	return DB.Save(tx).Error
}

func DeleteTransaction(id uuid.UUID) error {
	return DB.Delete(&models.Transaction{}, id).Error
}

func GetStaleActiveTransactions(timeout time.Duration) []models.Transaction {
	var txs []models.Transaction
	threshold := time.Now().Add(-timeout)
	
	// Complex query to find active transactions whose last event is older than threshold
	// Or transactions with no events that are older than threshold
	DB.Where("status = 'active' AND (updated_at < ?)", threshold).Find(&txs)
	return txs
}
