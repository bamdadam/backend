package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/bamdadam/backend/graph/model"
)

type TypeRepository interface {
	GetByURI(ctx context.Context, uri string) (*model.Type, error)
}

type typeRepository struct {
	db    *sql.DB
	space SpaceRepository
	user  UserRepository
}

func NewTypeRepository(db *sql.DB, space SpaceRepository, user UserRepository) TypeRepository {
	return &typeRepository{db: db, space: space, user: user}
}

func (r *typeRepository) GetByURI(ctx context.Context, uri string) (*model.Type, error) {
	query := `SELECT uri, name, space_uri, creation_date, author FROM types WHERE uri = $1`

	var t model.Type
	var spaceURI, authorURI string
	var creationDate int64

	err := r.db.QueryRowContext(ctx, query, uri).Scan(
		&t.URI, &t.Name, &spaceURI, &creationDate, &authorURI,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("type not found: %s", uri)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get type: %w", err)
	}

	t.CreationDate = strconv.FormatInt(creationDate, 10)

	space, err := r.space.GetByURI(ctx, spaceURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get space for type: %w", err)
	}
	t.Space = space

	author, err := r.user.GetByURI(ctx, authorURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get author for type: %w", err)
	}
	t.Author = author

	return &t, nil
}
