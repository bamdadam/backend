package service

import (
	"context"
	"fmt"

	"github.com/bamdadam/backend/graph/model"
	models "github.com/bamdadam/backend/src/model"
	"github.com/bamdadam/backend/src/pubsub"
	"github.com/bamdadam/backend/src/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

var PaginationLimit int32 = 20

type ElementService struct {
	db *pgxpool.Pool

	*UserService

	elementRepo repository.ElementRepository
	typeRepo    repository.TypeRepository
	space       repository.SpaceRepository
	fieldValue  repository.ElementFieldValueRepository
	pubsub      *pubsub.ElementPubSub
}

func NewElementService(db *pgxpool.Pool, us *UserService, elementRepo repository.ElementRepository,
	typeRepo repository.TypeRepository, spaceRepo repository.SpaceRepository,
	fieldValueRepo repository.ElementFieldValueRepository, pubsub *pubsub.ElementPubSub) *ElementService {
	return &ElementService{
		db:          db,
		UserService: us,
		elementRepo: elementRepo,
		typeRepo:    typeRepo,
		space:       spaceRepo,
		fieldValue:  fieldValueRepo,
		pubsub:      pubsub,
	}
}

func (s *ElementService) GetByURI(ctx context.Context, uri string) (*model.Element, error) {
	userSpaces, err := s.getUserSpaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get element by uri: %w", err)
	}

	elem, params, err := s.elementRepo.GetByURI(ctx, uri, userSpaces)
	if err != nil {
		return nil, fmt.Errorf("failed to get element by uri: %w", err)
	}

	err = s.loadRelations(ctx, elem, params)
	if err != nil {
		return nil, fmt.Errorf("failed to load element relations: %w", err)
	}
	return elem, err
}

func (s *ElementService) List(ctx context.Context, params models.ListParams) (*model.ElementConnection, error) {
	userSpaces, err := s.getUserSpaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list elements by uri: %w", err)
	}

	if err = s.validateFieldValueFilter(params.FieldValueFilter); err != nil {
		return nil, fmt.Errorf("failed to validate filed value filter: %w", err)
	}

	if params.Limit <= 0 {
		params.Limit = PaginationLimit
	}

	elemsWithRelations, err := s.elementRepo.List(ctx, params, userSpaces)
	if err != nil {
		return nil, fmt.Errorf("failed to list elements: %w", err)
	}

	var elements []*model.Element
	for _, elemWithRel := range elemsWithRelations {
		if err = s.loadRelations(ctx, elemWithRel.Element, &elemWithRel.LoadRelationParams); err != nil {
			return nil, fmt.Errorf("failed to load element relations: %w", err)
		}
		elements = append(elements, elemWithRel.Element)
	}

	hasNextPage := len(elements) > int(params.Limit)
	if hasNextPage {
		elements = elements[:params.Limit]
	}

	return s.buildConnection(elements, hasNextPage), nil
}

func (s *ElementService) UpdateTitle(ctx context.Context, uri, title string) (*model.Element, error) {
	userSpaces, err := s.getUserSpaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update element: %w", err)
	}

	elem, params, err := s.elementRepo.UpdateTitle(ctx, uri, title, userSpaces)
	if err != nil {
		return nil, fmt.Errorf("failed to update element: %w", err)
	}

	err = s.loadRelations(ctx, elem, params)
	if err != nil {
		return nil, fmt.Errorf("failed to load element relations: %w", err)
	}

	s.pubsub.Publish(elem)

	return elem, nil
}

func (s *ElementService) UpdateElementSubscribe(ctx context.Context, uri string) (<-chan *model.Element, error) {
	userSpaces, err := s.getUserSpaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to element by uri: %w", err)
	}

	_, _, err = s.elementRepo.GetByURI(ctx, uri, userSpaces)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to element by uri: %w", err)
	}

	ch := s.pubsub.Subscribe(uri)

	go func() {
		<-ctx.Done()
		s.pubsub.Unsubscribe(uri, ch)
	}()

	return ch, nil
}

// buildConnection transforms a slice of elements into a GraphQL-compliant connection structure
// with edges, cursors, and pagination info. Each element's URI is used as its cursor.
func (s *ElementService) buildConnection(elements []*model.Element, hasNextPage bool) *model.ElementConnection {
	edges := make([]*model.ElementEdge, len(elements))
	for i, elem := range elements {
		edges[i] = &model.ElementEdge{
			Cursor: elem.URI,
			Node:   elem,
		}
	}

	pageInfo := &model.PageInfo{
		HasNextPage: hasNextPage,
	}

	if len(edges) > 0 {
		pageInfo.StartCursor = &edges[0].Cursor
		pageInfo.EndCursor = &edges[len(edges)-1].Cursor
	}

	return &model.ElementConnection{
		Edges:      edges,
		PageInfo:   pageInfo,
		TotalCount: int32(len(elements)),
	}
}

func (s *ElementService) loadRelations(ctx context.Context, elem *model.Element, params *models.LoadRelationParams) error {
	var err error

	elem.Type, err = s.typeRepo.GetByURI(ctx, params.TypeURI)
	if err != nil {
		return fmt.Errorf("failed to get type: %w", err)
	}

	elem.Space, err = s.space.GetByURI(ctx, params.SpaceURI)
	if err != nil {
		return fmt.Errorf("failed to get space: %w", err)
	}

	elem.Author, err = s.user.GetByURI(ctx, params.AuthorURI)
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}

	elem.FieldValues, err = s.fieldValue.GetByElementURI(ctx, elem.URI)
	if err != nil {
		return fmt.Errorf("failed to get field values: %w", err)
	}

	return nil
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
