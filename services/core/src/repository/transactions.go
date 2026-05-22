package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
	"gorm.io/gorm"
)

func CreateTransaction(tx *models.Transaction) *models.Transaction {
	if tx.ID == uuid.Nil {
		tx.ID = uuid.New()
	}
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

// GetTransactionRaw fetches only id and gate_id — used by the matchmaker for
// lightweight existence and gate-ownership checks without loading events.
func GetTransactionRaw(id uuid.UUID) *models.Transaction {
	var tx models.Transaction
	if err := DB.Select("id", "gate_id").First(&tx, id).Error; err != nil {
		return nil
	}
	return &tx
}

type TransactionFilter struct {
	GateID  string
	Search  string
	Open    bool // if true, only return currently-open transactions (Valkey key exists)
	Page    int
	Limit   int
	StartAt *time.Time
	EndAt   *time.Time
}

func ListTransactions(f TransactionFilter) ([]models.Transaction, int64) {
	var txs []models.Transaction
	var total int64

	q := DB.Model(&models.Transaction{})
	if f.GateID != "" {
		q = q.Where("gate_id = ?", f.GateID)
	}
	if f.StartAt != nil {
		q = q.Where("created_at >= ?", f.StartAt)
	}
	if f.EndAt != nil {
		q = q.Where("created_at <= ?", f.EndAt)
	}
	if f.Search != "" {
		// Шукаємо одночасно:
		//   1. за кодом транзакції (TRK-…) — для пошуку за ID
		//   2. за searchable_value в подіях — точний підрядок (ILIKE) АБО
		//      нечіткий збіг через pg_trgm (оператор %) для опечаток.
		// EXISTS зупиняється на першому збігу і використовує GIN-індекс
		// idx_events_searchable_trgm.
		pattern := "%" + strings.ToUpper(f.Search) + "%"
		trgm := strings.ToUpper(f.Search)
		q = q.Where(
			`code ILIKE ? OR EXISTS (
				SELECT 1 FROM events
				WHERE events.transaction_id = transactions.id
				  AND (events.searchable_value ILIKE ? OR events.searchable_value % ?)
			)`,
			"%"+f.Search+"%", pattern, trgm,
		)
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

// CloseTransaction видаляє Valkey-ключ tx_active:{gateID}, закриваючи активну транзакцію.
// Повертає (found=false) якщо транзакція не існує, (wasOpen=false) якщо вже закрита.
func CloseTransaction(id uuid.UUID) (found bool, wasOpen bool) {
	tx := GetTransactionRaw(id)
	if tx == nil {
		return false, false
	}
	key := fmt.Sprintf("tx_active:%s", tx.GateID)
	val, err := RDB.Get(context.Background(), key).Result()
	if err != nil || val != id.String() {
		return true, false
	}
	RDB.Del(context.Background(), key)
	return true, true
}

func UpdateTransaction(tx *models.Transaction) error {
	return DB.Save(tx).Error
}

func DeleteTransaction(id uuid.UUID) error {
	return DB.Delete(&models.Transaction{}, id).Error
}

type TransactionNeighbours struct {
	PrevID *uuid.UUID
	NextID *uuid.UUID
}

// GetTransactionNeighbours returns the IDs of adjacent transactions by created_at.
func GetTransactionNeighbours(id uuid.UUID, createdAt time.Time) TransactionNeighbours {
	result := TransactionNeighbours{}

	var prev struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	if err := DB.Model(&models.Transaction{}).
		Select("id").
		Where("created_at < ? AND id != ?", createdAt, id).
		Order("created_at DESC").
		Limit(1).
		Scan(&prev).Error; err == nil && prev.ID != uuid.Nil {
		pid := prev.ID
		result.PrevID = &pid
	}

	var next struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	if err := DB.Model(&models.Transaction{}).
		Select("id").
		Where("created_at > ? AND id != ?", createdAt, id).
		Order("created_at ASC").
		Limit(1).
		Scan(&next).Error; err == nil && next.ID != uuid.Nil {
		nid := next.ID
		result.NextID = &nid
	}

	return result
}

// CountEventsForTransaction returns the number of events attached to a transaction.
func CountEventsForTransaction(txID uuid.UUID) int64 {
	var count int64
	DB.Model(&models.Event{}).Where("transaction_id = ?", txID).Count(&count)
	return count
}

// FindPastTransactionsFuzzy повертає до limit закритих транзакцій, в яких
// зафіксований номер авто (searchable_value) є нечітко схожим на detectedPlate.
//
// Алгоритм двох етапів — повністю на рівні PostgreSQL:
//
//  1. Оператор % (pg_trgm): швидке відсіювання через GIN-індекс.
//     Відкидає рядки, схожість яких нижча за similarity_threshold (за замовч. 0.3).
//
//  2. levenshtein_less_equal(a, b, 2) <= 2 (fuzzystrmatch): точна перевірка.
//     Зупиняється достроково, якщо відстань вже перевищила поріг — ефективніше
//     за звичайний levenshtein().
//
// Приймає db *gorm.DB явно, щоб caller міг передати DB.WithContext(ctx)
// для коректного propagation OpenTelemetry-трейсів.
func FindPastTransactionsFuzzy(db *gorm.DB, detectedPlate string, limit int) ([]models.Transaction, error) {
	// Нормалізація: та сама логіка, що у BeforeSave хуку моделі Event.
	plate := strings.ToUpper(strings.ReplaceAll(detectedPlate, " ", ""))
	if plate == "" || limit <= 0 {
		return nil, nil
	}

	// Крок 1: знаходимо унікальні transaction_id через таблицю events.
	// GIN-індекс idx_events_searchable_trgm обробляє фільтрацію по `%`,
	// levenshtein_less_equal уточнює результат без повного сканування.
	var txIDs []uuid.UUID
	err := db.Raw(`
		SELECT DISTINCT transaction_id
		FROM events
		WHERE searchable_value % ?
		  AND levenshtein_less_equal(searchable_value, ?, 2) <= 2
		  AND transaction_id IS NOT NULL
	`, plate, plate).Scan(&txIDs).Error
	if err != nil {
		return nil, fmt.Errorf("FindPastTransactionsFuzzy: query events: %w", err)
	}
	if len(txIDs) == 0 {
		return nil, nil
	}

	// Крок 2: завантажуємо самі транзакції за знайденими ID.
	var txs []models.Transaction
	err = db.Where("id IN ?", txIDs).
		Preload("Events").
		Order("created_at DESC").
		Limit(limit).
		Find(&txs).Error
	if err != nil {
		return nil, fmt.Errorf("FindPastTransactionsFuzzy: query transactions: %w", err)
	}

	// Перевіряємо статус відкритості через Valkey (батч-запит — один RTT на gate).
	annotateOpen(txs)

	// Відфільтровуємо відкриті транзакції: нас цікавить тільки завершена історія.
	closed := txs[:0]
	for _, tx := range txs {
		if !tx.IsOpen {
			closed = append(closed, tx)
		}
	}

	return closed, nil
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
	seen := map[string]string{} // gateID → active txID
	gates := make([]string, 0)
	for _, tx := range txs {
		if _, ok := seen[tx.GateID]; !ok {
			seen[tx.GateID] = ""
			gates = append(gates, tx.GateID)
		}
	}
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
