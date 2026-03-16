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

	auditDomain "github.com/oyzg/OnCall/backend/go-api/internal/audit/domain"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
)

type RecordInput struct {
	Category   string
	Action     string
	Status     string
	Actor      *authDomain.User
	TargetType string
	TargetID   string
	TargetName string
	Detail     string
	Metadata   map[string]any
}

type ListOptions struct {
	Category string
	Action   string
	Status   string
	Actor    string
	Limit    int
}

type Service struct {
	mu          sync.RWMutex
	logs        []auditDomain.Log
	storagePath string
}

func NewService() *Service {
	service := &Service{
		logs:        make([]auditDomain.Log, 0, 128),
		storagePath: filepath.Join("tmp", "audit", "audit-logs.json"),
	}
	service.load()
	return service
}

func (s *Service) Record(input RecordInput) auditDomain.Log {
	s.mu.Lock()
	defer s.mu.Unlock()

	log := auditDomain.Log{
		ID:         nextID("audit"),
		Category:   fallback(input.Category, "system"),
		Action:     fallback(input.Action, "unknown"),
		Status:     normalizeStatus(input.Status),
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
		TargetName: input.TargetName,
		Detail:     strings.TrimSpace(input.Detail),
		Metadata:   cloneMetadata(input.Metadata),
		CreatedAt:  time.Now(),
	}
	if input.Actor != nil {
		log.ActorID = input.Actor.ID
		log.ActorName = actorName(*input.Actor)
		log.ActorRoles = append([]string(nil), input.Actor.Roles...)
	}

	s.logs = append([]auditDomain.Log{log}, s.logs...)
	if len(s.logs) > 1000 {
		s.logs = s.logs[:1000]
	}
	s.persistLocked()
	return log
}

func (s *Service) List(options ListOptions) []auditDomain.Log {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := options.Limit
	if limit <= 0 {
		limit = 50
	}

	actorQuery := strings.ToLower(strings.TrimSpace(options.Actor))
	items := make([]auditDomain.Log, 0, limit)
	for _, log := range s.logs {
		if options.Category != "" && log.Category != options.Category {
			continue
		}
		if options.Action != "" && log.Action != options.Action {
			continue
		}
		if options.Status != "" && log.Status != options.Status {
			continue
		}
		if actorQuery != "" &&
			!strings.Contains(strings.ToLower(log.ActorName), actorQuery) &&
			!strings.Contains(strings.ToLower(log.ActorID), actorQuery) {
			continue
		}
		items = append(items, log)
		if len(items) >= limit {
			break
		}
	}
	return items
}

func (s *Service) BuildStats() auditDomain.Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := auditDomain.Stats{
		ByCategory: make(map[string]int),
	}
	uniqueActors := make(map[string]struct{})
	deadline := time.Now().Add(-24 * time.Hour)

	for _, log := range s.logs {
		stats.Total++
		stats.ByCategory[log.Category]++
		if log.Status == "success" {
			stats.Success++
		} else {
			stats.Failed++
		}
		if !log.CreatedAt.Before(deadline) {
			stats.Last24Hours++
		}
		if log.ActorID != "" {
			uniqueActors[log.ActorID] = struct{}{}
		}
	}

	stats.UniqueActors = len(uniqueActors)
	return stats
}

func (s *Service) Categories() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set := make(map[string]struct{})
	for _, log := range s.logs {
		if log.Category != "" {
			set[log.Category] = struct{}{}
		}
	}

	items := make([]string, 0, len(set))
	for category := range set {
		items = append(items, category)
	}
	sort.Strings(items)
	return items
}

func (s *Service) load() {
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return
	}

	var logs []auditDomain.Log
	if err := json.Unmarshal(data, &logs); err != nil {
		return
	}
	s.logs = logs
}

func (s *Service) persistLocked() {
	_ = os.MkdirAll(filepath.Dir(s.storagePath), 0o755)
	payload, err := json.MarshalIndent(s.logs, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.storagePath, payload, 0o644)
}

func cloneMetadata(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func normalizeStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "success", "failed":
		return strings.TrimSpace(strings.ToLower(status))
	default:
		return "success"
	}
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return strings.TrimSpace(value)
}

func actorName(user authDomain.User) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}

func nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
