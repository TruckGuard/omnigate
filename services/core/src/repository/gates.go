package repository

import (
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
	DB.Create(gate)
	return gate
}

func UpdateGate(gate *models.Gate) error {
	return DB.Save(gate).Error
}

func DeleteGate(id uuid.UUID) error {
	return DB.Delete(&models.Gate{}, id).Error
}
