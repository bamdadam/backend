package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/bamdadam/backend/graph/model"
)

var PaginationLimit int32 = 20

type ElementRepository interface {
	GetByURI(ctx context.Context, uri string) (*model.Element, error)
	List(ctx context.Context, params ListParams) (*model.ElementConnection, error)
	UpdateTitle(ctx context.Context, uri string, title string) (*model.Element, error)
}

type elementRepository struct {
	db         *sql.DB
	typeRepo   TypeRepository
	space      SpaceRepository
	user       UserRepository
	fieldValue ElementFieldValueRepository
}

func NewElementRepository(
	db *sql.DB,
	typeRepo TypeRepository,
	space SpaceRepository,
	user UserRepository,
	fieldValue ElementFieldValueRepository,
) ElementRepository {
	return &elementRepository{
		db:         db,
		typeRepo:   typeRepo,
		space:      space,
		user:       user,
		fieldValue: fieldValue,
	}
}

func (r *elementRepository) GetByURI(ctx context.Context, uri string) (*model.Element, error) {
	query := `
		SELECT uri, title, type_uri, space_uri, creation_date, author
		FROM elements WHERE uri = $1
	`

	var elem model.Element
	var typeURI, spaceURI, authorURI string
	var creationDate int64

	err := r.db.QueryRowContext(ctx, query, uri).Scan(
		&elem.URI, &elem.Title, &typeURI, &spaceURI, &creationDate, &authorURI,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get element: %w", err)
	}

	elem.CreationDate = strconv.FormatInt(creationDate, 10)

	if err := r.loadRelations(ctx, &elem, typeURI, spaceURI, authorURI); err != nil {
		return nil, err
	}

	return &elem, nil
}

type ListParams struct {
	Limit            int32
	After            *string
	TypeURI          *string
	SpaceURI         *string
	FieldValueFilter *model.FieldValueFilter
}

func (r *elementRepository) List(ctx context.Context, params ListParams) (*model.ElementConnection, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = PaginationLimit
	}

	query, args, err := r.buildListQuery(params, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to build list query elements: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list elements: %w", err)
	}
	defer rows.Close()

	var elements []*model.Element
	for rows.Next() {
		elem, typeURI, spaceURI, authorURI, err := r.scanElementRow(rows)
		if err != nil {
			return nil, err
		}
		if err := r.loadRelations(ctx, elem, typeURI, spaceURI, authorURI); err != nil {
			return nil, err
		}
		elements = append(elements, elem)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating elements: %w", err)
	}

	hasNextPage := len(elements) > int(limit)
	if hasNextPage {
		elements = elements[:limit]
	}

	return r.buildConnection(elements, hasNextPage), nil
}

func (r *elementRepository) UpdateTitle(ctx context.Context, uri string, title string) (*model.Element, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE elements SET title = $1 WHERE uri = $2`, title, uri)
	if err != nil {
		return nil, fmt.Errorf("failed to update element title: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("element not found: %s", uri)
	}

	return r.GetByURI(ctx, uri)
}

func (r *elementRepository) buildListQuery(params ListParams, limit int32) (string, []interface{}, error) {
	query := `SELECT e.uri, e.title, e.type_uri, e.space_uri, e.creation_date, e.author FROM elements e`
	var conditions []string
	var args []interface{}
	argIdx := 1

	if params.FieldValueFilter != nil {
		col, val, err := r.getFilterColumnAndValue(params.FieldValueFilter)
		if err != nil {
			return "", nil, err
		}
		query += fmt.Sprintf(` JOIN element_field_values efv ON e.uri = efv.element_uri
			AND efv.field_uri = $%d AND efv.%s = $%d`, argIdx, col, argIdx+1)
		args = append(args, params.FieldValueFilter.FieldURI, val)
		argIdx += 2
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
	args = append(args, limit+1)

	return query, args, nil
}

func (r *elementRepository) scanElementRow(rows *sql.Rows) (*model.Element, string, string, string, error) {
	var elem model.Element
	var typeURI, spaceURI, authorURI string
	var creationDate int64

	if err := rows.Scan(&elem.URI, &elem.Title, &typeURI, &spaceURI, &creationDate, &authorURI); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to scan element: %w", err)
	}

	elem.CreationDate = strconv.FormatInt(creationDate, 10)
	return &elem, typeURI, spaceURI, authorURI, nil
}

func (r *elementRepository) loadRelations(ctx context.Context, elem *model.Element, typeURI, spaceURI, authorURI string) error {
	var err error

	elem.Type, err = r.typeRepo.GetByURI(ctx, typeURI)
	if err != nil {
		return fmt.Errorf("failed to get type: %w", err)
	}

	elem.Space, err = r.space.GetByURI(ctx, spaceURI)
	if err != nil {
		return fmt.Errorf("failed to get space: %w", err)
	}

	elem.Author, err = r.user.GetByURI(ctx, authorURI)
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}

	elem.FieldValues, err = r.fieldValue.GetByElementURI(ctx, elem.URI)
	if err != nil {
		return fmt.Errorf("failed to get field values: %w", err)
	}

	return nil
}

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

func (r *elementRepository) getFilterColumnAndValue(filter *model.FieldValueFilter) (string, interface{}, error) {
	switch filter.ValueType {
	case model.FieldValueTypeText:
		return "value_text", filter.Value, nil
	case model.FieldValueTypeNumber:
		num, err := strconv.ParseFloat(filter.Value, 64)
		if err != nil {
			return "", nil, fmt.Errorf("invalid number value: %w", err)
		}
		return "value_number", num, nil
	case model.FieldValueTypeDate:
		date, err := strconv.ParseInt(filter.Value, 10, 64)
		if err != nil {
			return "", nil, fmt.Errorf("invalid date value: %w", err)
		}
		return "value_date", date, nil
	case model.FieldValueTypeBoolean:
		b, err := strconv.ParseBool(filter.Value)
		if err != nil {
			return "", nil, fmt.Errorf("invalid boolean value: %w", err)
		}
		return "value_boolean", b, nil
	case model.FieldValueTypeJSON:
		return "value_json", filter.Value, nil
	default:
		return "", nil, fmt.Errorf("unknown value type: %s", filter.ValueType)
	}
}
