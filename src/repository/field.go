package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bamdadam/backend/graph/model"
)

type FieldRepository interface {
	GetByURI(ctx context.Context, uri string) (*model.Field, error)
}

type fieldRepository struct {
	db       *sql.DB
	typeRepo TypeRepository
	user     UserRepository
}

func NewFieldRepository(db *sql.DB, typeRepo TypeRepository, user UserRepository) FieldRepository {
	return &fieldRepository{db: db, typeRepo: typeRepo, user: user}
}

func (r *fieldRepository) GetByURI(ctx context.Context, uri string) (*model.Field, error) {
	query := `SELECT uri, name, field_type, type_uri, creation_date, author, options, required FROM fields WHERE uri = $1`

	var field model.Field
	var fieldTypeStr, typeURI, authorURI string
	var creationDate int64
	var options sql.NullString

	err := r.db.QueryRowContext(ctx, query, uri).Scan(
		&field.URI, &field.Name, &fieldTypeStr, &typeURI, &creationDate, &authorURI, &options, &field.Required,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("field not found: %s", uri)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get field: %w", err)
	}

	field.FieldType = model.FieldType(strings.ToUpper(fieldTypeStr))
	field.CreationDate = strconv.FormatInt(creationDate, 10)
	if options.Valid {
		field.Options = &options.String
	}

	fieldType, err := r.typeRepo.GetByURI(ctx, typeURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get type for field: %w", err)
	}
	field.Type = fieldType

	author, err := r.user.GetByURI(ctx, authorURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get author for field: %w", err)
	}
	field.Author = author

	return &field, nil
}
