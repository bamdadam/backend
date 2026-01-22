package graph

import (
	"github.com/bamdadam/backend/src/pubsub"
	"github.com/bamdadam/backend/src/repository"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	ElementRepo   repository.ElementRepository
	ElementPubSub *pubsub.ElementPubSub
}