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

