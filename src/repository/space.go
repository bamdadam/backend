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

type SpaceRepository interface {
	GetByURI(ctx context.Context, uri string) (*model.Space, error)
}

type spaceRepository struct {
	db     *pgxpool.Pool
	tenant TenantRepository
}

func NewSpaceRepository(db *pgxpool.Pool, tenant TenantRepository) SpaceRepository {
	return &spaceRepository{db: db, tenant: tenant}
}

func (r *spaceRepository) GetByURI(ctx context.Context, uri string) (*model.Space, error) {
	query := `SELECT uri, name, creation_date, tenant_uri FROM spaces WHERE uri = $1`

	var space model.Space
	var tenantURI string
	var creationDate int64

	err := r.db.QueryRow(ctx, query, uri).Scan(
		&space.URI, &space.Name, &creationDate, &tenantURI,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("space not found: %s", uri)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get space: %w", err)
	}

	space.CreationDate = strconv.FormatInt(creationDate, 10)

	tenant, err := r.tenant.GetByURI(ctx, tenantURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant for space: %w", err)
	}
	space.Tenant = tenant

	return &space, nil
}
