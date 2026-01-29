package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/bamdadam/backend/graph"
	"github.com/bamdadam/backend/src/middleware"
	"github.com/bamdadam/backend/src/pubsub"
	"github.com/bamdadam/backend/src/repository"
	"github.com/bamdadam/backend/src/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vektah/gqlparser/v2/ast"
)

func Run(ctx context.Context, db *pgxpool.Pool, addr string) error {
	graphqlHandler := newGraphQLHandler(db)
	healthHandler := newHealthHandler(db)

	http.Handle("/", playground.Handler("GraphQL Playground", "/graphql"))
	http.Handle("/graphql", middleware.Auth(graphqlHandler))
	http.Handle("/health", healthHandler)

	log.Printf("GraphQL playground: http://localhost%s/", addr)

	server := &http.Server{
		Addr: addr,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

func newGraphQLHandler(db *pgxpool.Pool) http.Handler {
	userRepo := repository.NewUserRepository(db)
	tenantRepo := repository.NewTenantRepository(db)
	spaceRepo := repository.NewSpaceRepository(db, tenantRepo)
	typeRepo := repository.NewTypeRepository(db, spaceRepo, userRepo)
	fieldRepo := repository.NewFieldRepository(db, typeRepo, userRepo)
	fieldValueRepo := repository.NewElementFieldValueRepository(db, fieldRepo)
	userSpaceRepo := repository.NewUserSpacesRepository(db)
	elementRepo := repository.NewElementRepository(db, typeRepo, spaceRepo, userRepo, fieldValueRepo, userSpaceRepo)

	elementService := service.NewElementService(db, elementRepo)

	resolver := &graph.Resolver{
		ElementService: elementService,
		ElementPubSub:  pubsub.NewElementPubSub(),
	}

	srv := handler.New(graph.NewExecutableSchema(
		graph.Config{Resolvers: resolver}),
	)
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Websocket{})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.ServeHTTP(w, r)
	})
}

func newHealthHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":   "unhealthy",
				"postgres": "disconnected",
				"error":    err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":   "healthy",
			"postgres": "connected",
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
