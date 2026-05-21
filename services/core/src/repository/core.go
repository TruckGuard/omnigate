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
	MigrateDB(DB)
}

// MigrateDB виконує повну міграцію схеми:
// AutoMigrate моделей + PostgreSQL-розширення для нечіткого пошуку + GIN-індекс.
//
// Приймає *gorm.DB явно, щоб функцію можна було викликати в тестах
// з ізольованою БД, незалежно від глобальної DB.
func MigrateDB(db *gorm.DB) {
	// AutoMigrate в порядку залежностей (EventType → Gate → Transaction → Event → ...)
	if err := db.AutoMigrate(
		&models.EventType{},
		&models.Gate{},
		&models.Transaction{},
		&models.Event{},
		&models.DeviceConfig{},
		&models.UserProfile{},
	); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}

	// pg_trgm — тригамний індекс, оператор %, функція similarity().
	// fuzzystrmatch — функція levenshtein_less_equal() для точної перевірки відстані.
	db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm")
	db.Exec("CREATE EXTENSION IF NOT EXISTS fuzzystrmatch")

	// Видалення застарілого поля raw_payload (дані тепер лише в Garage/S3).
	db.Exec("ALTER TABLE events DROP COLUMN IF EXISTS raw_payload")

	// GIN-індекс з класом операторів gin_trgm_ops на полі searchable_value.
	// Дозволяє PostgreSQL ефективно виконувати запити з оператором %
	// без повного сканування таблиці events.
	db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_events_searchable_trgm
		ON events USING GIN (searchable_value gin_trgm_ops)
	`)

	// FK для рядкового поля gate_id (природній ключ, не UUID).
	// GORM не генерує їх автоматично без асоціативних полів у моделях,
	// тому додаємо вручну. DO/EXCEPTION — ідемпотентний варіант без IF NOT EXISTS.
	for _, stmt := range []string{
		`DO $$ BEGIN
			ALTER TABLE transactions ADD CONSTRAINT fk_transactions_gate_id
				FOREIGN KEY (gate_id) REFERENCES gates(gate_id)
				ON UPDATE CASCADE ON DELETE RESTRICT;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
		`DO $$ BEGIN
			ALTER TABLE events ADD CONSTRAINT fk_events_gate_id
				FOREIGN KEY (gate_id) REFERENCES gates(gate_id)
				ON UPDATE CASCADE ON DELETE RESTRICT;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
		`DO $$ BEGIN
			ALTER TABLE device_configs ADD CONSTRAINT fk_device_configs_gate_id
				FOREIGN KEY (gate_id) REFERENCES gates(gate_id)
				ON UPDATE CASCADE ON DELETE RESTRICT;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	} {
		db.Exec(stmt)
	}
}

func InitRedis(addr string) {
	RDB = redis.NewClient(&redis.Options{
		Addr: addr,
	})
}
