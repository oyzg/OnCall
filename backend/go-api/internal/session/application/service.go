package application

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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

func (s *Service) StartAssistantReply(user authDomain.User, sessionID, content string) (domain.Message, []string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok || session.UserID != user.ID {
		return domain.Message{}, nil, false
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

	replyContent := buildReply(content)
	assistantMessage := domain.Message{
		ID:        nextID("msg"),
		SessionID: sessionID,
		Role:      "assistant",
		Content:   "",
		Status:    "streaming",
		CreatedAt: now.Add(time.Millisecond),
	}

	s.messages[sessionID] = append(s.messages[sessionID], userMessage, assistantMessage)

	if session.Title == defaultSessionTitle {
		session.Title = titleFromMessage(content)
	}
	session.UpdatedAt = now
	session.LastMessageAt = now
	s.sessions[sessionID] = session

	return assistantMessage, splitChunks(replyContent, 18), true
}

func (s *Service) CompleteAssistantReply(user authDomain.User, sessionID, messageID, content string) bool {
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

func buildReply(content string) string {
	return fmt.Sprintf(
		"已收到你的问题：%s\n\n当前是阶段 5 的会话流式占位回复，下一阶段会在这里接入知识检索、工具调用和 AI 编排能力。",
		strings.TrimSpace(content),
	)
}

func splitChunks(content string, chunkSize int) []string {
	if chunkSize <= 0 || utf8.RuneCountInString(content) <= chunkSize {
		return []string{content}
	}

	runes := []rune(content)
	chunks := make([]string, 0, len(runes)/chunkSize+1)
	for start := 0; start < len(runes); start += chunkSize {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
