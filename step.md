# Routine Tracker — Build Guide

This guide builds the API step by step. **Every step has two parts:**

- **Simple implementation** — the smallest thing that works. This is what gets you running.
- **Best practice** — how you'd write it for a real/production codebase, and *why*.

Start with the simple version to learn the flow, then revisit the best-practice notes
when hardening the project. Many best-practice items are optional for a learning project
but expected in production.

---

## Where we are

```
ray-backend/
├── cmd/server/main.go          entrypoint
├── internal/
│   ├── config/config.go        viper config loader
│   ├── models/base.go          base model (ID, timestamps, soft delete)
│   ├── models/routine.go       Routine + RoutineLog structs
│   ├── database/database.go    GORM + Postgres connection
│   ├── handlers/               HTTP handlers
│   ├── services/               business logic
│   └── routes/                 route registration
├── docker-compose.yml          Postgres container
├── .env.example                DB + PORT vars
├── config.yml                  committed defaults
└── Makefile                    `make air`
```

---

## Dev setup — Air live reload

Install once:

```bash
go install github.com/air-verse/air@latest
```

`.air.toml`:

```toml
root = "."
tmp_dir = "tmp"

[build]
  bin = "./tmp/server"
  cmd = "go build -o ./tmp/server ./cmd/server"
  delay = 1000
  include_ext = ["go", "yml", "yaml", "toml"]
  exclude_dir = ["tmp", "vendor"]
  kill_delay = "0s"

[log]
  time = true
```

`Makefile` (real tab, not spaces):

```makefile
.PHONY: air
air:
	air
```

Add `tmp/` to `.gitignore`. Then `make air` rebuilds + restarts on every save.

> **Note:** `air` warns `build.bin is deprecated; set build.entrypoint instead` on newer
> versions. Harmless — `bin` still works. To silence it, rename `bin` → `entrypoint`.

---

## Step 1 — Models & foreign keys

`RoutineLog` belongs to a `Routine`. The link is a foreign key.

### Simple implementation

GORM infers the foreign key from the naming convention — a `uint` field named
`RoutineID` is understood to point at `Routine`:

```go
type RoutineLog struct {
    Base
    RoutineID   uint      `gorm:"not null" json:"routine_id"`
    CompletedAt time.Time `json:"completed_at"`
}
```

> **Bug to avoid:** `gorm:"foreignKey:ID"` on a plain `uint` does nothing — that tag is for
> *association fields* (a field whose type is another struct). On a `uint` column GORM
> silently ignores it. Use `gorm:"not null"` and let the naming convention do the work.

### Best practice

- **Add the association field** so you can preload the parent and let GORM enforce the
  constraint:

  ```go
  type RoutineLog struct {
      Base
      RoutineID   uint      `gorm:"not null;index" json:"routine_id"`
      Routine     Routine   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
      CompletedAt time.Time `gorm:"not null" json:"completed_at"`
  }
  ```

  - `index` on `RoutineID` — every "logs for this routine" query filters on it; without an
    index that's a full table scan.
  - `constraint:OnDelete:CASCADE` — deleting a routine removes its logs at the DB level,
    not just in Go.
  - `json:"-"` hides the embedded struct from API responses unless you explicitly preload it.
- **Be deliberate about soft delete.** `Base` includes `gorm.DeletedAt`, so `Delete` only
  sets `deleted_at`; rows stay. Good for audit trails, but every query must account for it
  (GORM does automatically). Know which behavior you want.

---

## Step 2 — Config: config.yml + env overrides

`config.yml` holds defaults; environment variables override them at runtime (Docker, CI,
prod) without editing the file.

### Simple implementation

```go
func Load() *Config {
    viper.SetConfigFile("config.yml")
    viper.AutomaticEnv()
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

    if err := viper.ReadInConfig(); err != nil {
        log.Fatalf("config read error: %v", err)
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        log.Fatalf("config unmarshal error: %v", err)
    }
    return &cfg
}
```

`AutomaticEnv` + the `.`→`_` replacer means viper checks `DB_HOST` for `db.host`,
`APP_PORT` for `app.port`, etc. Precedence (high→low): **env var > config.yml > defaults**.
Viper does **not** read `.env` itself — a tool (air, your shell, docker compose) loads
`.env` into the environment first.

### Best practice

- **Return an error instead of `log.Fatal`.** A config loader is library code — let
  `main` decide to exit. `log.Fatal` mid-package kills testability:

  ```go
  func Load() (*Config, error) {
      ...
      if err := viper.ReadInConfig(); err != nil {
          return nil, fmt.Errorf("read config: %w", err)
      }
      ...
      return &cfg, nil
  }
  ```

- **Set defaults in code,** so the app still boots if `config.yml` is missing:
  `viper.SetDefault("app.port", "8080")`.
- **Validate required values** after unmarshal (e.g. DB host/user non-empty) and fail
  fast with a clear message — beats a cryptic connection error later.
- **Never commit real secrets** to `config.yml` (it's tracked). It currently has
  `password: ray` — fine only because that's a throwaway local password. Real secrets go
  in `.env` (gitignored) or a secrets manager.
- **`SetEnvPrefix("RAY")`** to namespace your vars (`RAY_DB_HOST`) and avoid collisions
  with unrelated environment variables on the host.

---

## Step 3 — Connect to Postgres & migrate

Open the connection once at startup; share the `*gorm.DB` (it's a connection pool)
everywhere. Never call `gorm.Open` inside a request handler or service method.

### Simple implementation

```go
func Connect(cfg config.DB) (*gorm.DB, error) {
    db, err := gorm.Open(postgres.Open(cfg.DSN()))
    if err != nil {
        return nil, err
    }
    db.AutoMigrate(&models.Routine{}, &models.RoutineLog{})
    return db, nil
}
```

`AutoMigrate` reads the structs and creates/updates matching tables. It only **adds**
tables/columns/indexes — it never drops or alters existing columns.

### Best practice

- **Capture the migrate error.** The simple version ignores it — a failed migration goes
  unnoticed until queries break:

  ```go
  if err := db.AutoMigrate(models.AllModels()...); err != nil {
      return nil, fmt.Errorf("automigrate: %w", err)
  }
  ```

- **Central model registry** instead of listing models inline. With 30 models you don't
  edit `database.go` every time — GORM has no auto-discovery, so each model *must* be
  listed somewhere:

  ```go
  // internal/models/models.go
  func AllModels() []any {
      return []any{
          &Routine{},     // parents first...
          &RoutineLog{},  // ...children after (FK target must exist)
      }
  }
  ```

  Order matters: a model with a foreign key must come **after** the model it references,
  or GORM may fail creating the constraint.
- **Configure the connection pool** — defaults are unbounded and will exhaust Postgres
  under load:

  ```go
  sqlDB, _ := db.DB()
  sqlDB.SetMaxOpenConns(25)
  sqlDB.SetMaxIdleConns(5)
  sqlDB.SetConnMaxLifetime(time.Hour)
  ```

- **Tune the GORM logger** (`logger.Default.LogMode(logger.Warn)`) so dev logs aren't
  drowned in every SQL statement; raise to `Info` when debugging queries.
- **Don't `AutoMigrate` in production.** It can't express renames, data backfills, or safe
  column drops, and running schema changes automatically on every deploy is risky. Graduate
  to versioned migration files ([`golang-migrate`](https://github.com/golang-migrate/migrate)
  or [`goose`](https://github.com/pressly/goose)); keep `AutoMigrate` for local/dev only.

---

## Step 4 — Service layer

Services hold business logic. They know the database; they know nothing about HTTP.

### Simple implementation

```go
type RoutineService struct {
    db *gorm.DB
}

func NewRoutineService(db *gorm.DB) *RoutineService {
    return &RoutineService{db: db}
}

func (s *RoutineService) Create(name, description string) (models.Routine, error) {
    r := models.Routine{Name: name, Description: description}
    res := s.db.Create(&r)
    return r, res.Error
}
```

Methods to implement: `Create`, `List`, `Get`, `Complete`, `DailyHistory`.

**GORM ↔ SQL cheat sheet:**

| Want | GORM | SQL |
|------|------|-----|
| Insert | `db.Create(&t)` | `INSERT` |
| All rows | `db.Find(&ts)` | `SELECT *` |
| One by ID | `db.First(&t, id)` | `SELECT … WHERE id=? LIMIT 1` |
| Filter | `db.Where("x = ?", v).Find(&ts)` | `WHERE x = ?` |
| Count | `db.Model(&T{}).Where(…).Count(&n)` | `SELECT COUNT(*)` |

### Best practice

- **Take `context.Context` as the first parameter** and pass it to GORM with
  `WithContext` — enables request cancellation and per-request timeouts:

  ```go
  func (s *RoutineService) Create(ctx context.Context, name, desc string) (models.Routine, error) {
      r := models.Routine{Name: name, Description: desc}
      res := s.db.WithContext(ctx).Create(&r)
      return r, res.Error
  }
  ```

- **Avoid N+1 queries in `DailyHistory`.** The naive version loops over routines and runs
  a count query per routine — 1 + N queries. For N routines that's death by round-trips.
  Do it in one query with a `LEFT JOIN` / `GROUP BY`, or fetch all of the day's logs once
  and match in Go.
- **Wrap writes that touch multiple tables in a transaction** (`s.db.Transaction(func(tx *gorm.DB) error { … })`)
  so a partial failure rolls back.
- **Depend on an interface, not `*gorm.DB`,** at the boundaries you want to test — lets you
  swap a mock. (For this size, a real in-memory SQLite DB in tests is simpler — see Step 8.)
- **Wrap errors with context:** `fmt.Errorf("create routine: %w", err)` so logs say *what*
  failed, not just `record not found`.

---

## Step 5 — Handlers

Handlers are the HTTP boundary. Each does exactly three things: parse the request, call a
service, return JSON. **No business logic, no SQL, no date math here** — push that into the
service.

### Simple implementation

```go
func (h *RoutineHandler) Create(c fiber.Ctx) error {
    var in struct {
        Name        string `json:"name"`
        Description string `json:"description"`
    }
    if err := c.Bind().Body(&in); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
    }
    r, err := h.svc.Create(in.Name, in.Description)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
    }
    return c.Status(fiber.StatusCreated).JSON(r)
}
```

Input parsing patterns:

```go
id, err := strconv.Atoi(c.Params("id"))      // URL param  /api/routines/:id
date, err := time.Parse("2006-01-02", c.Query("date"))  // query  ?date=2026-06-12
```

### Best practice

- **Validate input,** don't just bind it. Empty `name`, negative id, bad date → return
  `400` with a specific message *before* calling the service. A validation library like
  [`go-playground/validator`](https://github.com/go-playground/validator) with struct tags
  (`validate:"required,min=1"`) scales better than hand-written `if`s.
- **Don't leak internal errors to clients.** `err.Error()` can expose SQL/schema details.
  Log the real error server-side; return a generic message + a request ID to the client.
- **Map errors to correct status codes.** `gorm.ErrRecordNotFound` → `404`, not `500`:

  ```go
  if errors.Is(err, gorm.ErrRecordNotFound) {
      return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "routine not found"})
  }
  ```

- **Consistent error envelope** across all handlers (e.g. always `{"error": "..."}` or a
  richer `{"error": {"code": "...", "message": "..."}}`) so the frontend can rely on one shape.
- **Named request/response DTO types** instead of anonymous structs once they repeat —
  easier to document and reuse.

---

## Step 6 — Routes & middleware

Middleware runs before every handler, top to bottom.

### Simple implementation

```go
func Register(app *fiber.App, h *handlers.RoutineHandler) {
    app.Use(logger.New())
    app.Use(recover.New())
    app.Use(cors.New())

    app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })

    api := app.Group("/api")
    api.Post("/routines", h.Create)
    api.Get("/routines", h.List)
    api.Get("/routines/:id", h.Get)
    api.Post("/routines/:id/complete", h.Complete)
    api.Get("/history", h.DailyHistory)
}
```

- `logger.New()` — logs each request
- `recover.New()` — catches panics so one bad handler doesn't crash the server
- `cors.New()` — lets the frontend (different origin) call the API

### Best practice

- **Lock down CORS.** `cors.New()` with defaults allows all origins — fine for local dev,
  unsafe in production. Set an explicit allow-list:

  ```go
  app.Use(cors.New(cors.Config{
      AllowOrigins: []string{"https://app.example.com"},
      AllowMethods: []string{"GET", "POST"},
  }))
  ```

- **Version the API:** `app.Group("/api/v1")`. Lets you ship breaking changes under `/v2`
  without breaking existing clients.
- **Split liveness from readiness.** `/health` ("process is up") vs `/ready` ("DB is
  reachable", pinging `sqlDB.Ping()`). Orchestrators (k8s, compose healthchecks) use them
  differently.
- **Add a request-timeout middleware** so a slow query can't hold a connection forever, and
  a **request-ID** middleware so logs and client errors can be correlated.

---

## Step 7 — Wire everything in main.go

### Simple implementation

```go
func main() {
    cfg := config.Load()

    db, err := database.Connect(cfg.DB)
    if err != nil {
        log.Fatalf("database connection failed: %v", err)
    }

    svc := services.NewRoutineService(db)
    handler := handlers.NewRoutineHandler(svc)

    app := fiber.New()
    routes.Register(app, handler)

    log.Fatal(app.Listen(":" + cfg.App.Port))
}
```

Order: load config → connect DB (fatal on error) → build service → build handler →
create app → register routes → listen.

### Best practice

- **Extract a `run() error`** and keep `main` tiny. `log.Fatal` skips deferred cleanup
  (`defer` doesn't run on `os.Exit`); returning an error lets you close resources first:

  ```go
  func main() {
      if err := run(); err != nil {
          log.Fatal(err)
      }
  }
  ```

- **Graceful shutdown.** Listen for `SIGINT`/`SIGTERM`, then `app.Shutdown()` so in-flight
  requests finish and the DB pool closes cleanly instead of dropping connections:

  ```go
  go func() { _ = app.Listen(":" + cfg.App.Port) }()
  quit := make(chan os.Signal, 1)
  signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
  <-quit
  _ = app.ShutdownWithTimeout(10 * time.Second)
  ```

- **Use a structured logger** (`log/slog`) instead of the standard `log`, so production
  logs are queryable JSON.

---

## Step 8 — Tests

Go's standard `testing` package needs no extra libraries.

### Simple implementation

Test the service against an **in-memory SQLite** DB — no Docker needed:

```bash
go get gorm.io/driver/sqlite
```

```go
func newTestDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(models.AllModels()...); err != nil { t.Fatal(err) }
    return db
}

func TestCreate(t *testing.T) {
    svc := services.NewRoutineService(newTestDB(t))
    r, err := svc.Create("Morning run", "")
    if err != nil { t.Fatal(err) }
    if r.ID == 0 { t.Fatal("expected ID to be set") }
}
```

```bash
go test ./...
```

### Best practice

- **Table-driven tests** for multiple cases in one function (clearer than copy-paste):

  ```go
  tests := []struct{ name, input string; wantErr bool }{
      {"valid", "Run", false},
      {"empty", "", true},
  }
  for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) { /* ... */ })
  }
  ```

- **Integration tests against real Postgres** with
  [testcontainers-go](https://github.com/testcontainers/testcontainers-go). SQLite and
  Postgres differ (types, constraints, SQL dialect) — a test that passes on SQLite can
  hide a Postgres bug. Use SQLite for fast unit tests, Postgres for the critical paths.
- **`t.Cleanup()`** to tear down resources instead of manual defer chains.
- **Check coverage:** `go test -cover ./...`; focus it on the service layer where the
  logic lives.

---

## Step 9 — Dockerize

A **two-stage build** keeps the final image small: stage 1 has the Go toolchain and
compiles; stage 2 is a tiny image with only the binary.

### Simple implementation

```dockerfile
# Stage 1 — build
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server ./cmd/server

# Stage 2 — runtime
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /src/server .
COPY config.yml .
EXPOSE 8080
CMD ["./server"]
```

Add an `api` service to `docker-compose.yml`:

```yaml
api:
  build: .
  ports:
    - "8080:8080"
  env_file:
    - .env
  depends_on:
    - postgres
```

> **Docker networking:** inside Docker, `localhost` is the container itself — Go can't reach
> Postgres that way. Set `DB_HOST=postgres` (the compose service name acts as the hostname).

### Best practice

- **Add a `.dockerignore`** (`.git`, `tmp/`, `*.md`, `.env`) so secrets and junk don't get
  copied into the image and the build context stays small.
- **`CGO_ENABLED=0`** for a fully static binary, then run on `scratch` or
  [`distroless`](https://github.com/GoogleContainerTools/distroless) — smaller and far less
  attack surface than alpine:

  ```dockerfile
  RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server
  ```

- **Run as non-root** (`USER nonroot`) — don't let a container compromise run as root.
- **Pin versions** (`golang:1.26.4-alpine`, `postgres:16.3`) so builds are reproducible —
  `latest` drifts. *(Note: this repo's Dockerfile pins `golang:1.23` but `go.mod` requires
  1.26 — bump it or the build fails.)*
- **`depends_on` with a healthcheck condition** so the API waits until Postgres actually
  accepts connections, not just until the container starts:

  ```yaml
  postgres:
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER"]
      interval: 5s
      retries: 5
  api:
    depends_on:
      postgres:
        condition: service_healthy
  ```

- **Use Docker build cache mounts** (`RUN --mount=type=cache,target=/go/pkg/mod`) to speed
  up repeated builds.

---

## End-to-end smoke test

After `make air` (or `docker compose up --build`):

```bash
curl http://localhost:8080/health
# -> ok

curl -X POST http://localhost:8080/api/routines \
  -H "Content-Type: application/json" \
  -d '{"name": "Morning run", "description": "5km easy pace"}'
# -> 201 + JSON with id

curl http://localhost:8080/api/routines
# -> 200 + array of one

curl -X POST http://localhost:8080/api/routines/1/complete
# -> 201 + log with routine_id, completed_at

curl "http://localhost:8080/api/history?date=2026-06-23"
# -> 200 + array where "Morning run" has "completed": true
```

> **Common gotcha:** omitting `-H "Content-Type: application/json"` on POSTs — Fiber's
> `Bind().Body()` then fails to parse and your body is empty.

---

## Troubleshooting

| Error | Fix |
|-------|-----|
| `connection refused` on start | `docker compose ps` — is `ray-postgres` running? |
| `port is already allocated` | Another process owns the host port. `lsof -nP -iTCP:<port> -sTCP:LISTEN`; stop it or change `DB_PORT` in `.env`. |
| `connection reset by peer` on DB | Host port maps to the wrong container port. Mapping must be `<host>:5432` (Postgres always listens on 5432 inside the container). |
| `password authentication failed` | `.env` values don't match what Postgres was first created with. The volume persists old creds — `docker compose down -v` to reset. |
| Tables missing after connect | `AutoMigrate` not called, errored silently, or model not in `AllModels()`. |
| Bind error on POST | Missing `-H "Content-Type: application/json"`. |
| `404` on all `/api/` routes | `routes.Register` not called before `app.Listen`. |
| Panic in terminal | `recover.New()` not added, or a nil service/handler. |
| CORS error from frontend | `cors.New()` not added, or origin not in the allow-list. |
| `go.mod requires go >= 1.26` in Docker | Builder image Go version is older than `go.mod`. Bump the `golang:` tag. |

---

## Next steps

Steps 1–9 ship a working MVP: create/list/get/complete routines and a daily history,
served from Postgres, tested, and dockerized. Steps 10–17 take it from "runs on my machine"
to "I'd let other people use it." Same two-part format — **Simple implementation** to get it
working, **Best practice** (with examples) for when you harden it.

## Step 10 — Finish the CRUD: Update & Delete

A routine can be created but never edited or removed. Add `Update` and `Delete` to the
service, handlers, and routes. `Delete` is a **soft delete** — `Base.DeletedAt` makes GORM
set `deleted_at` instead of removing the row.

### Simple implementation

Service:

```go
func (s *RoutineService) Update(ctx context.Context, id uint, name, description string) (models.Routine, error) {
    var routine models.Routine
    if err := s.db.WithContext(ctx).First(&routine, id).Error; err != nil {
        return models.Routine{}, fmt.Errorf("failed to get routine: %w", err)
    }
    if name != "" {
        routine.Name = name
    }
    if description != "" {
        routine.Description = description
    }
    if err := s.db.WithContext(ctx).Save(&routine).Error; err != nil {
        return models.Routine{}, fmt.Errorf("failed to update routine: %w", err)
    }
    return routine, nil
}

func (s *RoutineService) Delete(ctx context.Context, id uint) error {
    res := s.db.WithContext(ctx).Delete(&models.Routine{}, id)
    if res.Error != nil {
        return fmt.Errorf("failed to delete routine: %w", res.Error)
    }
    return nil
}
```

Routes:

```go
api.Put("/routines/:id", h.Update)
api.Delete("/routines/:id", h.Delete)
```

> **Bug to avoid — load *then* mutate.** If you set `routine.Name = name` *before* calling
> `First(&routine, id)`, the `First` scan overwrites the struct and your changes are lost —
> the update silently no-ops. Always `First` first, apply changes second, `Save` last.

### Best practice

- **Detect the missing row on delete.** GORM's `Delete` does **not** return
  `ErrRecordNotFound` — deleting a non-existent id gives `err == nil` and
  `RowsAffected == 0`. Without this check the handler returns `200 "deleted"` for a routine
  that never existed:

  ```go
  func (s *RoutineService) Delete(ctx context.Context, id uint) error {
      res := s.db.WithContext(ctx).Delete(&models.Routine{}, id)
      if res.Error != nil {
          return fmt.Errorf("failed to delete routine: %w", res.Error)
      }
      if res.RowsAffected == 0 {
          return gorm.ErrRecordNotFound
      }
      return nil
  }
  ```

- **Map `ErrRecordNotFound` → `404`** in both handlers, same as `Get`:

  ```go
  if err := h.svc.Delete(c.Context(), uint(id)); err != nil {
      if errors.Is(err, gorm.ErrRecordNotFound) {
          return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "routine not found"})
      }
      return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
  }
  ```

- **Pick the right verb for partial vs full update.** A handler that only changes the
  non-empty fields is `PATCH` semantics. Either use `api.Patch(...)`, or accept the whole
  object and replace it for true `PUT`. Don't label a partial update `PUT`.
- **Soft delete leaves child rows.** `RoutineLog`'s `OnDelete:CASCADE` only fires on a real
  `DELETE`; a soft delete is an `UPDATE`, so the logs linger (invisible, since queries
  filter `deleted_at IS NULL`). Fine here — just know they're there if you ever hard-delete.

---

## Step 11 — Unique routine name (409 Conflict)

Two live routines named "Morning run" is a bug, not a feature. Enforce uniqueness at the
**database** (the only race-safe guard), then translate the violation to `409`.

### Simple implementation

Tag the field so new databases get the index:

```go
// internal/models/routine.go
Name string `gorm:"not null;uniqueIndex" json:"name"`
```

`AutoMigrate` then runs `CREATE UNIQUE INDEX idx_routines_name ON routines(name)`.

> **Two gotchas on an existing table:**
> 1. **Duplicates already present → migration fails.** Postgres won't build a unique index
>    over duplicate rows. Find and clear them first:
>    ```sql
>    SELECT name, COUNT(*) FROM routines GROUP BY name HAVING COUNT(*) > 1;
>    ```
> 2. **Soft delete breaks a plain `uniqueIndex`.** It covers *all* rows incl. soft-deleted,
>    so create → delete → recreate the same name fails. Use the partial index below.

### Best practice

- **Partial unique index on live rows only.** GORM struct tags can't express a `WHERE`
  clause, so leave the tag plain and create the index after `AutoMigrate`:

  ```go
  // internal/database/database.go, after AutoMigrate
  if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_routines_name_active
      ON routines (name) WHERE deleted_at IS NULL`).Error; err != nil {
      return nil, fmt.Errorf("create unique name index: %w", err)
  }
  ```

  > Don't try a composite `uniqueIndex` on `(name, deleted_at)` — Postgres treats `NULL`s
  > as distinct, so two live rows (`deleted_at = NULL`) slip past. Must be the partial
  > `WHERE deleted_at IS NULL` index.

- **Catch the violation, don't pre-`SELECT`.** A "does this name exist?" check before
  insert is a race — two requests both see "free" and both insert. Let the DB reject it and
  translate Postgres error code `23505`:

  ```go
  import "github.com/jackc/pgx/v5/pgconn"

  var ErrDuplicateName = errors.New("routine name already exists")

  if err := s.db.WithContext(ctx).Create(&r).Error; err != nil {
      var pgErr *pgconn.PgError
      if errors.As(err, &pgErr) && pgErr.Code == "23505" {
          return models.Routine{}, ErrDuplicateName
      }
      return models.Routine{}, fmt.Errorf("failed to create routine: %w", err)
  }
  ```

- **Map `ErrDuplicateName` → `409`** in the handler (do the same in `Update` — a rename to
  a taken name also hits `23505`):

  ```go
  if errors.Is(err, services.ErrDuplicateName) {
      return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "routine name already exists"})
  }
  ```

- **`go mod tidy`** after importing `pgconn` — it's currently an indirect dependency.
- **Test the 409 against real Postgres.** `pgconn.PgError` won't surface from the SQLite
  test DB (different error type), so the conflict path needs a Postgres integration test
  (Step 15).

---

## Step 12 — Input validation as a layer

Hand-written `if name == ""` works for one field; it won't scale. Move validation to a
declarative layer with [`go-playground/validator`](https://github.com/go-playground/validator),
running at the HTTP boundary *before* the service is called.

### Simple implementation

```bash
go get github.com/go-playground/validator/v10
```

One shared validator instance (reused — don't construct per request):

```go
// internal/handlers/validate.go
package handlers

import "github.com/go-playground/validator/v10"

var validate = validator.New(validator.WithRequiredStructEnabled())
```

Tagged DTOs, validated in the handler:

```go
type createRoutineInput struct {
    Name        string `json:"name"        validate:"required,min=1,max=100"`
    Description string `json:"description" validate:"max=1000"`
}

func (h *RoutineHandler) Create(c fiber.Ctx) error {
    var input createRoutineInput
    if err := c.Bind().Body(&input); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
    }
    if err := validate.Struct(input); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }
    // ... call service
}
```

The tag replaces the manual empty-name check in both the handler *and* the service — the
boundary now guarantees valid input, so the service can trust it.

> **`omitempty` for partial updates.** `Update` is partial (empty field = "leave it"), so
> its DTO must not mark fields `required`: use `validate:"omitempty,min=1,max=100"`. The
> rules apply only when the field is present.

### Best practice

- **Return field-level messages,** not validator's raw dump. Map the error:

  ```go
  func validationErrors(err error) map[string]string {
      out := map[string]string{}
      var ve validator.ValidationErrors
      if errors.As(err, &ve) {
          for _, fe := range ve {
              switch fe.Tag() {
              case "required":
                  out[fe.Field()] = fe.Field() + " is required"
              case "max":
                  out[fe.Field()] = fe.Field() + " too long (max " + fe.Param() + ")"
              default:
                  out[fe.Field()] = "invalid " + fe.Field()
              }
          }
      }
      return out
  }
  // → {"errors": {"Name": "Name is required"}}
  ```

- **Hook the validator into Fiber** so `Bind().Body()` validates automatically (one less
  call per handler):

  ```go
  type structValidator struct{ v *validator.Validate }
  func (s structValidator) Validate(out any) error { return s.v.Struct(out) }

  app := fiber.New(fiber.Config{
      StructValidator: structValidator{v: validator.New()},
  })
  ```

- **Keep validation (`400`) separate from uniqueness (`409`).** The validator can't know a
  name is already taken — that's a DB round-trip (Step 11). Validator → `400`,
  `23505` → `409`.

---

## Step 13 — Harden routes & middleware

Step 6 shipped the middleware defaults. Here are the production versions, each with the
example.

### Simple implementation

The Step 6 stack — `logger`, `recover`, `cors.New()`, one `/health` — is the simple
version. Everything below tightens it.

### Best practice

- **Config-driven CORS allow-list** instead of all-origins default, so dev and prod differ
  without code changes:

  ```go
  app.Use(cors.New(cors.Config{
      AllowOrigins: cfg.CORS.AllowedOrigins, // ["http://localhost:5173"] in dev
      AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
  }))
  ```

- **Request-ID middleware before the logger,** so every log line and error carries a
  correlation ID:

  ```go
  import "github.com/gofiber/fiber/v3/middleware/requestid"

  app.Use(requestid.New())
  app.Use(logger.New())
  // in a handler: reqID := requestid.FromContext(c)
  ```

- **Request-timeout middleware** so a slow query can't pin a connection. Relies on the
  `WithContext(ctx)` discipline already in the service layer to actually cancel the query:

  ```go
  app.Use(func(c fiber.Ctx) error {
      ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
      defer cancel()
      c.SetContext(ctx)
      return c.Next()
  })
  ```

- **Split liveness from readiness** — orchestrators use them differently:

  ```go
  app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") }) // process up
  app.Get("/ready", func(c fiber.Ctx) error {                                // deps reachable
      sqlDB, _ := db.DB()
      if err := sqlDB.Ping(); err != nil {
          return c.SendStatus(fiber.StatusServiceUnavailable)
      }
      return c.SendString("ready")
  })
  ```

- **Version the API** so breaking changes ship under `/v2` without breaking clients:

  ```go
  api := app.Group("/api/v1")
  ```

---

## Step 14 — Production-grade `main.go`

Step 7's `main` loads, connects, and listens — but `log.Fatal` skips cleanup and a `Ctrl-C`
drops in-flight requests. Harden the entrypoint.

### Simple implementation

The Step 7 `main` (load config → connect → build → listen) is the simple version.

### Best practice

- **Extract `run() error`** so deferred cleanup runs (`defer` doesn't fire on
  `os.Exit`, which `log.Fatal` calls):

  ```go
  func main() {
      if err := run(); err != nil {
          log.Fatal(err)
      }
  }

  func run() error {
      cfg, err := config.Load()
      if err != nil {
          return err
      }
      // ... build app
  }
  ```

- **Graceful shutdown** on `SIGINT`/`SIGTERM` so in-flight requests finish and the pool
  closes cleanly:

  ```go
  go func() { _ = app.Listen(":" + cfg.App.Port) }()

  quit := make(chan os.Signal, 1)
  signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
  <-quit
  _ = app.ShutdownWithTimeout(10 * time.Second)
  ```

- **Structured logging** with `log/slog` instead of the standard `log`, so prod logs are
  queryable JSON:

  ```go
  logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
  slog.SetDefault(logger)
  slog.Info("server starting", "port", cfg.App.Port)
  ```

- **Configure the connection pool** (Step 3) once the DB is open:

  ```go
  sqlDB, _ := db.DB()
  sqlDB.SetMaxOpenConns(25)
  sqlDB.SetMaxIdleConns(5)
  sqlDB.SetConnMaxLifetime(time.Hour)
  ```

---

## Step 15 — Broaden the test suite

Only `Create` is covered. Add cases for every method, with the gnarly date logic getting
the most attention.

### Simple implementation

Table-driven service tests against the in-memory SQLite DB from Step 8. The highest-value
target is `DailyHistory` — its day-boundary JOIN is the easiest thing to get subtly wrong:

```go
func TestDailyHistory(t *testing.T) {
    tests := []struct {
        name          string
        completeOnDay bool
        offsetDays    int  // query date relative to completion day
        wantCompleted bool
    }{
        {"completed today",     true,  0,  true},
        {"not completed",       false, 0,  false},
        {"completed other day", true,  -1, false}, // log exists, wrong day
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            svc := services.NewRoutineService(newTestDB(t)) // fresh DB per case
            r, _ := svc.Create(t.Context(), "Run", "")
            if tt.completeOnDay {
                svc.Complete(t.Context(), r.ID)
            }
            day := time.Now().AddDate(0, 0, tt.offsetDays)
            entries, err := svc.DailyHistory(t.Context(), day)
            if err != nil {
                t.Fatal(err)
            }
            if entries[0].Completed != tt.wantCompleted {
                t.Fatalf("completed = %v, want %v", entries[0].Completed, tt.wantCompleted)
            }
        })
    }
}
```

### Best practice

- **Handler tests** via `app.Test(req)` to assert status codes and JSON shape end-to-end:

  ```go
  req := httptest.NewRequest("GET", "/api/routines/999", nil)
  resp, _ := app.Test(req)
  if resp.StatusCode != fiber.StatusNotFound {
      t.Fatalf("got %d, want 404", resp.StatusCode)
  }
  ```

- **Integration tests against real Postgres** with
  [testcontainers-go](https://github.com/testcontainers/testcontainers-go) — SQLite and
  Postgres differ enough (types, constraints, the `23505` path from Step 11) that a green
  SQLite test can hide a Postgres bug:

  ```go
  pg, _ := postgres.Run(ctx, "postgres:16",
      postgres.WithDatabase("test"), postgres.WithUsername("t"), postgres.WithPassword("t"))
  t.Cleanup(func() { pg.Terminate(ctx) })
  ```

- **`t.Cleanup()`** for teardown instead of manual defer chains, and **`go test -cover ./...`**
  to find untested branches — focus coverage on the service layer where the logic lives.

---

## Step 16 — Versioned migrations

`AutoMigrate` only *adds* — it can't express renames, backfills, or safe column drops, and
auto-running schema changes on every deploy is risky. Graduate to checked-in migration
files.

### Simple implementation

Keep `AutoMigrate` for local/dev and tests; that's the simple version and it's fine until
you ship to a shared environment.

### Best practice

- **Adopt [`golang-migrate`](https://github.com/golang-migrate/migrate)** (or
  [`goose`](https://github.com/pressly/goose)). Each change is a numbered `up`/`down` pair
  checked into the repo:

  ```
  migrations/
  ├── 000001_create_routines.up.sql
  ├── 000001_create_routines.down.sql
  ├── 000002_unique_name_index.up.sql   -- the Step 11 partial index lives here
  └── 000002_unique_name_index.down.sql
  ```

  ```sql
  -- 000002_unique_name_index.up.sql
  CREATE UNIQUE INDEX IF NOT EXISTS idx_routines_name_active
      ON routines (name) WHERE deleted_at IS NULL;
  ```

- **Run migrations as an explicit deploy step,** not on app boot:

  ```bash
  migrate -path ./migrations -database "$DATABASE_URL" up
  ```

- **Disable `AutoMigrate` in production** (gate it on `cfg.App.Mode != "production"`) so the
  migration files are the single source of schema truth.

---

## Step 17 — CI pipeline

Stop "works on my machine" regressions before they merge. A push-triggered workflow that
builds, vets, and tests.

### Simple implementation

A GitHub Actions workflow at `.github/workflows/ci.yml`:

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26' }
      - run: go vet ./...
      - run: go test -race ./...
      - run: go build ./...
```

### Best practice

- **Add static analysis** with [`golangci-lint`](https://golangci-lint.run/) — catches far
  more than `go vet`:

  ```yaml
      - uses: golangci/golangci-lint-action@v6
  ```

- **Run integration tests too.** Actions provides Postgres as a service container, so the
  testcontainers/Postgres tests from Step 15 run in CI:

  ```yaml
      services:
        postgres:
          image: postgres:16
          env: { POSTGRES_PASSWORD: test }
          ports: ["5432:5432"]
          options: >-
            --health-cmd pg_isready --health-interval 5s --health-retries 5
  ```

- **Gate merges on CI** with a branch-protection rule, and **report coverage**
  (`go test -coverprofile=cover.out ./...`) so it can't silently rot.

---

## Further out

- **Auth & multi-tenant** — users own their routines (`user_id` FK, JWT or session
  middleware, scope every query by the authenticated user).
- **Pagination & filtering** on `List` (`?limit=&offset=`, or keyset pagination) before the
  table grows.
- **Observability** — structured logs + metrics (Prometheus `/metrics`) + tracing.
- **Rate limiting** (`limiter` middleware) on write endpoints.
- **OpenAPI spec** generated from the handlers so the frontend has a typed contract.
