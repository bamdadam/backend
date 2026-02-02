package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/bamdadam/backend/graph/model"
	"github.com/bamdadam/backend/src/middleware"
	"github.com/bamdadam/backend/src/server"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	testServer *httptest.Server
	testDB     *pgxpool.Pool
	testUserID = "user:test-user-1"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/technical_assessment?sslmode=disable"
	}

	var err error
	testDB, err = pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Printf("Failed to connect to test database: %v\n", err)
		os.Exit(1)
	}

	if err := testDB.Ping(ctx); err != nil {
		fmt.Printf("Failed to ping test database: %v\n", err)
		os.Exit(1)
	}

	if err := setupTestData(ctx); err != nil {
		fmt.Printf("Failed to setup test data: %v\n", err)
		os.Exit(1)
	}

	testServer = setupTestServer()

	code := m.Run()

	testServer.Close()
	cleanupTestData(ctx)
	testDB.Close()

	os.Exit(code)
}

func setupTestServer() *httptest.Server {
	graphqlHandler := server.NewGraphQLHandler(testDB)
	mux := http.NewServeMux()
	mux.Handle("/graphql", middleware.Auth(graphqlHandler))

	return httptest.NewServer(mux)
}

func setupTestData(ctx context.Context) error {
	now := time.Now().Unix()

	queries := []string{
		fmt.Sprintf(`INSERT INTO users (uri, email, display_name) VALUES ('%s', 'test@example.com', 'Test User') ON CONFLICT (uri) DO NOTHING`, testUserID),

		fmt.Sprintf(`INSERT INTO tenants (uri, name, status, creation_date) VALUES ('tenant:test-1', 'Test Tenant', 'active', %d) ON CONFLICT (uri) DO NOTHING`, now),

		fmt.Sprintf(`INSERT INTO spaces (uri, name, tenant_uri, creation_date) VALUES ('space:test-1', 'Test Space', 'tenant:test-1', %d) ON CONFLICT (uri) DO NOTHING`, now),
		fmt.Sprintf(`INSERT INTO spaces (uri, name, tenant_uri, creation_date) VALUES ('space:test-2', 'Test Space', 'tenant:test-1', %d) ON CONFLICT (uri) DO NOTHING`, now),

		`INSERT INTO user_spaces (user_uri, space_uri) VALUES ('user:test-user-1', 'space:test-1') ON CONFLICT DO NOTHING`,

		fmt.Sprintf(`INSERT INTO types (uri, name, space_uri, creation_date, author) VALUES ('type:test-1', 'Test Type', 'space:test-1', %d, '%s') ON CONFLICT (uri) DO NOTHING`, now, testUserID),

		fmt.Sprintf(`INSERT INTO fields (uri, name, field_type, type_uri, creation_date, author, options, required) VALUES ('field:test-1', 'Test Text Field', 'text', 'type:test-1', %d, '%s', null, true) ON CONFLICT (uri) DO NOTHING`, now, testUserID),
		fmt.Sprintf(`INSERT INTO fields (uri, name, field_type, type_uri, creation_date, author, options, required) VALUES ('field:test-2', 'Test Select Field', 'select', 'type:test-1', %d, '%s', '["option1","option2"]', false) ON CONFLICT (uri) DO NOTHING`, now, testUserID),
		fmt.Sprintf(`INSERT INTO fields (uri, name, field_type, type_uri, creation_date, author, options, required) VALUES ('field:test-3', 'Test Number Field', 'number', 'type:test-1', %d, '%s', null, true) ON CONFLICT (uri) DO NOTHING`, now, testUserID),

		fmt.Sprintf(`INSERT INTO elements (uri, title, type_uri, space_uri, creation_date, author) VALUES ('element:test-1', 'Test Element 1', 'type:test-1', 'space:test-1', %d, '%s') ON CONFLICT (uri) DO NOTHING`, now, testUserID),
		fmt.Sprintf(`INSERT INTO elements (uri, title, type_uri, space_uri, creation_date, author) VALUES ('element:test-2', 'Test Element 2', 'type:test-1', 'space:test-1', %d, '%s') ON CONFLICT (uri) DO NOTHING`, now, testUserID),
		fmt.Sprintf(`INSERT INTO elements (uri, title, type_uri, space_uri, creation_date, author) VALUES ('element:test-3', 'Test Element 3', 'type:test-1', 'space:test-2', %d, '%s') ON CONFLICT (uri) DO NOTHING`, now, testUserID),
		fmt.Sprintf(`INSERT INTO elements (uri, title, type_uri, space_uri, creation_date, author) VALUES ('element:test-4', 'Test Element 4', 'type:test-1', 'space:test-1', %d, '%s') ON CONFLICT (uri) DO NOTHING`, now, testUserID),

		fmt.Sprintf(`INSERT INTO element_field_values (uri, element_uri, field_uri, value_text, value_number, value_date, value_boolean, value_json, creation_date, updated_date) VALUES ('efv:test-1-1', 'element:test-1', 'field:test-1', 'Hello World', null, null, null, null, %d, %d) ON CONFLICT (uri) DO NOTHING`, now, now),
		fmt.Sprintf(`INSERT INTO element_field_values (uri, element_uri, field_uri, value_text, value_number, value_date, value_boolean, value_json, creation_date, updated_date) VALUES ('efv:test-1-2', 'element:test-1', 'field:test-2', 'option1', null, null, null, null, %d, %d) ON CONFLICT (uri) DO NOTHING`, now, now),
		fmt.Sprintf(`INSERT INTO element_field_values (uri, element_uri, field_uri, value_text, value_number, value_date, value_boolean, value_json, creation_date, updated_date) VALUES ('efv:test-1-3', 'element:test-1', 'field:test-3', null, 42.5, null, null, null, %d, %d) ON CONFLICT (uri) DO NOTHING`, now, now),

		fmt.Sprintf(`INSERT INTO element_field_values (uri, element_uri, field_uri, value_text, value_number, value_date, value_boolean, value_json, creation_date, updated_date) VALUES ('efv:test-2-1', 'element:test-2', 'field:test-1', 'Another text value', null, null, null, null, %d, %d) ON CONFLICT (uri) DO NOTHING`, now, now),
		fmt.Sprintf(`INSERT INTO element_field_values (uri, element_uri, field_uri, value_text, value_number, value_date, value_boolean, value_json, creation_date, updated_date) VALUES ('efv:test-2-3', 'element:test-2', 'field:test-3', null, 100, null, null, null, %d, %d) ON CONFLICT (uri) DO NOTHING`, now, now),

		fmt.Sprintf(`INSERT INTO element_field_values (uri, element_uri, field_uri, value_text, value_number, value_date, value_boolean, value_json, creation_date, updated_date) VALUES ('efv:test-4-2', 'element:test-4', 'field:test-2', 'option1', null, null, null, null, %d, %d) ON CONFLICT (uri) DO NOTHING`, now, now),
		fmt.Sprintf(`INSERT INTO element_field_values (uri, element_uri, field_uri, value_text, value_number, value_date, value_boolean, value_json, creation_date, updated_date) VALUES ('efv:test-4-3', 'element:test-4', 'field:test-3', null, 555, null, null, null, %d, %d) ON CONFLICT (uri) DO NOTHING`, now, now),
	}

	for _, q := range queries {
		if _, err := testDB.Exec(ctx, q); err != nil {
			return fmt.Errorf("failed to execute query %q: %w", q, err)
		}
	}

	return nil
}

func cleanupTestData(ctx context.Context) {
	queries := []string{
		`DELETE FROM element_field_values WHERE uri LIKE 'efv:test-%'`,
		`DELETE FROM elements WHERE uri LIKE 'element:test-%'`,
		`DELETE FROM fields WHERE uri LIKE 'field:test-%'`,
		`DELETE FROM types WHERE uri LIKE 'type:test-%'`,
		`DELETE FROM user_spaces WHERE user_uri = 'user:test-user-1'`,
		`DELETE FROM spaces WHERE uri LIKE 'space:test-%'`,
		`DELETE FROM tenants WHERE uri LIKE 'tenant:test-%'`,
		`DELETE FROM users WHERE uri = 'user:test-user-1'`,
	}

	for _, q := range queries {
		testDB.Exec(ctx, q)
	}
}

func executeGraphQL(t *testing.T, query string, variables map[string]any) graphql.Response {
	t.Helper()

	reqBody := graphql.RawParams{
		Query:     query,
		Variables: variables,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	//c := client.New()
	req, err := http.NewRequest("POST", testServer.URL+"/graphql", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	var gqlResp graphql.Response
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	return gqlResp
}

func TestElementsQuery(t *testing.T) {
	query := `
		query Elements($limit: Int) {
			elements(limit: $limit) {
				edges {
					cursor
					node {
						uri
						title
						type {
							uri
							name
						}
						space {
							uri
							name
						}
						author {
							uri
							displayName
						}
					}
				}
				pageInfo {
					hasNextPage
					startCursor
					endCursor
				}
				totalCount
			}
		}
	`

	resp := executeGraphQL(t, query, map[string]any{"limit": 10})

	if len(resp.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", resp.Errors)
	}

	data := struct {
		model.ElementConnection `json:"elements"`
	}{}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if data.ElementConnection.TotalCount < 1 {
		t.Errorf("Expected at least 1 element, got %d", data.ElementConnection.TotalCount)
	}

	if data.ElementConnection.TotalCount > 3 {
		t.Errorf("Expected at most 3 element, got %d", data.ElementConnection.TotalCount)
	}

	if len(data.ElementConnection.Edges) < 1 {
		t.Errorf("Expected at least 1 edge, got %d", len(data.ElementConnection.Edges))
	}

	if len(data.ElementConnection.Edges) > 0 {
		first := data.ElementConnection.Edges[0].Node
		if first.URI == "" {
			t.Error("Expected element URI to be non-empty")
		}
		if first.Title == "" {
			t.Error("Expected element title to be non-empty")
		}
		if first.Type.URI == "" {
			t.Error("Expected type URI to be non-empty")
		}
		if first.Space.URI == "" {
			t.Error("Expected space URI to be non-empty")
		}
		if first.Author.URI == "" {
			t.Error("Expected author URI to be non-empty")
		}
	}

	for _, node := range data.ElementConnection.Edges {
		elem := node.Node
		if elem.URI == "element:test-3" {
			t.Error("Expected 3rd Element to get filtered out")
		}
	}
}

func TestElementQuery(t *testing.T) {
	query := `
		query Element($uri: ID!) {
			element(uri: $uri) {
				uri
				title
				creationDate
				type {
					uri
					name
					author {
						uri
						email
						displayName
					}
					creationDate
					space {
						uri
						name
						creationDate
						tenant {
							uri
							name
							status
							creationDate
						}
					}
				}
				space {
					uri
					name
					tenant {
							uri
							name
							status
							creationDate
						}
				}
				author {
					uri
					email
					displayName
				}
				fieldValues {
					uri
					value
					field {
						uri
						name
						fieldType
						options
						required	
						author {
							uri
							email
							displayName
						}
						creationDate
						type {
							uri
							name
						}
					}
				}
			}
		}
	`

	resp := executeGraphQL(t, query, map[string]any{"uri": "element:test-1"})

	if len(resp.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", resp.Errors)
	}

	data := struct {
		model.Element `json:"element"`
	}{}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if data.Element.URI != "element:test-1" {
		t.Errorf("Expected URI 'element:test-1', got %q", data.Element.URI)
	}

	if data.Element.Title != "Test Element 1" {
		t.Errorf("Expected title 'Test Element 1', got %q", data.Element.Title)
	}

	if data.Element.Type.URI != "type:test-1" {
		t.Errorf("Expected type URI 'type:test-1', got %q", data.Element.Type.URI)
	}

	if data.Element.Space.URI != "space:test-1" {
		t.Errorf("Expected space URI 'space:test-1', got %q", data.Element.Space.URI)
	}

	if data.Element.Type.Space.URI != data.Element.Space.URI {
		t.Errorf("Expected space URI 'space:test-1', got %q", data.Element.Type.Space.URI)
	}

	if data.Element.Author.URI != testUserID {
		t.Errorf("Expected author URI %q, got %q", testUserID, data.Element.Author.URI)
	}

	if data.Element.Type.Space.Tenant.Status != "ACTIVE" {
		t.Errorf("Expected tenant status 'ACTIVE', got %q", data.Element.Type.Space.Tenant.Status)
	}

	if data.Element.FieldValues == nil {
		t.Errorf("Expected field values, got nil")
	}

	if !slices.ContainsFunc(data.Element.FieldValues, func(efv *model.ElementFieldValue) bool {
		return (efv.URI == "efv:test-1-2") && (efv.Field.URI == "field:test-2") && (efv.Value.(string) == "option1")
	}) {
		efv, err := json.Marshal(data.Element.FieldValues)
		if err != nil {
			t.Fatalf("Failed to marshal data: %v", err)
		}
		t.Errorf("Expected correct element field value, got %s", efv)
	}
}

func TestElementsPagination(t *testing.T) {
	query := `
		query Elements($limit: Int, $after: String) {
			elements(limit: $limit, after: $after) {
				edges {
					cursor
					node {
						uri
						title
					}
				}
				pageInfo {
					hasNextPage
					endCursor
				}
				totalCount
			}
		}
	`

	resp := executeGraphQL(t, query, map[string]any{"limit": 2})

	if len(resp.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", resp.Errors)
	}

	data := struct {
		model.ElementConnection `json:"elements"`
	}{}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}
	if len(data.ElementConnection.Edges) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(data.ElementConnection.Edges))
	}

	if !data.ElementConnection.PageInfo.HasNextPage {
		t.Error("Expected hasNextPage to be true")
	}

	if data.ElementConnection.PageInfo.EndCursor != nil {
		resp2 := executeGraphQL(t, query, map[string]any{
			"limit": 2,
			"after": *data.ElementConnection.PageInfo.EndCursor,
		})

		if len(resp2.Errors) > 0 {
			t.Fatalf("GraphQL errors on second page: %v", resp2.Errors)
		}

		data2 := struct {
			model.ElementConnection `json:"elements"`
		}{}

		if err := json.Unmarshal(resp2.Data, &data2); err != nil {
			t.Fatalf("Failed to unmarshal second page data: %v", err)
		}

		if len(data2.ElementConnection.Edges) < 1 {
			t.Error("Expected at least 1 element on second page")
		}

		for _, edge := range data2.ElementConnection.Edges {
			for _, firstEdge := range data.ElementConnection.Edges {
				if edge.Node.URI == firstEdge.Node.URI {
					t.Errorf("Second page contains element from first page: %s", edge.Node.URI)
				}
			}
		}
	}
}

func TestMissingAuthHeader(t *testing.T) {
	query := `
		query Element($uri: ID!) {
			element(uri:$uri) {
					creationDate
			}
		}
	`

	reqBody := graphql.RawParams{Query: query}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", testServer.URL+"/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}
