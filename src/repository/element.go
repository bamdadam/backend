package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bamdadam/backend/graph/model"
	models "github.com/bamdadam/backend/src/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ElementRepository interface {
	GetByURI(ctx context.Context, uri string, userSpaces []string) (*model.Element, *models.LoadRelationParams, error)
	List(ctx context.Context, params models.ListParams, userSpaces []string) ([]*models.ElemWithRelation, error)
	UpdateTitle(ctx context.Context, uri, title string, userSpaces []string) (*model.Element, *models.LoadRelationParams, error)
}

type elementRepository struct {
	db *pgxpool.Pool
}

func NewElementRepository(db *pgxpool.Pool) ElementRepository {
	return &elementRepository{db: db}
}

// GetByURI retrieves a single element by its URI, filtered by the user's accessible spaces.
// Returns an error if the element is not found or not in an accessible space.
func (r *elementRepository) GetByURI(ctx context.Context, uri string, userSpaces []string) (*model.Element, *models.LoadRelationParams, error) {
	query := `
		SELECT uri, title, type_uri, space_uri, creation_date, author
		FROM elements WHERE uri = $1 AND space_uri = ANY($2)
	`

	var elem model.Element
	var typeURI, spaceURI, authorURI string
	var creationDate int64

	err := r.db.QueryRow(ctx, query, uri, userSpaces).Scan(
		&elem.URI, &elem.Title, &typeURI, &spaceURI, &creationDate, &authorURI,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get element: %w", err)
	}

	elem.CreationDate = strconv.FormatInt(creationDate, 10)

	relationParams := models.LoadRelationParams{
		TypeURI:   typeURI,
		AuthorURI: authorURI,
		SpaceURI:  spaceURI,
	}

	return &elem, &relationParams, nil
}

// List retrieves a paginated list of elements filtered by the user's accessible spaces.
// Supports filtering by type URI, space URI, and field values.
func (r *elementRepository) List(ctx context.Context, params models.ListParams, userSpaces []string) ([]*models.ElemWithRelation, error) {
	query, args, err := r.buildListQuery(params, userSpaces)
	if err != nil {
		return nil, fmt.Errorf("failed to build list query elements: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list elements: %w", err)
	}

	var uri, title, typeURI, spaceURI, authorURI string
	var creationDate int64
	var elements []*models.ElemWithRelation

	_, err = pgx.ForEachRow(rows,
		[]any{
			&uri,
			&title,
			&typeURI,
			&spaceURI,
			&creationDate,
			&authorURI},
		func() error {
			elem := &models.ElemWithRelation{
				Element: &model.Element{
					URI:          uri,
					Title:        title,
					CreationDate: strconv.FormatInt(creationDate, 10),
				},
				LoadRelationParams: models.LoadRelationParams{
					TypeURI:   typeURI,
					AuthorURI: authorURI,
					SpaceURI:  spaceURI,
				},
			}
			elements = append(elements, elem)
			return nil
		})

	if err != nil {
		return nil, fmt.Errorf("error iterating elements: %w", err)
	}

	return elements, nil
}

func (r *elementRepository) UpdateTitle(ctx context.Context, uri, title string, userSpaces []string) (*model.Element, *models.LoadRelationParams, error) {
	result, err := r.db.Exec(ctx, `UPDATE elements SET title = $1 WHERE uri = $2 AND space_uri = ANY($3)`, title, uri, userSpaces)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update element title: %w", err)
	}

	if result.RowsAffected() == 0 {
		return nil, nil, fmt.Errorf("element not found: %s", uri)
	}

	return r.GetByURI(ctx, uri, userSpaces)
}

// buildListQuery constructs a dynamic SQL query for listing elements based on the provided filter parameters.
// Supports filtering by type URI, space URI, field values, user spaces, and cursor-based pagination.
// Returns the query string, positional arguments, and any error encountered during query construction.
func (r *elementRepository) buildListQuery(params models.ListParams, userSpaces []string) (string, []interface{}, error) {
	query := `SELECT e.uri, e.title, e.type_uri, e.space_uri, e.creation_date, e.author FROM elements e`
	var conditions []string
	var args []interface{}
	argIdx := 1

	if params.FieldValueFilter != nil {
		query += ` JOIN element_field_values efv ON e.uri = efv.element_uri`
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

	// Always filter by user's accessible spaces
	conditions = append(conditions, fmt.Sprintf("e.space_uri = ANY($%d)", argIdx))
	args = append(args, userSpaces)
	argIdx++

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
