# queryx

Modern SQL extensions for Go — a drop-in replacement for [jmoiron/sqlx](https://github.com/jmoiron/sqlx).

queryx provides a thin wrapper around Go's `database/sql` that adds struct scanning, named parameters, and convenience methods without ORM complexity.

## Features

- **Struct scanning** — scan rows directly into structs with `db` tags
- **Named parameters** — use `:name` syntax in queries
- **Generic helpers** — type-safe `Get[T]` and `Select[T]` functions
- **In clause expansion** — expand slice arguments in `IN (?)` clauses
- **Multiple bind formats** — `$1` (Postgres), `?` (MySQL/SQLite), `@p1` (MSSQL), `:name` (Oracle)
- **Context-first API** — all methods require `context.Context`
- **Fixed Postgres parsing** — correctly handles `::type` casts and SQL comments
- **Zero reflect in hot path** — aggressive struct field mapping cache

## Installation

```bash
go get github.com/agentine/queryx
```

Requires Go 1.22 or later.

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/agentine/queryx"
	_ "github.com/lib/pq"
)

type User struct {
	ID    int    `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

func main() {
	ctx := context.Background()

	db, err := queryx.Connect(ctx, "postgres", "postgres://localhost/mydb?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Struct scanning with Get[T]
	user, err := queryx.Get[User](ctx, db, "SELECT * FROM users WHERE id = $1", 1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("User: %s <%s>\n", user.Name, user.Email)

	// Query multiple rows with Select[T]
	users, err := queryx.Select[User](ctx, db, "SELECT * FROM users WHERE name LIKE $1", "%alice%")
	if err != nil {
		log.Fatal(err)
	}
	for _, u := range users {
		fmt.Printf("  %d: %s\n", u.ID, u.Name)
	}

	// Named parameters
	_, err = db.NamedExecContext(ctx,
		"INSERT INTO users (name, email) VALUES (:name, :email)",
		map[string]interface{}{"name": "bob", "email": "bob@example.com"},
	)
	if err != nil {
		log.Fatal(err)
	}

	// In clause expansion
	query, args, err := queryx.In("SELECT * FROM users WHERE id IN (?)", []int{1, 2, 3})
	if err != nil {
		log.Fatal(err)
	}
	query = queryx.Rebind(queryx.DOLLAR, query)
	var result []User
	err = db.SelectContext(ctx, &result, query, args...)
	if err != nil {
		log.Fatal(err)
	}
}
```

## Bind Variable Formats

queryx supports rebinding queries to different placeholder formats:

| Constant         | Format  | Databases        |
|------------------|---------|------------------|
| `queryx.QUESTION`| `?`     | MySQL, SQLite    |
| `queryx.DOLLAR`  | `$1`    | PostgreSQL       |
| `queryx.AT`      | `@p1`   | SQL Server       |
| `queryx.NAMED`   | `:name` | Oracle           |

## Migration from sqlx

queryx is designed as a drop-in replacement for jmoiron/sqlx. See [MIGRATION.md](MIGRATION.md) for a detailed migration guide.

Key differences:
- Import `github.com/agentine/queryx` instead of `github.com/jmoiron/sqlx`
- All methods require `context.Context` (no non-context variants)
- New generic helpers: `Get[T]` and `Select[T]`
- Fixed Postgres `::type` cast parsing

## License

MIT
