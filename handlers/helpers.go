package handlers

import (
	"bytes"
	"compress/zlib"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"api/test/structs"

	"github.com/google/uuid"
)

// Compression helpers
func compressMessages(messages []structs.StoredMessage) ([]byte, error) {
	data, err := json.Marshal(messages)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(data)
	w.Close()
	return buf.Bytes(), nil
}

func decompressMessages(data []byte) ([]structs.StoredMessage, error) {
	if len(data) == 0 {
		return []structs.StoredMessage{}, nil
	}
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var messages []structs.StoredMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// ConversationManager
type ConversationManager struct {
	ConversationID string
	UserID         string
	Messages       []structs.StoredMessage
	loaded         bool
}

func NewConversationManager(conversationID, userID string) *ConversationManager {
	return &ConversationManager{
		ConversationID: conversationID,
		UserID:         userID,
	}
}

func (cm *ConversationManager) Load(db *sql.DB) error {
	if cm.loaded {
		return nil
	}

	var compressedData []byte
	var ownerID string
	err := db.QueryRow(
		"SELECT compressed_messages, user_id FROM conversations WHERE id = $1::uuid",
		cm.ConversationID,
	).Scan(&compressedData, &ownerID)

	if err == sql.ErrNoRows {
		cm.Messages = []structs.StoredMessage{}
		cm.loaded = true
		return nil
	}
	if err != nil {
		return err
	}
	if ownerID != cm.UserID {
		return fmt.Errorf("forbidden")
	}

	messages, err := decompressMessages(compressedData)
	if err != nil {
		return err
	}

	cm.Messages = messages
	cm.loaded = true
	return nil
}

func (cm *ConversationManager) Append(newMessages []map[string]any) {
	for _, m := range newMessages {
		role, _ := m["role"].(string)
		cm.Messages = append(cm.Messages, structs.StoredMessage{
			ID:        uuid.New().String(),
			Message:   m,
			Role:      role,
			CreatedAt: structs.FlexTime{},
			Metadata:  map[string]any{},
		})
	}
}

func (cm *ConversationManager) Persist(db *sql.DB) error {
	compressed, err := compressMessages(cm.Messages)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"UPDATE conversations SET compressed_messages = $1, updated_at = NOW() WHERE id = $2",
		compressed, cm.ConversationID,
	)
	return err
}

func (cm *ConversationManager) GetMemorySnapshot(recentN int) []structs.LLMMessage {
	messages := cm.Messages
	if len(messages) > recentN {
		messages = messages[len(messages)-recentN:]
	}

	snapshot := []structs.LLMMessage{
		{Role: "system", Content: "You are an assistant aware of the recent conversation context with the user."},
	}
	for _, m := range messages {
		content, _ := m.Message["content"].(string)
		snapshot = append(snapshot, structs.LLMMessage{
			Role:    m.Role,
			Content: content,
		})
	}
	return snapshot
}
