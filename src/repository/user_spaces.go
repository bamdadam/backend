package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserSpacesRepository interface {
	GetByUser(ctx context.Context, uri string) ([]string, error)
}

type userSpacesRepository struct {
	db *pgxpool.Pool
}

func NewUserSpacesRepository(db *pgxpool.Pool) UserSpacesRepository {
	return &userSpacesRepository{db: db}
}

func (r *userSpacesRepository) GetByUser(ctx context.Context, uri string) ([]string, error) {
	query := `SELECT space_uri from user_spaces WHERE user_uri = $1`

	rows, err := r.db.Query(ctx, query, uri)
	if err != nil {
		return nil, fmt.Errorf("failed to get user spaces: %w", err)
	}
	defer rows.Close()
	spaceList, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var space string
		err := rows.Scan(&space)
		return space, err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to collect user spaces: %w", err)
	}
	//for rows.Next() {
	//	var space string
	//	if err := rows.Scan(&space); err != nil {
	//		return nil, fmt.Errorf("failed to scan space :%w", err)
	//	}
	//	spaceList = append(spaceList, space)
	//}

	//if err := rows.Err(); err != nil {
	//	return nil, fmt.Errorf("error iterating elements: %w", err)
	//}
	return spaceList, nil
}
