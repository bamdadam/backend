package repository

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/bamdadam/backend/graph/model"
	models "github.com/bamdadam/backend/src/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type loadRelationParams struct {
	typeURI,
	spaceURI,
	authorURI string
}

type ElementRepository interface {
	GetByURI(ctx context.Context, uri, userID string) (*model.Element, error)
	List(ctx context.Context, params models.ListParams, userID string) (*model.ElementConnection, error)
	UpdateTitle(ctx context.Context, uri, title, userID string) (*model.Element, error)
}

type elementRepository struct {
	db         *pgxpool.Pool
	typeRepo   TypeRepository
	space      SpaceRepository
	user       UserRepository
	fieldValue ElementFieldValueRepository
	userSpace  UserSpacesRepository
}

func NewElementRepository(
	db *pgxpool.Pool,
	typeRepo TypeRepository,
	space SpaceRepository,
	user UserRepository,
	fieldValue ElementFieldValueRepository,
	userSpace UserSpacesRepository,
) ElementRepository {
	return &elementRepository{
		db:         db,
		typeRepo:   typeRepo,
		space:      space,
		user:       user,
		fieldValue: fieldValue,
		userSpace:  userSpace,
	}
}

// GetByURI retrieves a single element by its URI, if the user has access to the element's space.
// Returns nil if the user has no accessible spaces. Returns an error if the element is not found
// or if the user lacks permission to access the element's space.
func (r *elementRepository) GetByURI(ctx context.Context, uri, userID string) (*model.Element, error) {
	userSpaces, err := r.userSpace.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(userSpaces) == 0 {
		return nil, nil
	}
	// todo handle elements was not found regardless that user had permission to see the space logic
	query := `
		SELECT uri, title, type_uri, space_uri, creation_date, author
		FROM elements WHERE uri = $1 AND space_uri = ANY($2)
	`

	var elem model.Element
	var typeURI, spaceURI, authorURI string
	var creationDate int64

	err = r.db.QueryRow(ctx, query, uri, userSpaces).Scan(
		&elem.URI, &elem.Title, &typeURI, &spaceURI, &creationDate, &authorURI,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get element: %w", err)
	}

	elem.CreationDate = strconv.FormatInt(creationDate, 10)
	// todo handle loading relations in service layer and decouple repositories from each other
	if err = r.loadRelations(ctx, &elem,
		loadRelationParams{
			typeURI:   typeURI,
			authorURI: authorURI,
			spaceURI:  spaceURI,
		}); err != nil {
		return nil, err
	}

	return &elem, nil
}

// List retrieves a paginated list of elements the user has access to based on their space permissions.
// Supports filtering by type URI, space URI, and field values. Returns nil if the user has no accessible spaces.
// If a space filter is provided but the user lacks access, the filter is ignored.
func (r *elementRepository) List(ctx context.Context, params models.ListParams, userID string) (*model.ElementConnection, error) {
	userSpaces, err := r.userSpace.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(userSpaces) == 0 {
		return nil, nil
	}
	// todo handle multiple space or type filter
	if params.SpaceURI != nil {
		if !slices.Contains(userSpaces, *params.SpaceURI) {
			params.SpaceURI = nil
		}
	}

	query, args, err := r.buildListQuery(params)
	if err != nil {
		return nil, fmt.Errorf("failed to build list query elements: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list elements: %w", err)
	}

	var uri, title, typeURI, spaceURI, authorURI string
	var creationDate int64
	var elements []*model.Element

	_, err = pgx.ForEachRow(rows,
		[]any{
			&uri,
			&title,
			&typeURI,
			&spaceURI,
			&creationDate,
			&authorURI},
		func() error {
			var elem model.Element
			elem.CreationDate = strconv.FormatInt(creationDate, 10)
			elem.URI = uri
			elem.Title = title
			// todo handle loading relations in service layer and decouple repositories from each other
			if err = r.loadRelations(ctx, &elem,
				loadRelationParams{
					typeURI:   typeURI,
					authorURI: authorURI,
					spaceURI:  spaceURI,
				}); err != nil {
				return err
			}

			elements = append(elements, &elem)
			return nil
		})

	if err != nil {
		return nil, fmt.Errorf("error iterating elements: %w", err)
	}

	hasNextPage := len(elements) > int(params.Limit)
	if hasNextPage {
		elements = elements[:params.Limit]
	}

	return r.buildConnection(elements, hasNextPage), nil
}

func (r *elementRepository) UpdateTitle(ctx context.Context, uri string, title string, userID string) (*model.Element, error) {
	result, err := r.db.Exec(ctx, `UPDATE elements SET title = $1 WHERE uri = $2`, title, uri)
	if err != nil {
		return nil, fmt.Errorf("failed to update element title: %w", err)
	}

	if result.RowsAffected() == 0 {
		return nil, fmt.Errorf("element not found: %s", uri)
	}

	return r.GetByURI(ctx, uri, userID)
}

// buildListQuery constructs a dynamic SQL query for listing elements based on the provided filter parameters.
// Supports filtering by type URI, space URI, field values, and cursor-based pagination.
// Returns the query string, positional arguments, and any error encountered during query construction.
func (r *elementRepository) buildListQuery(params models.ListParams) (string, []interface{}, error) {
	query := `SELECT e.uri, e.title, e.type_uri, e.space_uri, e.creation_date, e.author FROM elements e`
	var conditions []string
	var args []interface{}
	argIdx := 1

	if params.FieldValueFilter != nil {
		query += fmt.Sprintf(` JOIN element_field_values efv ON e.uri = efv.element_uri`)
		if params.FieldValueFilter.FieldURI != nil {
			query += fmt.Sprintf(` AND efv.field_uri = $%d`, argIdx)
			args = append(args, params.FieldValueFilter.FieldURI)
			argIdx++
		}
		if params.FieldValueFilter.Value != nil && params.FieldValueFilter.ValueType != nil {
			col, val, err := r.getFilterColumnAndValue(params.FieldValueFilter)
			if err != nil {
				return "", nil, err
			}
			query += fmt.Sprintf(` AND efv.%s = $%d`, col, argIdx)
			args = append(args, val)
			argIdx++
		}
	}

	if params.TypeURI != nil {
		conditions = append(conditions, fmt.Sprintf("e.type_uri = $%d", argIdx))
		args = append(args, *params.TypeURI)
		argIdx++
	}

	if params.SpaceURI != nil {
		conditions = append(conditions, fmt.Sprintf("e.space_uri = $%d", argIdx))
		args = append(args, *params.SpaceURI)
		argIdx++
	}

	if params.After != nil {
		conditions = append(conditions, fmt.Sprintf("e.uri > $%d", argIdx))
		args = append(args, *params.After)
		argIdx++
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += cond
		}
	}

	query += fmt.Sprintf(" ORDER BY e.uri ASC LIMIT $%d", argIdx)
	args = append(args, params.Limit+1)

	return query, args, nil
}

func (r *elementRepository) loadRelations(ctx context.Context, elem *model.Element, params loadRelationParams) error {
	var err error

	elem.Type, err = r.typeRepo.GetByURI(ctx, params.typeURI)
	if err != nil {
		return fmt.Errorf("failed to get type: %w", err)
	}

	elem.Space, err = r.space.GetByURI(ctx, params.spaceURI)
	if err != nil {
		return fmt.Errorf("failed to get space: %w", err)
	}

	elem.Author, err = r.user.GetByURI(ctx, params.authorURI)
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}

	elem.FieldValues, err = r.fieldValue.GetByElementURI(ctx, elem.URI)
	if err != nil {
		return fmt.Errorf("failed to get field values: %w", err)
	}

	return nil
}

// buildConnection transforms a slice of elements into a GraphQL-compliant connection structure
// with edges, cursors, and pagination info. Each element's URI is used as its cursor.
func (r *elementRepository) buildConnection(elements []*model.Element, hasNextPage bool) *model.ElementConnection {
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

// getFilterColumnAndValue maps a FieldValueFilter to the appropriate database column name and
// parsed value based on the filter's value type. Handles TEXT, NUMBER, DATE, BOOLEAN, and JSON types.
// Returns the column name, converted value, and any parsing error.
func (r *elementRepository) getFilterColumnAndValue(filter *model.FieldValueFilter) (string, interface{}, error) {
	switch *filter.ValueType {
	case model.FieldValueTypeText:
		return "value_text", *filter.Value, nil
	case model.FieldValueTypeNumber:
		num, err := strconv.ParseFloat(*filter.Value, 64)
		if err != nil {
			return "", nil, fmt.Errorf("invalid number value: %w", err)
		}
		return "value_number", num, nil
	case model.FieldValueTypeDate:
		date, err := strconv.ParseInt(*filter.Value, 10, 64)
		if err != nil {
			return "", nil, fmt.Errorf("invalid date value: %w", err)
		}
		return "value_date", date, nil
	case model.FieldValueTypeBoolean:
		b, err := strconv.ParseBool(*filter.Value)
		if err != nil {
			return "", nil, fmt.Errorf("invalid boolean value: %w", err)
		}
		return "value_boolean", b, nil
	case model.FieldValueTypeJSON:
		return "value_json", *filter.Value, nil
	default:
		return "", nil, fmt.Errorf("unknown value type: %s", *filter.ValueType)
	}
}
