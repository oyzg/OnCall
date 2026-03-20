package application

import (
	"context"

	knowledgeDomain "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/domain"
)

type Repository interface {
	SaveDocument(ctx context.Context, document knowledgeDomain.Document) error
	ListDocuments(ctx context.Context, userID, status, category, query string, limit int) ([]knowledgeDomain.Document, error)
	GetDocument(ctx context.Context, userID, documentID string) (knowledgeDomain.Document, bool, error)
	GetDocumentByID(ctx context.Context, documentID string) (knowledgeDomain.Document, bool, error)
	DeleteDocument(ctx context.Context, userID, documentID string) (knowledgeDomain.Document, bool, error)
	ReplaceChunks(ctx context.Context, documentID string, chunks []Chunk) error
	ListChunksByDocument(ctx context.Context, documentID string) ([]Chunk, error)
	ListReadyChunks(ctx context.Context, userID, category string) ([]Chunk, error)
}
