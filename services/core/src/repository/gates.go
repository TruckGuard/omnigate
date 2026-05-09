package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
)

func ListGates() []models.Gate {
	var gates []models.Gate
	DB.Order("name ASC").Find(&gates)
	return gates
}

func GetGate(id uuid.UUID) *models.Gate {
	var gate models.Gate
	if err := DB.First(&gate, id).Error; err != nil {
		return nil
	}
	return &gate
}

func GetGateByGateID(gateID string) *models.Gate {
	var gate models.Gate
	if err := DB.Where("gate_id = ?", gateID).First(&gate).Error; err != nil {
		return nil
	}
	return &gate
}

func CreateGate(gate *models.Gate) *models.Gate {
	if gate.ID == uuid.Nil {
		gate.ID = uuid.New()
	}
	DB.Create(gate)
	return gate
}

func UpdateGate(gate *models.Gate) error {
	return DB.Save(gate).Error
}

func DeleteGate(id uuid.UUID) error {
	return DB.Delete(&models.Gate{}, id).Error
}

type GateStats struct {
	TotalTransactions int64          `json:"total_transactions"`
	OpenTransactions  int64          `json:"open_transactions"`
	TotalDevices      int64          `json:"total_devices"`
	RecentTransactions []models.Transaction `json:"recent_transactions"`
}

func GetGateStats(gateID string) GateStats {
	var stats GateStats
	DB.Model(&models.Transaction{}).Where("gate_id = ?", gateID).Count(&stats.TotalTransactions)

	// Open = a Valkey key exists for this gate pointing to an active transaction
	activeTxID, err := RDB.Get(context.Background(), fmt.Sprintf("tx_active:%s", gateID)).Result()
	if err == nil && activeTxID != "" {
		stats.OpenTransactions = 1
	}

	DB.Model(&models.DeviceConfig{}).Where("gate_id = ?", gateID).Count(&stats.TotalDevices)
	DB.Where("gate_id = ?", gateID).Order("created_at DESC").Limit(5).Find(&stats.RecentTransactions)

	// Annotate is_open on recent transactions
	for i := range stats.RecentTransactions {
		stats.RecentTransactions[i].IsOpen = activeTxID != "" && activeTxID == stats.RecentTransactions[i].ID.String()
	}
	return stats
}
