package dbutil
package dbutil

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TxOptions represents transaction options
type TxOptions struct {
	Isolation sql.IsolationLevel
	ReadOnly  bool
	Timeout   time.Duration
}

// DefaultTxOptions provides sensible transaction defaults
var DefaultTxOptions = TxOptions{
	Isolation: sql.LevelDefault,
	ReadOnly:  false,
	Timeout:   30 * time.Second,
}

// DB interface for database operations (allows for easy testing)
type DB interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	PingContext(ctx context.Context) error
}

// TxFunc represents a function that operates within a transaction
type TxFunc func(tx *sql.Tx) error

// QueryFunc represents a function for read operations
type QueryFunc func(ctx context.Context, db DB) error

// ExecFunc represents a function for write operations
type ExecFunc func(ctx context.Context, db DB) (sql.Result, error)

// Wrapper provides database operation utilities
type Wrapper struct {
	db      DB
	timeout time.Duration
}

// NewWrapper creates a new database wrapper
func NewWrapper(db DB, timeout time.Duration) *Wrapper {
	return &Wrapper{
		db:      db,
		timeout: timeout,
	}
}

// WithTransaction executes a function within a database transaction
func (w *Wrapper) WithTransaction(ctx context.Context, fn TxFunc, opts ...TxOptions) error {
	return w.WithTransactionResult(ctx, func(tx *sql.Tx) (interface{}, error) {
		return nil, fn(tx)
	}, opts...)
}

// WithTransactionResult executes a function within a transaction and returns a result
func (w *Wrapper) WithTransactionResult(ctx context.Context, fn func(tx *sql.Tx) (interface{}, error), opts ...TxOptions) error {
	_, err := w.ExecWithTransactionResult(ctx, fn, opts...)
	return err
}

// ExecWithTransactionResult executes a function within a transaction and returns both result and error
func (w *Wrapper) ExecWithTransactionResult(ctx context.Context, fn func(tx *sql.Tx) (interface{}, error), opts ...TxOptions) (interface{}, error) {
	var options TxOptions
	if len(opts) > 0 {
		options = opts[0]
	} else {
		options = DefaultTxOptions
	}

	// Create context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	// Begin transaction
	sqlOpts := &sql.TxOptions{
		Isolation: options.Isolation,
		ReadOnly:  options.ReadOnly,
	}
	
	tx, err := w.db.BeginTx(ctxWithTimeout, sqlOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute function
	result, err := fn(tx)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, fmt.Errorf("transaction failed with error: %v, rollback also failed: %w", err, rollbackErr)
		}
		return nil, fmt.Errorf("transaction rolled back due to error: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

// QueryWithTimeout executes a read operation with timeout
func (w *Wrapper) QueryWithTimeout(ctx context.Context, fn QueryFunc, timeout ...time.Duration) error {
	var t time.Duration
	if len(timeout) > 0 {
		t = timeout[0]
	} else {
		t = w.timeout
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	return fn(ctxWithTimeout, w.db)
}

// ExecWithTimeout executes a write operation with timeout
func (w *Wrapper) ExecWithTimeout(ctx context.Context, fn ExecFunc, timeout ...time.Duration) (sql.Result, error) {
	var t time.Duration
	if len(timeout) > 0 {
		t = timeout[0]
	} else {
		t = w.timeout
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	return fn(ctxWithTimeout, w.db)
}

// PingWithTimeout checks database connectivity with timeout
func (w *Wrapper) PingWithTimeout(ctx context.Context, timeout ...time.Duration) error {
	var t time.Duration
	if len(timeout) > 0 {
		t = timeout[0]
	} else {
		t = w.timeout
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	return w.db.PingContext(ctxWithTimeout)
}

// Common database operations helpers

// ExecQuery executes a query and returns the result
func (w *Wrapper) ExecQuery(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return w.ExecWithTimeout(ctx, func(ctx context.Context, db DB) (sql.Result, error) {
		return db.ExecContext(ctx, query, args...)
	})
}

// QueryRow executes a query that returns a single row
func (w *Wrapper) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()
	
	return w.db.QueryRowContext(ctxWithTimeout, query, args...)
}

// QueryRows executes a query that returns multiple rows
func (w *Wrapper) QueryRows(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()
	
	return w.db.QueryContext(ctxWithTimeout, query, args...)
}

// SaveWithRetry attempts to save data with retry logic for transaction conflicts
func (w *Wrapper) SaveWithRetry(ctx context.Context, fn TxFunc, maxRetries int) error {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := w.WithTransaction(ctx, fn)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if error is retryable (e.g., deadlock, busy database)
		if !isRetryableError(err) {
			return err
		}
		
		if attempt < maxRetries {
			// Wait before retry with exponential backoff
			waitTime := time.Duration(attempt+1) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
				// Continue to next retry
			}
		}
	}
	
	return fmt.Errorf("operation failed after %d retries, last error: %w", maxRetries, lastErr)
}

// isRetryableError determines if a database error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	// Common retryable SQLite errors
	retryableErrors := []string{
		"database is locked",
		"database is busy",
		"deadlock",
		"cannot start a transaction within a transaction",
	}
	
	for _, retryableErr := range retryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}
	
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	return len(str) >= len(substr) && 
		   (str == substr || 
		    (len(str) > len(substr) && 
		     findSubstring(str, substr)))
}

// findSubstring performs case-insensitive substring search
func findSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLowerCase(str[i+j]) != toLowerCase(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// toLowerCase converts a byte to lowercase
func toLowerCase(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}