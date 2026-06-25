# Routine Tracker Project Stack

## Tech Stack

### Frontend

- Next.js
- Tailwind CSS

### Backend

- Go
- Fiber
- GORM

### Database

- PostgreSQL

### Infrastructure

- Docker Compose
- GitHub Actions (CI/CD)

---

## Database Choice

**Use PostgreSQL.**

Reasons:

- Industry standard
- Excellent support in Go and GORM
- Fits relational data well (routines, logs, streaks, goals)
- Scales as the project grows

---

## Database Hosting Strategy

### Start with Self-Hosted PostgreSQL (Recommended)

Run PostgreSQL in Docker:

```text
Docker Compose
├── Next.js
├── Fiber
└── PostgreSQL
```

Benefits:

- Learn Docker networking
- Learn volumes and data persistence
- Learn environment configuration
- Learn deployment and backups
- Better understanding of backend infrastructure

---

## Development Roadmap

### Phase 1 - Build MVP

```text
Next.js
Fiber
GORM
PostgreSQL (Docker)
```

Features:

- Create routines
- Mark routines as completed
- View daily history

### Phase 2 - Add CI

GitHub Actions:

- Run tests
- Run linting/vetting
- Build Docker images

### Phase 3 - Deploy

Deploy to a VPS:

```text
VPS
├── Next.js Container
├── Fiber Container
└── PostgreSQL Container
```

Learn:

- Docker deployment
- Reverse proxies
- Environment management
- Server administration

### Phase 4 - Scale

Move database to a managed service if needed:

```text
VPS
├── Next.js
└── Fiber

Managed PostgreSQL
(Neon / Supabase / AWS RDS)
```

---

## Useful Tools

Database GUI:

- DBeaver (Free)
- TablePlus
- pgAdmin

Even when using GORM, learn basic SQL and inspect your database regularly.

---

## Final Recommendation

For learning:

- Go
- Fiber
- GORM
- Docker
- CI/CD

Use:

```text
Next.js
    ↓
Fiber
    ↓
GORM
    ↓
PostgreSQL (Docker)
```

Manage PostgreSQL yourself initially and move to a managed database only when the project becomes large enough to justify it.
