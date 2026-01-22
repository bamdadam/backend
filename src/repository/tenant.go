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

type TenantRepository interface {
	GetByURI(ctx context.Context, uri string) (*model.Tenant, error)
}

type tenantRepository struct {
	db *sql.DB
}

func NewTenantRepository(db *sql.DB) TenantRepository {
	return &tenantRepository{db: db}
}

func (r *tenantRepository) GetByURI(ctx context.Context, uri string) (*model.Tenant, error) {
	query := `SELECT uri, name, status, creation_date FROM tenants WHERE uri = $1`

	var tenant model.Tenant
	var status string
	var creationDate int64

	err := r.db.QueryRowContext(ctx, query, uri).Scan(
		&tenant.URI, &tenant.Name, &status, &creationDate,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("tenant not found: %s", uri)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	tenant.Status = model.TenantStatus(strings.ToUpper(status))
	tenant.CreationDate = strconv.FormatInt(creationDate, 10)

	return &tenant, nil
}
