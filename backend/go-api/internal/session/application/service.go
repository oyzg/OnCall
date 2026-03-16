package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/session/domain"
)

type Service struct {
	mu        sync.RWMutex
	sessions  map[string]domain.Session
	messages  map[string][]domain.Message
	storePath string
}

func NewService() *Service {
	service := &Service{
		sessions:  make(map[string]domain.Session),
		messages:  make(map[string][]domain.Message),
		storePath: resolveStorePath(),
	}
	service.load()
	return service
}

func (s *Service) CreateSession(user authDomain.User, title string) domain.Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	session := domain.Session{
		ID:                 nextID("sess"),
		UserID:             user.ID,
		Title:              normalizeTitle(title),
		LastMessagePreview: "",
		MessageCount:       0,
		CreatedAt:          now,
		UpdatedAt:          now,
		LastMessageAt:      now,
	}

	s.sessions[session.ID] = session
	s.messages[session.ID] = []domain.Message{}
	s.persistLocked()
	return session
}

func (s *Service) ListSessions(user authDomain.User, query string, limit int) []domain.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]domain.Session, 0)
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	for _, session := range s.sessions {
		if session.UserID != user.ID {
			continue
		}
		if normalizedQuery != "" && !strings.Contains(strings.ToLower(session.Title), normalizedQuery) {
			continue
		}
		items = append(items, session)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	return items
}

func (s *Service) DeleteSession(user authDomain.User, sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok || session.UserID != user.ID {
		return false
	}

	delete(s.sessions, sessionID)
	delete(s.messages, sessionID)
	s.persistLocked()
	return true
}

type MessagePage struct {
	Messages   []domain.Message `json:"messages"`
	Total      int              `json:"total"`
	HasMore    bool             `json:"has_more"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

func (s *Service) ListMessages(user authDomain.User, sessionID string, limit int, beforeID string) (MessagePage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok || session.UserID != user.ID {
		return MessagePage{}, false
	}

	items := append([]domain.Message(nil), s.messages[sessionID]...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	total := len(items)
	end := len(items)
	if beforeID != "" {
		for index, message := range items {
			if message.ID == beforeID {
				end = index
				break
			}
		}
	}

	eligible := items[:end]
	hasMore := false
	nextCursor := ""
	if limit > 0 && len(eligible) > limit {
		hasMore = true
		eligible = eligible[len(eligible)-limit:]
		nextCursor = eligible[0].ID
	}

	return MessagePage{
		Messages:   eligible,
		Total:      total,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, true
}

func (s *Service) StartAssistantReply(user authDomain.User, sessionID, content string) (domain.Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok || session.UserID != user.ID {
		return domain.Message{}, false
	}

	now := time.Now()
	userMessage := domain.Message{
		ID:        nextID("msg"),
		SessionID: sessionID,
		Role:      "user",
		Content:   strings.TrimSpace(content),
		Status:    "completed",
		CreatedAt: now,
	}

	assistantMessage := domain.Message{
		ID:         nextID("msg"),
		SessionID:  sessionID,
		Role:       "assistant",
		Content:    "",
		Status:     "streaming",
		References: nil,
		CreatedAt:  now.Add(time.Millisecond),
	}

	s.messages[sessionID] = append(s.messages[sessionID], userMessage, assistantMessage)

	if session.Title == defaultSessionTitle {
		session.Title = titleFromMessage(content)
	}
	s.refreshSessionLocked(sessionID, session, now)
	s.persistLocked()

	return assistantMessage, true
}

func (s *Service) CompleteAssistantReply(user authDomain.User, sessionID, messageID, content string, references []domain.Reference) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok || session.UserID != user.ID {
		return false
	}

	messages := s.messages[sessionID]
	for index := range messages {
		if messages[index].ID != messageID {
			continue
		}
		messages[index].Content = content
		messages[index].Status = "completed"
		messages[index].References = references
		break
	}

	now := time.Now()
	s.refreshSessionLocked(sessionID, session, now)
	s.messages[sessionID] = messages
	s.persistLocked()
	return true
}

func (s *Service) refreshSessionLocked(sessionID string, session domain.Session, now time.Time) {
	messageList := s.messages[sessionID]
	session.UpdatedAt = now
	session.LastMessageAt = now
	session.MessageCount = len(messageList)
	if preview := latestPreview(messageList); preview != "" {
		session.LastMessagePreview = preview
	}
	s.sessions[sessionID] = session
}

type snapshot struct {
	Sessions map[string]domain.Session   `json:"sessions"`
	Messages map[string][]domain.Message `json:"messages"`
}

func (s *Service) load() {
	data, err := os.ReadFile(s.storePath)
	if err != nil {
		return
	}

	var stored snapshot
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	if stored.Sessions != nil {
		s.sessions = stored.Sessions
	}
	if stored.Messages != nil {
		s.messages = stored.Messages
	}
}

func (s *Service) persistLocked() {
	if s.storePath == "" {
		return
	}

	_ = os.MkdirAll(filepath.Dir(s.storePath), 0o755)
	payload, err := json.MarshalIndent(snapshot{
		Sessions: s.sessions,
		Messages: s.messages,
	}, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(s.storePath, payload, 0o644)
}

const defaultSessionTitle = "新会话"

func normalizeTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return defaultSessionTitle
	}
	return trimmed
}

func titleFromMessage(content string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return defaultSessionTitle
	}

	runes := []rune(text)
	if len(runes) > 20 {
		return string(runes[:20]) + "..."
	}
	return text
}

func summarizePreview(content string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}

	runes := []rune(text)
	if len(runes) > 42 {
		return string(runes[:42]) + "..."
	}
	return text
}

func latestPreview(messages []domain.Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if preview := summarizePreview(messages[index].Content); preview != "" {
			return preview
		}
	}
	return ""
}

func resolveStorePath() string {
	if value := strings.TrimSpace(os.Getenv("ONCALL_SESSION_STORE")); value != "" {
		return value
	}

	path, err := filepath.Abs(filepath.Join(".", "..", "..", "tmp", "dev", "session-store.json"))
	if err != nil {
		return ""
	}
	return path
}

func nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
