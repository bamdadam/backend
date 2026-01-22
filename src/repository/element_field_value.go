package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/bamdadam/backend/graph/model"
)

type ElementFieldValueRepository interface {
	GetByElementURI(ctx context.Context, elementURI string) ([]*model.ElementFieldValue, error)
}

type elementFieldValueRepository struct {
	db    *sql.DB
	field FieldRepository
}

func NewElementFieldValueRepository(db *sql.DB, field FieldRepository) ElementFieldValueRepository {
	return &elementFieldValueRepository{db: db, field: field}
}

func (r *elementFieldValueRepository) GetByElementURI(ctx context.Context, elementURI string) ([]*model.ElementFieldValue, error) {
	query := `
		SELECT uri, field_uri, value_text, value_number, value_date, value_boolean, value_json
		FROM element_field_values
		WHERE element_uri = $1
	`

	rows, err := r.db.QueryContext(ctx, query, elementURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get element field values: %w", err)
	}
	defer rows.Close()

	var fieldValues []*model.ElementFieldValue
	for rows.Next() {
		var fv model.ElementFieldValue
		var fieldURI string
		var valueText, valueJSON sql.NullString
		var valueNumber sql.NullFloat64
		var valueDate sql.NullInt64
		var valueBool sql.NullBool

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

func (r *elementFieldValueRepository) extractValue(
	valueText sql.NullString,
	valueNumber sql.NullFloat64,
	valueDate sql.NullInt64,
	valueBool sql.NullBool,
	valueJSON sql.NullString,
) interface{} {
	if valueText.Valid {
		return valueText.String
	}
	if valueNumber.Valid {
		return valueNumber.Float64
	}
	if valueDate.Valid {
		return valueDate.Int64
	}
	if valueBool.Valid {
		return valueBool.Bool
	}
	if valueJSON.Valid {
		var jsonValue interface{}
		if err := json.Unmarshal([]byte(valueJSON.String), &jsonValue); err == nil {
			return jsonValue
		}
		return valueJSON.String
	}
	return nil
}
