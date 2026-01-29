package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bamdadam/backend/graph/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FieldRepository interface {
	GetByURI(ctx context.Context, uri string) (*model.Field, error)
}

type fieldRepository struct {
	db       *pgxpool.Pool
	typeRepo TypeRepository
	user     UserRepository
}

func NewFieldRepository(db *pgxpool.Pool, typeRepo TypeRepository, user UserRepository) FieldRepository {
	return &fieldRepository{db: db, typeRepo: typeRepo, user: user}
}

func (r *fieldRepository) GetByURI(ctx context.Context, uri string) (*model.Field, error) {
	query := `SELECT uri, name, field_type, type_uri, creation_date, author, options, required FROM fields WHERE uri = $1`

	var field model.Field
	var fieldTypeStr, typeURI, authorURI string
	var creationDate int64
	var options *string

	err := r.db.QueryRow(ctx, query, uri).Scan(
		&field.URI, &field.Name, &fieldTypeStr, &typeURI, &creationDate, &authorURI, &options, &field.Required,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("field not found: %s", uri)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get field: %w", err)
	}

	field.FieldType = model.FieldType(strings.ToUpper(fieldTypeStr))
	field.CreationDate = strconv.FormatInt(creationDate, 10)
	field.Options = options

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
