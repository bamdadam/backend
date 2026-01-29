package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/bamdadam/backend/graph/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TypeRepository interface {
	GetByURI(ctx context.Context, uri string) (*model.Type, error)
}

type typeRepository struct {
	db    *pgxpool.Pool
	space SpaceRepository
	user  UserRepository
}

func NewTypeRepository(db *pgxpool.Pool, space SpaceRepository, user UserRepository) TypeRepository {
	return &typeRepository{db: db, space: space, user: user}
}

func (r *typeRepository) GetByURI(ctx context.Context, uri string) (*model.Type, error) {
	query := `SELECT uri, name, space_uri, creation_date, author FROM types WHERE uri = $1`

	var t model.Type
	var spaceURI, authorURI string
	var creationDate int64

	err := r.db.QueryRow(ctx, query, uri).Scan(
		&t.URI, &t.Name, &spaceURI, &creationDate, &authorURI,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("type not found: %s", uri)
		}
		return nil, fmt.Errorf("failed to get type: %w", err)
	}

	t.CreationDate = strconv.FormatInt(creationDate, 10)

	t.Space, err = r.space.GetByURI(ctx, spaceURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get space for type: %w", err)
	}

	t.Author, err = r.user.GetByURI(ctx, authorURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get author for type: %w", err)
	}

	return &t, nil
}
