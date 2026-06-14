package food

import "context"

type Filter struct {
	Labels           []string
	ExcludeAllergens []string
	Search           *string
}

type Repository interface {
	List(ctx context.Context, filter Filter) ([]Food, error)
	GetByID(ctx context.Context, id string) (Food, error)
	GetByIDs(ctx context.Context, ids []string) ([]Food, error)
	Create(ctx context.Context, f Food) (Food, error)
	Update(ctx context.Context, f Food) (Food, error)
	Delete(ctx context.Context, id string) error
}
