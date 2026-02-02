package service

import (
	"context"
	"fmt"

	"github.com/bamdadam/backend/graph/model"
	models "github.com/bamdadam/backend/src/model"
	"github.com/bamdadam/backend/src/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

var PaginationLimit int32 = 20

type ElementService struct {
	db          *pgxpool.Pool
	elementRepo repository.ElementRepository
}

func NewElementService(db *pgxpool.Pool, elementRepo repository.ElementRepository) *ElementService {
	return &ElementService{db: db, elementRepo: elementRepo}
}

func (s *ElementService) GetByURI(ctx context.Context, uri string) (*model.Element, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	elem, err := s.elementRepo.GetByURI(ctx, uri, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get element by uri: %w", err)
	}
	return elem, err
}

func (s *ElementService) List(ctx context.Context, params models.ListParams) (*model.ElementConnection, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err = s.validateFieldValueFilter(params.FieldValueFilter); err != nil {
		return nil, fmt.Errorf("failed to validate filed value filter: %w", err)
	}

	limit := params.Limit
	if limit <= 0 {
		limit = PaginationLimit
	}

	elems, err := s.elementRepo.List(ctx, params, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list elements: %w", err)
	}
	return elems, nil
}

func (s *ElementService) UpdateTitle(ctx context.Context, uri, title string) (*model.Element, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	elem, err := s.elementRepo.UpdateTitle(ctx, uri, title, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to update element: %w", err)
	}
	return elem, nil
}

// validateFieldValueFilter validates FieldValueFilter value and valueType fields
// by checking if they are both present or not present at the same time and if only
// one of them is present, returns an error
func (s *ElementService) validateFieldValueFilter(filter *model.FieldValueFilter) error {
	if filter != nil &&
		((filter.Value != nil && filter.ValueType == nil) ||
			(filter.Value == nil && filter.ValueType != nil)) {
		return fmt.Errorf("value and value type not present together")
	}
	return nil
}
