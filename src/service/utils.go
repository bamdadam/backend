package service

import (
	"context"
	"fmt"

	"github.com/bamdadam/backend/src/model"
)

func getUserID(ctx context.Context) (string, error) {
	if userID, ok := ctx.Value(model.UserIDKey).(string); ok {
		return userID, nil
	}
	return "", fmt.Errorf("user not found")
}
