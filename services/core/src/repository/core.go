package repository

import (
	"log"

	"github.com/omnigate/services/core/src/models"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
)

func InitDB(dsn string) {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// AutoMigrate all models (order matters for foreign keys)
	DB.AutoMigrate(&models.EventType{}, &models.Transaction{}, &models.Event{}, &models.DeviceConfig{})
}

func InitRedis(addr string) {
	RDB = redis.NewClient(&redis.Options{
		Addr: addr,
	})
}
