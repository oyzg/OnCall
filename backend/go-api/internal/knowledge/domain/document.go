package domain

import "time"

type Document struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	Title         string     `json:"title"`
	Category      string     `json:"category"`
	SourceType    string     `json:"source_type"`
	FileName      string     `json:"file_name"`
	ContentType   string     `json:"content_type"`
	StoragePath   string     `json:"storage_path"`
	SizeBytes     int64      `json:"size_bytes"`
	Status        string     `json:"status"`
	Summary       string     `json:"summary"`
	TextPreview   string     `json:"text_preview"`
	ChunkPreviews []string   `json:"chunk_previews"`
	FailureReason string     `json:"failure_reason,omitempty"`
	ChunkCount    int        `json:"chunk_count"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
}

type Reference struct {
	DocumentID    string  `json:"document_id"`
	DocumentTitle string  `json:"document_title"`
	Category      string  `json:"category"`
	Excerpt       string  `json:"excerpt"`
	Score         float64 `json:"score"`
}
