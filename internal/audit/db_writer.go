package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultBatchSize    = 50
	defaultFlushTimeout = 100 * time.Millisecond
	defaultChanSize     = 1000
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func toNullUUID(s string) sql.NullString {
	if uuidRegex.MatchString(s) {
		return sql.NullString{String: s, Valid: true}
	}
	return sql.NullString{Valid: false}
}

// DBWriter writes audit events asynchronously to the database.
type DBWriter struct {
	db        *sql.DB
	ch        chan Event
	done      chan struct{}
	wg        sync.WaitGroup
	logger    *slog.Logger
	closed    atomic.Bool
	closeOnce sync.Once
}

// NewDBWriter creates and starts a new DBWriter. If db is nil, it returns nil.
func NewDBWriter(db *sql.DB, logger *slog.Logger) *DBWriter {
	if db == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	w := &DBWriter{
		db:     db,
		ch:     make(chan Event, defaultChanSize),
		done:   make(chan struct{}),
		logger: logger,
	}
	w.wg.Add(1)
	go w.worker()
	return w
}

// Write queues an event for writing to the database. It is thread-safe and non-blocking.
func (w *DBWriter) Write(event Event) {
	if w.closed.Load() {
		w.logger.Warn("audit event written to closed DBWriter", "event_type", event.EventType)
		return
	}
	select {
	case w.ch <- event:
	default:
		w.logger.Warn("audit event channel is full, dropping event", "event_type", event.EventType)
	}
}

// Close stops the worker loop and flushes all pending events.
func (w *DBWriter) Close() error {
	var err error
	w.closeOnce.Do(func() {
		w.closed.Store(true)
		close(w.done)
		w.wg.Wait()
		close(w.ch)
	})
	return err
}

func (w *DBWriter) worker() {
	defer w.wg.Done()

	ticker := time.NewTicker(defaultFlushTimeout)
	defer ticker.Stop()

	batch := make([]Event, 0, defaultBatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := w.writeBatch(context.Background(), batch); err != nil {
			w.logger.Error("failed to write audit events batch to database", "error", err, "batch_size", len(batch))
		}
		batch = batch[:0]
	}

	for {
		select {
		case event, ok := <-w.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, event)
			if len(batch) >= defaultBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-w.done:
			// Drain remaining events from channel
			for {
				select {
				case event, ok := <-w.ch:
					if !ok {
						flush()
						return
					}
					batch = append(batch, event)
					if len(batch) >= defaultBatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

func (w *DBWriter) writeBatch(ctx context.Context, batch []Event) error {
	if len(batch) == 0 {
		return nil
	}

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO audit_events (user_id, event_type, datasource_id, model_id, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, event := range batch {
		var detailsJSON []byte
		if len(event.Details) > 0 {
			var err error
			detailsJSON, err = json.Marshal(event.Details)
			if err != nil {
				w.logger.Warn("failed to marshal audit event details", "error", err)
			}
		}

		var detailsVal any
		if len(detailsJSON) > 0 {
			detailsVal = string(detailsJSON)
		}

		var createdAt time.Time
		if event.Timestamp.IsZero() {
			createdAt = time.Now().UTC()
		} else {
			createdAt = event.Timestamp.UTC()
		}

		userIDVal := toNullUUID(event.UserID)
		datasourceIDVal := toNullUUID(event.DatasourceID)
		modelIDVal := toNullUUID(event.ModelID)

		_, err = stmt.ExecContext(ctx,
			userIDVal,
			string(event.EventType),
			datasourceIDVal,
			modelIDVal,
			detailsVal,
			createdAt,
		)
		if err != nil {
			return fmt.Errorf("exec statement: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
