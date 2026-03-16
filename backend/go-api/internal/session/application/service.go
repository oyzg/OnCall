package application

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/session/domain"
)

type Service struct {
	mu       sync.RWMutex
	sessions map[string]domain.Session
	messages map[string][]domain.Message
}

func NewService() *Service {
	return &Service{
		sessions: make(map[string]domain.Session),
		messages: make(map[string][]domain.Message),
	}
}

func (s *Service) CreateSession(user authDomain.User, title string) domain.Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	session := domain.Session{
		ID:            nextID("sess"),
		UserID:        user.ID,
		Title:         normalizeTitle(title),
		CreatedAt:     now,
		UpdatedAt:     now,
		LastMessageAt: now,
	}

	s.sessions[session.ID] = session
	s.messages[session.ID] = []domain.Message{}
	return session
}

func (s *Service) ListSessions(user authDomain.User) []domain.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]domain.Session, 0)
	for _, session := range s.sessions {
		if session.UserID != user.ID {
			continue
		}
		items = append(items, session)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

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
	return true
}

func (s *Service) ListMessages(user authDomain.User, sessionID string) ([]domain.Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok || session.UserID != user.ID {
		return nil, false
	}

	items := append([]domain.Message(nil), s.messages[sessionID]...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, true
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
	session.UpdatedAt = now
	session.LastMessageAt = now
	s.sessions[sessionID] = session

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
	session.UpdatedAt = now
	session.LastMessageAt = now
	s.sessions[sessionID] = session
	s.messages[sessionID] = messages
	return true
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

func nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
