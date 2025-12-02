# Set up

## 1. Create the PostgreSQL instance

Make sure you have docker installed, and run:

```bash
docker compose up postgres
```

This will start a PostgreSQL 16 container and automatically:
- Create the database `technical_assessment`
- Run all SQL schema files to create the tables
- Populate the database with sample data (3,700+ elements)

After creation, verify that it is running with:

```bash
docker compose exec postgres psql -U postgres -d technical_assessment -c "SELECT COUNT(*) FROM elements;"
```

You should see a count of 3700 elements.

## 2. Build the Go tools

Make sure you have Go installed (1.24 or later recommended), then build the project:

```bash
go build ./...
```

This will compile all packages and verify everything is set up correctly.

## 3. Run the API

Start the API server:

```bash
go run ./cmd/api
```

The server will start on `http://localhost:8080`. You can verify it's running:

```bash
curl http://localhost:8080/health
```

You should see:

```json
{"postgres":"connected","status":"healthy"}
```

## Project Structure

```
.
├── cmd/
│   └── api/                   # API server 
├── src/
|   └── server/
|       └── server.go          # Server Handler (build your solution from here)
├── sql/
│   ├── 01_users.sql           # User accounts
│   ├── 02_tenants.sql         # Multi-tenant organizations
│   ├── 03_permission_verbs.sql # Permission actions
│   ├── 04_spaces.sql          # Hierarchical containers
│   ├── 05_types.sql           # Content type definitions
│   ├── 06_elements.sql        # Content items/documents
│   ├── 07_user_tenants.sql    # User-tenant membership
│   ├── 08_user_spaces.sql     # User-space access
│   ├── 09_user_space_permissions.sql # Fine-grained permissions
│   ├── 10_fields.sql          # Field definitions
│   ├── 11_element_field_values.sql # Field values for elements
│   └── 99_sample_data.sql     # Sample data generation
├── docker-compose.yml         # PostgreSQL container config
├── go.mod                     # Go module definition
├── go.sum                     # Go dependencies checksum



```

## Database Schema

### Structure

The database implements a flexible, structure where:

- **Types** define the schema for different kinds of content (e.g., Project, Task, Employee)
- **Fields** define the columns for each type with specific data types:
  - `text` - Text values
  - `number` - Numeric values
  - `date` - Date/timestamp values
  - `boolean` - True/false values
  - `select` - Single selection from options
  - `multi_select` - Multiple selections from options
  - `url` - URL values
  - `email` - Email addresses
- **Elements** are individual records/rows
- **Element Field Values** store the actual values for each element's fields

### Sample Data

The sample data includes:

| Type | Count | Description |
|------|-------|-------------|
| Projects | 500 | With name, status, budget, dates |
| Tasks | 1,000 | With title, priority, status, estimate |
| Employees | 300 | With name, email, salary, department |
| Products | 800 | With name, SKU, price, stock, category |
| Customers | 400 | With company, email, revenue, tier |
| Support Tickets | 600 | With title, description, priority, status |
| Inventory Items | 500 | With name, SKU, quantity, cost, location |
| Suppliers | 100 | With name, email, rating |
| **Total** | **3,700** | Elements with ~18,500 field values |

### Multi-tenant Architecture

- **Tenants**: Top-level organization units (Acme, Globex, Initech)
- **Spaces**: Containers within tenants (Projects, HR, Products, etc.)
- **Types/Elements**: Content within spaces
- **Permissions**: Fine-grained access control through user-space-permission associations

## Environment Variables

The API server supports the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/technical_assessment?sslmode=disable` | PostgreSQL connection string |

# Backend Technical Test - Golang & GraphQL
# Elements GraphQL API (Golang)

## Objective

Build a small backend service in **Golang** that exposes a **GraphQL API** to manage elements, with:

- Authentication  
- Cursor-based pagination  
- Subscriptions  
- End-to-end testing

The goal is to evaluate:

- Design quality  
- Architectural decisions  
- Implementation skills  

---

## 1. Element Management

### API Representation

When returned by the API, an element must expose:

- `uri`
- `title`
- `type_uri`
- `space_uri`
- `creation_date`
- `author`
- `field_values`

Each item in `field_values` must contain:
- `value` — can be any type
- `uri`
- `field`

`field_values` must resolve the linked values
and `field` must resolve to a field object.

---

## 2. GraphQL Queries & Mutations

The GraphQL API must support the following operations:

### Mutations

- **Update an Elements** (updating `title` is sufficient)

### Queries

- **Retrieve a list of elements** using **cursor-based pagination** and a **basic filter on field_values**

### Subscriptions

- **Subscribe to updates** when an element is modified

---

## 3. Authentication

All API requests must include a header:

`X-User-ID: <some-user-identifier>`

The application must:

- Read this header through a middleware
- Reject requests where the header is **missing**
- Make the header value accessible within resolvers (for example via the context)
- Only return `elements` located in `spaces` where the user has permission

---

## 4. End-to-End Test

The project must include at least one end-to-end (E2E) test that:

- Starts the application (or runs against a running instance)
- Interacts with the **real GraphQL API**
- Covers **at minimum two different queries** of the GraphQL API

## 5. Stack

The GraphQL API must be implemented using ([gqlgen](https://github.com/99designs/gqlgen))

File structure is up to you.