package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/omnigate/services/ingestor/src/models"
)

var (
	CoreURL         string
	WorkerSystemKey string
)

func InitCoreClient(url, key string) {
	CoreURL = url
	WorkerSystemKey = key
}

func GetOrCreateTransaction(gateID string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/transactions", CoreURL)
	payload, _ := json.Marshal(map[string]string{"gate_id": gateID})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}

	req.Header.Set("X-API-Key", WorkerSystemKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("core returned status: %d", resp.StatusCode)
	}

	var tx models.CoreTransaction
	if err := json.NewDecoder(resp.Body).Decode(&tx); err != nil {
		return "", err
	}

	return tx.ID, nil
}

func GetDeviceConfig(sourceID string) (*models.CoreDeviceConfig, error) {
	url := fmt.Sprintf("%s/api/v1/configs/device/%s", CoreURL, sourceID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", WorkerSystemKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("core returned status: %d", resp.StatusCode)
	}

	var config models.CoreDeviceConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
