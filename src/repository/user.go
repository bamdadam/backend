package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bamdadam/backend/graph/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository interface {
	GetByURI(ctx context.Context, uri string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type userRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetByURI(ctx context.Context, uri string) (*model.User, error) {
	query := `SELECT uri, email, display_name FROM users WHERE uri = $1`

	var user model.User
	err := r.db.QueryRow(ctx, query, uri).Scan(
		&user.URI, &user.Email, &user.DisplayName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("user not found: %s", uri)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `SELECT uri, email, display_name FROM users WHERE email = $1`

	var user model.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.URI, &user.Email, &user.DisplayName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("user not found with email: %s", email)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}
