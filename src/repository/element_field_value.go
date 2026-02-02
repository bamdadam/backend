package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bamdadam/backend/graph/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ElementFieldValueRepository interface {
	GetByElementURI(ctx context.Context, elementURI string) ([]*model.ElementFieldValue, error)
}

type elementFieldValueRepository struct {
	db    *pgxpool.Pool
	field FieldRepository
}

func NewElementFieldValueRepository(db *pgxpool.Pool, field FieldRepository) ElementFieldValueRepository {
	return &elementFieldValueRepository{db: db, field: field}
}

func (r *elementFieldValueRepository) GetByElementURI(ctx context.Context, elementURI string) ([]*model.ElementFieldValue, error) {
	query := `
		SELECT uri, field_uri, value_text, value_number, value_date, value_boolean, value_json
		FROM element_field_values
		WHERE element_uri = $1
	`

	rows, err := r.db.Query(ctx, query, elementURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get element field values: %w", err)
	}
	defer rows.Close()

	var fieldValues []*model.ElementFieldValue
	for rows.Next() {
		var fv model.ElementFieldValue
		var fieldURI string
		var valueText, valueJSON *string
		var valueNumber *float64
		var valueDate *int64
		var valueBool *bool

		if err := rows.Scan(
			&fv.URI, &fieldURI, &valueText, &valueNumber, &valueDate, &valueBool, &valueJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan element field value: %w", err)
		}

		field, err := r.field.GetByURI(ctx, fieldURI)
		if err != nil {
			return nil, fmt.Errorf("failed to get field for element field value: %w", err)
		}
		fv.Field = field

		fv.Value = r.extractValue(valueText, valueNumber, valueDate, valueBool, valueJSON)
		if fv.Value == nil {
			return nil, fmt.Errorf("field value is nil, possible data corruption for element: %s, field: %s", elementURI, fv.Field.URI)
		}
		fieldValues = append(fieldValues, &fv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating element field values: %w", err)
	}

	return fieldValues, nil
}

// extractValue checks which of the value fields in the database has a value
// and extracts that value, this function assumes the data at the database level
// is always correct and only one value type is present.
func (r *elementFieldValueRepository) extractValue(
	valueText *string,
	valueNumber *float64,
	valueDate *int64,
	valueBool *bool,
	valueJSON *string,
) interface{} {
	if valueText != nil {
		return *valueText
	}
	if valueNumber != nil {
		return *valueNumber
	}
	if valueDate != nil {
		return *valueDate
	}
	if valueBool != nil {
		return *valueBool
	}
	if valueJSON != nil {
		var jsonValue interface{}
		if err := json.Unmarshal([]byte(*valueJSON), &jsonValue); err == nil {
			return jsonValue
		}
		return *valueJSON
	}
	return nil
}
