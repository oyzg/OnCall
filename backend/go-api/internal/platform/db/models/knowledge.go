package models

import "time"

type KnowledgeDocument struct {
	ID                string `gorm:"primaryKey;size:64"`
	UserID            string `gorm:"index:idx_knowledge_user_updated,priority:1;size:64;not null"`
	Title             string `gorm:"size:255;not null"`
	Category          string `gorm:"index:idx_knowledge_category_updated,priority:1;size:64;not null"`
	SourceType        string `gorm:"size:16;not null"`
	FileName          string `gorm:"size:255;not null"`
	ContentType       string `gorm:"size:128;not null"`
	StoragePath       string `gorm:"size:512;not null"`
	SizeBytes         int64  `gorm:"not null"`
	Status            string `gorm:"index:idx_knowledge_status_updated,priority:1;size:16;not null"`
	Summary           string `gorm:"type:text;not null"`
	TextPreview       string `gorm:"type:text;not null"`
	ChunkPreviewsJSON string `gorm:"type:longtext;not null"`
	FailureReason     string `gorm:"size:512"`
	ChunkCount        int    `gorm:"not null;default:0"`
	IndexStatus       string `gorm:"size:32"`
	EmbeddingBackend  string `gorm:"size:128"`
	VectorBackend     string `gorm:"size:128"`
	LexicalBackend    string `gorm:"size:128"`
	IndexError        string `gorm:"type:text"`
	ProcessedAt       *time.Time
	IndexedAt         *time.Time
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"index:idx_knowledge_user_updated,priority:2;index:idx_knowledge_status_updated,priority:2;index:idx_knowledge_category_updated,priority:2;not null"`
}

type KnowledgeChunk struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	DocumentID    string    `gorm:"uniqueIndex:idx_knowledge_chunk_doc_index,priority:1;index:idx_knowledge_chunk_doc;size:64;not null"`
	DocumentTitle string    `gorm:"size:255;not null"`
	Category      string    `gorm:"index;size:64;not null"`
	ChunkIndex    int       `gorm:"uniqueIndex:idx_knowledge_chunk_doc_index,priority:2;not null"`
	Content       string    `gorm:"type:longtext;not null"`
	Normalized    string    `gorm:"type:longtext;not null"`
	TermsJSON     string    `gorm:"type:longtext;not null"`
	CharTermsJSON string    `gorm:"type:longtext;not null"`
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (KnowledgeDocument) TableName() string { return "knowledge_documents" }
func (KnowledgeChunk) TableName() string    { return "knowledge_chunks" }
