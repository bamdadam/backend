package service

import (
	"context"
	"fmt"

	"github.com/bamdadam/backend/src/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserService struct {
	db        *pgxpool.Pool
	user      repository.UserRepository
	userSpace repository.UserSpacesRepository
}

func NewUserService(db *pgxpool.Pool, user repository.UserRepository, userSpace repository.UserSpacesRepository) *UserService {
	return &UserService{db: db, user: user, userSpace: userSpace}
}

func (s *UserService) getUserSpaces(ctx context.Context) ([]string, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	us, err := s.userSpace.GetByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user spaces: %w", err)
	}

	if len(us) == 0 {
		return nil, fmt.Errorf("user has no permissions")
	}
	return us, nil
}
