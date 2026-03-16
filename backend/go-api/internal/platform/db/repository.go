package db

import "context"

type Repository[T any] interface {
	Create(ctx context.Context, entity *T) error
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*T, error)
}
