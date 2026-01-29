package model

import "github.com/bamdadam/backend/graph/model"

type contextKey string

const UserIDKey contextKey = "userID"

type ListParams struct {
	Limit            int32
	After            *string
	TypeURI          *string
	SpaceURI         *string
	FieldValueFilter *model.FieldValueFilter
}
