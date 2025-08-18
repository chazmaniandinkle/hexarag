package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/username/hexarag/internal/domain/entities"
	"github.com/username/hexarag/internal/domain/ports"
)

// Adapter implements the StoragePort interface using SQLite
type Adapter struct {
	db             *sql.DB
	migrationsPath string
}

// NewAdapter creates a new SQLite storage adapter
func NewAdapter(dbPath, migrationsPath string) (*Adapter, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	adapter := &Adapter{
		db:             db,
		migrationsPath: migrationsPath,
	}

	return adapter, nil
}

// Migrate runs database migrations
func (a *Adapter) Migrate(ctx context.Context) error {
	// Create migrations table if it doesn't exist
	_, err := a.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	appliedMigrations := make(map[string]bool)
	rows, err := a.db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("failed to scan migration version: %w", err)
		}
		appliedMigrations[version] = true
	}

	// Read migration files
	migrationFiles, err := filepath.Glob(filepath.Join(a.migrationsPath, "*.sql"))
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	sort.Strings(migrationFiles)

	// Apply new migrations
	for _, file := range migrationFiles {
		version := strings.TrimSuffix(filepath.Base(file), ".sql")
		if appliedMigrations[version] {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		// Execute migration in transaction
		tx, err := a.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin migration transaction: %w", err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", version, err)
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", version, err)
		}
	}

	return nil
}

// Ping checks database connectivity
func (a *Adapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

// Close closes the database connection
func (a *Adapter) Close() error {
	return a.db.Close()
}

// Message operations
func (a *Adapter) SaveMessage(ctx context.Context, message *entities.Message) error {
	query := `
		INSERT INTO messages (id, conversation_id, role, content, parent_message_id, token_count, model, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := a.db.ExecContext(ctx, query,
		message.ID,
		message.ConversationID,
		string(message.Role),
		message.Content,
		message.ParentID,
		message.TokenCount,
		message.Model,
		message.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	// Save tool calls if present
	for _, toolCall := range message.ToolCalls {
		if err := a.SaveToolCall(ctx, &toolCall); err != nil {
			return fmt.Errorf("failed to save tool call: %w", err)
		}
	}

	return nil
}

func (a *Adapter) GetMessage(ctx context.Context, id string) (*entities.Message, error) {
	query := `
		SELECT id, conversation_id, role, content, parent_message_id, token_count, model, created_at
		FROM messages WHERE id = ?
	`

	row := a.db.QueryRowContext(ctx, query, id)

	var message entities.Message
	var parentID sql.NullString
	var model sql.NullString

	err := row.Scan(
		&message.ID,
		&message.ConversationID,
		&message.Role,
		&message.Content,
		&parentID,
		&message.TokenCount,
		&model,
		&message.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("message not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	if parentID.Valid {
		message.ParentID = &parentID.String
	}
	if model.Valid {
		message.Model = model.String
	}

	// Load tool calls
	toolCalls, err := a.GetToolCallsForMessage(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load tool calls: %w", err)
	}

	for _, tc := range toolCalls {
		message.ToolCalls = append(message.ToolCalls, *tc)
	}

	return &message, nil
}

func (a *Adapter) GetMessages(ctx context.Context, conversationID string, limit int) ([]*entities.Message, error) {
	query := `
		SELECT id, conversation_id, role, content, parent_message_id, token_count, model, created_at
		FROM messages 
		WHERE conversation_id = ? 
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := a.db.QueryContext(ctx, query, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []*entities.Message
	for rows.Next() {
		var message entities.Message
		var parentID sql.NullString
		var model sql.NullString

		err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.Role,
			&message.Content,
			&parentID,
			&message.TokenCount,
			&model,
			&message.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if parentID.Valid {
			message.ParentID = &parentID.String
		}
		if model.Valid {
			message.Model = model.String
		}

		messages = append(messages, &message)
	}

	// Load tool calls for all messages
	for _, msg := range messages {
		toolCalls, err := a.GetToolCallsForMessage(ctx, msg.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load tool calls for message %s: %w", msg.ID, err)
		}

		for _, tc := range toolCalls {
			msg.ToolCalls = append(msg.ToolCalls, *tc)
		}
	}

	return messages, nil
}

func (a *Adapter) GetMessagesAfter(ctx context.Context, conversationID string, afterID string, limit int) ([]*entities.Message, error) {
	// Get the timestamp of the "after" message
	var afterTime time.Time
	err := a.db.QueryRowContext(ctx, "SELECT created_at FROM messages WHERE id = ?", afterID).Scan(&afterTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get after message timestamp: %w", err)
	}

	query := `
		SELECT id, conversation_id, role, content, parent_message_id, token_count, model, created_at
		FROM messages 
		WHERE conversation_id = ? AND created_at > ?
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := a.db.QueryContext(ctx, query, conversationID, afterTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages after: %w", err)
	}
	defer rows.Close()

	var messages []*entities.Message
	for rows.Next() {
		var message entities.Message
		var parentID sql.NullString
		var model sql.NullString

		err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.Role,
			&message.Content,
			&parentID,
			&message.TokenCount,
			&model,
			&message.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if parentID.Valid {
			message.ParentID = &parentID.String
		}
		if model.Valid {
			message.Model = model.String
		}

		messages = append(messages, &message)
	}

	// Load tool calls for all messages
	for _, msg := range messages {
		toolCalls, err := a.GetToolCallsForMessage(ctx, msg.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load tool calls for message %s: %w", msg.ID, err)
		}

		for _, tc := range toolCalls {
			msg.ToolCalls = append(msg.ToolCalls, *tc)
		}
	}

	return messages, nil
}

// Conversation operations
func (a *Adapter) SaveConversation(ctx context.Context, conversation *entities.Conversation) error {
	query := `
		INSERT INTO conversations (id, title, system_prompt_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := a.db.ExecContext(ctx, query,
		conversation.ID,
		conversation.Title,
		conversation.SystemPromptID,
		conversation.CreatedAt,
		conversation.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w", err)
	}

	return nil
}

func (a *Adapter) GetConversation(ctx context.Context, id string) (*entities.Conversation, error) {
	query := `
		SELECT id, title, system_prompt_id, created_at, updated_at
		FROM conversations WHERE id = ?
	`

	row := a.db.QueryRowContext(ctx, query, id)

	var conversation entities.Conversation
	var title sql.NullString

	err := row.Scan(
		&conversation.ID,
		&title,
		&conversation.SystemPromptID,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conversation not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	if title.Valid {
		conversation.Title = title.String
	}

	// Load message IDs
	messageRows, err := a.db.QueryContext(ctx,
		"SELECT id FROM messages WHERE conversation_id = ? ORDER BY created_at ASC", id)
	if err != nil {
		return nil, fmt.Errorf("failed to load message IDs: %w", err)
	}
	defer messageRows.Close()

	for messageRows.Next() {
		var messageID string
		if err := messageRows.Scan(&messageID); err != nil {
			return nil, fmt.Errorf("failed to scan message ID: %w", err)
		}
		conversation.MessageIDs = append(conversation.MessageIDs, messageID)
	}

	return &conversation, nil
}

func (a *Adapter) GetConversations(ctx context.Context, limit int, offset int) ([]*entities.Conversation, error) {
	query := `
		SELECT id, title, system_prompt_id, created_at, updated_at
		FROM conversations 
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := a.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversations: %w", err)
	}
	defer rows.Close()

	var conversations []*entities.Conversation
	for rows.Next() {
		var conversation entities.Conversation
		var title sql.NullString

		err := rows.Scan(
			&conversation.ID,
			&title,
			&conversation.SystemPromptID,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}

		if title.Valid {
			conversation.Title = title.String
		}

		conversations = append(conversations, &conversation)
	}

	// Load message IDs for each conversation
	for _, conv := range conversations {
		messageRows, err := a.db.QueryContext(ctx,
			"SELECT id FROM messages WHERE conversation_id = ? ORDER BY created_at ASC", conv.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load message IDs for conversation %s: %w", conv.ID, err)
		}

		for messageRows.Next() {
			var messageID string
			if err := messageRows.Scan(&messageID); err != nil {
				messageRows.Close()
				return nil, fmt.Errorf("failed to scan message ID: %w", err)
			}
			conv.MessageIDs = append(conv.MessageIDs, messageID)
		}
		messageRows.Close()
	}

	return conversations, nil
}

func (a *Adapter) UpdateConversation(ctx context.Context, conversation *entities.Conversation) error {
	query := `
		UPDATE conversations 
		SET title = ?, system_prompt_id = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := a.db.ExecContext(ctx, query,
		conversation.Title,
		conversation.SystemPromptID,
		conversation.UpdatedAt,
		conversation.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}

	return nil
}

func (a *Adapter) DeleteConversation(ctx context.Context, id string) error {
	// SQLite will cascade delete messages and tool calls due to foreign key constraints
	query := "DELETE FROM conversations WHERE id = ?"

	_, err := a.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return nil
}

// System prompt operations
func (a *Adapter) SaveSystemPrompt(ctx context.Context, prompt *entities.SystemPrompt) error {
	query := `
		INSERT INTO system_prompts (id, name, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := a.db.ExecContext(ctx, query,
		prompt.ID,
		prompt.Name,
		prompt.Content,
		prompt.CreatedAt,
		prompt.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save system prompt: %w", err)
	}

	return nil
}

func (a *Adapter) GetSystemPrompt(ctx context.Context, id string) (*entities.SystemPrompt, error) {
	query := `
		SELECT id, name, content, created_at, updated_at
		FROM system_prompts WHERE id = ?
	`

	row := a.db.QueryRowContext(ctx, query, id)

	var prompt entities.SystemPrompt

	err := row.Scan(
		&prompt.ID,
		&prompt.Name,
		&prompt.Content,
		&prompt.CreatedAt,
		&prompt.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("system prompt not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get system prompt: %w", err)
	}

	return &prompt, nil
}

func (a *Adapter) GetSystemPrompts(ctx context.Context) ([]*entities.SystemPrompt, error) {
	query := `
		SELECT id, name, content, created_at, updated_at
		FROM system_prompts 
		ORDER BY name ASC
	`

	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompts: %w", err)
	}
	defer rows.Close()

	var prompts []*entities.SystemPrompt
	for rows.Next() {
		var prompt entities.SystemPrompt

		err := rows.Scan(
			&prompt.ID,
			&prompt.Name,
			&prompt.Content,
			&prompt.CreatedAt,
			&prompt.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan system prompt: %w", err)
		}

		prompts = append(prompts, &prompt)
	}

	return prompts, nil
}

func (a *Adapter) UpdateSystemPrompt(ctx context.Context, prompt *entities.SystemPrompt) error {
	query := `
		UPDATE system_prompts 
		SET name = ?, content = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := a.db.ExecContext(ctx, query,
		prompt.Name,
		prompt.Content,
		prompt.UpdatedAt,
		prompt.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update system prompt: %w", err)
	}

	return nil
}

func (a *Adapter) DeleteSystemPrompt(ctx context.Context, id string) error {
	query := "DELETE FROM system_prompts WHERE id = ?"

	_, err := a.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete system prompt: %w", err)
	}

	return nil
}

// Tool call operations
func (a *Adapter) SaveToolCall(ctx context.Context, toolCall *entities.ToolCall) error {
	argumentsJSON, err := toolCall.ArgumentsJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal tool call arguments: %w", err)
	}

	resultJSON, err := toolCall.ResultJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal tool call result: %w", err)
	}

	query := `
		INSERT INTO tool_calls (id, message_id, tool_name, arguments, result, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = a.db.ExecContext(ctx, query,
		toolCall.ID,
		toolCall.MessageID,
		toolCall.Name,
		argumentsJSON,
		resultJSON,
		string(toolCall.Status),
		toolCall.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save tool call: %w", err)
	}

	return nil
}

func (a *Adapter) GetToolCall(ctx context.Context, id string) (*entities.ToolCall, error) {
	query := `
		SELECT id, message_id, tool_name, arguments, result, status, created_at
		FROM tool_calls WHERE id = ?
	`

	row := a.db.QueryRowContext(ctx, query, id)

	var toolCall entities.ToolCall
	var argumentsJSON, resultJSON sql.NullString

	err := row.Scan(
		&toolCall.ID,
		&toolCall.MessageID,
		&toolCall.Name,
		&argumentsJSON,
		&resultJSON,
		&toolCall.Status,
		&toolCall.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tool call not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get tool call: %w", err)
	}

	// Unmarshal arguments
	if argumentsJSON.Valid {
		if err := json.Unmarshal([]byte(argumentsJSON.String), &toolCall.Arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
		}
	}

	// Unmarshal result
	if resultJSON.Valid && resultJSON.String != "" {
		if err := json.Unmarshal([]byte(resultJSON.String), &toolCall.Result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool call result: %w", err)
		}
	}

	return &toolCall, nil
}

func (a *Adapter) GetToolCallsForMessage(ctx context.Context, messageID string) ([]*entities.ToolCall, error) {
	query := `
		SELECT id, message_id, tool_name, arguments, result, status, created_at
		FROM tool_calls 
		WHERE message_id = ?
		ORDER BY created_at ASC
	`

	rows, err := a.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool calls: %w", err)
	}
	defer rows.Close()

	var toolCalls []*entities.ToolCall
	for rows.Next() {
		var toolCall entities.ToolCall
		var argumentsJSON, resultJSON sql.NullString

		err := rows.Scan(
			&toolCall.ID,
			&toolCall.MessageID,
			&toolCall.Name,
			&argumentsJSON,
			&resultJSON,
			&toolCall.Status,
			&toolCall.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool call: %w", err)
		}

		// Unmarshal arguments
		if argumentsJSON.Valid {
			if err := json.Unmarshal([]byte(argumentsJSON.String), &toolCall.Arguments); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
			}
		}

		// Unmarshal result
		if resultJSON.Valid && resultJSON.String != "" {
			if err := json.Unmarshal([]byte(resultJSON.String), &toolCall.Result); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool call result: %w", err)
			}
		}

		toolCalls = append(toolCalls, &toolCall)
	}

	return toolCalls, nil
}

func (a *Adapter) UpdateToolCall(ctx context.Context, toolCall *entities.ToolCall) error {
	resultJSON, err := toolCall.ResultJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal tool call result: %w", err)
	}

	query := `
		UPDATE tool_calls 
		SET result = ?, status = ?
		WHERE id = ?
	`

	_, err = a.db.ExecContext(ctx, query,
		resultJSON,
		string(toolCall.Status),
		toolCall.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update tool call: %w", err)
	}

	return nil
}

// Event operations
func (a *Adapter) SaveEvent(ctx context.Context, conversationID, eventType string, payload map[string]interface{}) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	eventID := fmt.Sprintf("%d_%x", time.Now().UnixNano(), []byte(eventType + conversationID)[:4])

	query := `
		INSERT INTO events (id, conversation_id, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err = a.db.ExecContext(ctx, query, eventID, conversationID, eventType, string(payloadJSON))
	if err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}

	return nil
}

func (a *Adapter) GetEvents(ctx context.Context, conversationID string, limit int) ([]ports.Event, error) {
	query := `
		SELECT id, conversation_id, event_type, payload, created_at
		FROM events 
		WHERE conversation_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := a.db.QueryContext(ctx, query, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}
	defer rows.Close()

	var events []ports.Event
	for rows.Next() {
		var event ports.Event
		var payloadJSON string

		err := rows.Scan(
			&event.ID,
			&event.ConversationID,
			&event.EventType,
			&payloadJSON,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		// Unmarshal payload
		if err := json.Unmarshal([]byte(payloadJSON), &event.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event payload: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}
