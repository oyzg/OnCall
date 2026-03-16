package domain

import "time"

type Message struct {
	ID         string      `json:"id"`
	SessionID  string      `json:"session_id"`
	Role       string      `json:"role"`
	Content    string      `json:"content"`
	Status     string      `json:"status"`
	References []Reference `json:"references,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
}

type Reference struct {
	DocumentID    string  `json:"document_id"`
	DocumentTitle string  `json:"document_title"`
	Category      string  `json:"category"`
	Excerpt       string  `json:"excerpt"`
	Score         float64 `json:"score"`
}
