# ray-backend

Routine-tracking REST API. Create routines, mark them complete, view daily history.

**Stack:** Go · [Fiber v3](https://gofiber.io/) · GORM · PostgreSQL

> Building it step by step? See [`step.md`](./step.md) — each step has a simple
> implementation and the best-practice version.

## Prerequisites

- Go 1.26+
- Docker + Docker Compose (runs Postgres)
- [`air`](https://github.com/air-verse/air) (optional, live reload): `go install github.com/air-verse/air@latest`

## Getting started

```bash
# 1. Config — copy template, edit values
cp .env.example .env

# 2. Start Postgres
docker compose up -d

# 3. Dependencies
go mod download

# 4. Run (live reload)
make air
#    ...or plain:
go run ./cmd/server
```

Verify:

```bash
curl http://localhost:8080/health   # -> ok
```

### Configuration

The app reads `config.yml` for defaults; matching environment variables override it
(`DB_HOST` overrides `db.host`, `PORT` overrides `app.port`, …). `docker compose` reads
`.env` to provision Postgres. Keep `.env` DB values and `config.yml` in sync, or rely on
the env vars to win.

`.env` is gitignored (secrets). `config.yml` is committed (safe local defaults only —
never put real secrets in it).

## API

Base path `/api`.

| Method | Path                          | Description           |
|--------|-------------------------------|-----------------------|
| GET    | `/health`                     | health check          |
| POST   | `/api/routines`               | create routine        |
| GET    | `/api/routines`               | list routines         |
| GET    | `/api/routines/:id`           | get routine           |
| POST   | `/api/routines/:id/complete`  | mark routine complete |
| GET    | `/api/history?date=YYYY-MM-DD`| daily history         |

## Layout

```
cmd/server         entrypoint
internal/config    viper config loader
internal/database  GORM + Postgres connection
internal/models    data models
internal/services  business logic (no HTTP)
internal/handlers  HTTP boundary (no business logic)
internal/routes    route + middleware registration
```
