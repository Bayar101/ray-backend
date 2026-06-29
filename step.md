# Migrating ray-backend to Domain-Driven Design

This guide walks you through restructuring the project from its current **technical layering**
(everything grouped by *what kind of code* it is — `models/`, `services/`, `handlers/`) into a
**Domain-Driven Design** layout (everything grouped by *what part of the business* it serves —
`routine/`, `finance/`).

It's written for *this* codebase. Every "before" snippet is real code from the repo today, and
every "after" snippet is where that same code lands. You can follow it top to bottom and end with
a working, refactored API — no rewrites from scratch.

---

## Part 0 — What DDD actually is (and what it isn't)

Domain-Driven Design is one core idea with some structure bolted on:

> **The code should be organized around the business domain, and the rules of the business
> should live in objects that model that business — not scattered across service functions
> and database queries.**

That's it. Everything below is mechanics in service of that sentence.

Three things DDD asks of us that the current code doesn't do:

1. **Model the domain with behavior, not just data.** Right now `models.Routine` is a bag of
   fields with GORM tags. "Completing a routine" lives in `RoutineService.Complete`. In DDD,
   *the routine knows how to be completed*. Logic moves onto the thing it's about.

2. **Keep the domain ignorant of the database.** Today `RoutineService` holds a `*gorm.DB` and
   calls `s.db.Create(...)` directly. The business logic and the persistence technology are welded
   together. DDD splits them: the domain declares *what* it needs ("save this routine") as an
   interface; a separate infrastructure layer says *how* (GORM + Postgres).

3. **Draw boundaries around each part of the business.** Habits/routines and money/transactions
   are two unrelated subjects that happen to share a database. DDD calls these **bounded
   contexts** and gives each its own folder, its own models, its own language.

### The dependency rule

DDD layers are arranged so that **dependencies only ever point inward**:

```
   transport (HTTP)  ─┐
   application        ─┼──►  domain   ◄── infrastructure (GORM)
                       ┘
```

- **domain** — the center. Knows nothing about HTTP, GORM, Fiber, or Postgres. Pure Go + business rules.
- **application** — orchestrates use cases ("create a routine, then log it"). Depends on domain only.
- **infrastructure** — implements the interfaces the domain declares (the GORM repository). Depends on domain.
- **transport** — translates HTTP ⇆ application calls (the Fiber handlers). Depends on application.

The domain is the one thing nothing is allowed to corrupt. GORM tags, Fiber contexts, JSON tags —
none of that leaks inward. That single rule is what makes a DDD codebase testable and durable.

---

## Where we are today

```
internal/
├── config/         viper loader            (shared)
├── database/       gorm + postgres connect (shared)
├── models/
│   ├── base.go     Base (id, timestamps, soft delete)
│   ├── routine.go  Routine, RoutineLog
│   ├── transaction.go  Transaction, TransactionCategory, TransactionType
│   └── models.go   AllModels() registry
├── services/
│   ├── routine_service.go      business logic + SQL, holds *gorm.DB
│   └── transaction_service.go  business logic + SQL, holds *gorm.DB
├── handlers/
│   ├── routine_handler.go      Fiber, DTOs, validation
│   ├── transaction_handler.go
│   └── validate.go             shared validator
└── routes/
    └── routes.go   wires handlers onto Fiber groups
```

**The smell:** to understand "routines" you open four folders. To add a feature you touch
`models/`, `services/`, `handlers/`, `routes/`. The business concept is shredded across technical
layers. And `Transaction` and `Routine` — which have nothing to do with each other — sit in the
same packages, importable from each other with zero friction.

---

## Where we're going

Group by **bounded context** first, then by **layer** inside each context:

```
internal/
├── platform/                  shared, domain-agnostic plumbing
│   ├── config/                (moved from internal/config)
│   ├── database/              (moved from internal/database)
│   └── httpx/                 shared HTTP helpers (error envelope, id parsing)
│
├── routine/                   ── BOUNDED CONTEXT: habit tracking ──
│   ├── domain/
│   │   ├── routine.go         Routine aggregate (entity + behavior + invariants)
│   │   ├── log.go             RoutineLog entity
│   │   ├── errors.go          ErrRoutineNotFound, ErrDuplicateName, ...
│   │   └── repository.go      Repository interface (the PORT)
│   ├── app/
│   │   └── service.go         use cases: Create, Complete, DailyHistory, ...
│   ├── infra/
│   │   └── gorm_repository.go GORM implementation of domain.Repository
│   └── transport/
│       └── http.go            Fiber handler + DTOs for /api/routines
│
└── finance/                   ── BOUNDED CONTEXT: money ──
    ├── domain/
    │   ├── transaction.go     Transaction aggregate
    │   ├── category.go        Category aggregate
    │   ├── money.go           Money value object
    │   ├── errors.go
    │   └── repository.go      TransactionRepository, CategoryRepository
    ├── app/
    │   └── service.go
    ├── infra/
    │   └── gorm_repository.go
    └── transport/
        └── http.go            /api/transactions
```

Now "routines" is **one folder**. Everything about habit tracking — its rules, its storage, its
HTTP surface — is in `internal/routine/`. Open it and the whole feature is in front of you.

> **Naming the contexts.** I called the money context `finance` rather than `transaction` because
> a bounded context is named after the *area of the business*, not one table. It will grow to hold
> budgets, categories, reports — all "finance". This is the **ubiquitous language** in action:
> name code the way the business talks about it.

---

## Step 1 — Carve out the platform (shared) layer

Start with the easy, mechanical move so the harder steps have a clean base.

`config` and `database` aren't part of any one domain — they're infrastructure every context uses.
Move them under `platform/`:

```bash
mkdir -p internal/platform
git mv internal/config   internal/platform/config
git mv internal/database internal/platform/database
```

Update imports (the module path is `github.com/Bayar101/ray-backend`):

```go
// before
"github.com/Bayar101/ray-backend/internal/config"
"github.com/Bayar101/ray-backend/internal/database"
// after
"github.com/Bayar101/ray-backend/internal/platform/config"
"github.com/Bayar101/ray-backend/internal/platform/database"
```

> **One catch with `database.Connect`.** Today it imports `internal/models` to call
> `db.AutoMigrate(models.AllModels()...)`. That couples shared plumbing to a specific domain's
> models — backwards under the dependency rule. We fix this in Step 6 by having each context
> register its own models. For now leave it; we'll come back.

Build to confirm nothing broke: `go build ./...`.

---

## Step 2 — Build the domain layer: entities with behavior

This is the heart of the migration. We turn anemic data structs into a real domain model.

### Before — anemic model + logic in the service

```go
// internal/models/routine.go
type Routine struct {
    Base
    Name        string `gorm:"not null;uniqueIndex" json:"name"`
    Description string `gorm:"type:text;" json:"description"`
}

// internal/services/routine_service.go — the rule lives HERE, not on the model
func (s *RoutineService) Create(ctx context.Context, name, description string) (models.Routine, error) {
    r := models.Routine{Name: name, Description: description}
    if name == "" {
        return models.Routine{}, fmt.Errorf("name is required")
    }
    ...
}
```

The invariant "a routine must have a name" floats in a service method. Nothing stops some other
code path from creating a nameless `Routine` directly.

### After — the domain enforces its own rules

`internal/routine/domain/routine.go`:

```go
package domain

import "time"

// Routine is the aggregate root for the habit-tracking context.
// It carries no GORM tags and no JSON tags — the domain doesn't know those exist.
type Routine struct {
    id          uint
    name        string
    description string
    createdAt   time.Time
    updatedAt   time.Time
}

// NewRoutine is the only way to construct a valid Routine.
// The invariant lives at the door: you cannot build an invalid one.
func NewRoutine(name, description string) (*Routine, error) {
    if name == "" {
        return nil, ErrNameRequired
    }
    if len(name) > 100 {
        return nil, ErrNameTooLong
    }
    return &Routine{name: name, description: description}, nil
}

// Rename is behavior on the entity, expressed in the ubiquitous language.
func (r *Routine) Rename(name string) error {
    if name == "" {
        return ErrNameRequired
    }
    r.name = name
    return nil
}

// Accessors — fields stay unexported so the invariants can't be bypassed.
func (r *Routine) ID() uint            { return r.id }
func (r *Routine) Name() string        { return r.name }
func (r *Routine) Description() string { return r.description }

// Hydrate rebuilds an entity from ALREADY-VALID stored data (used only by the
// repository). It skips NewRoutine's checks — the row in the DB was valid when it
// was written. This is the "rehydration" factory the infra layer needs.
func Hydrate(id uint, name, description string) *Routine {
    return &Routine{id: id, name: name, description: description}
}
```

The `RoutineLog` entity and its completion behavior live in the same package
(`internal/routine/domain/log.go` or alongside `routine.go`):

```go
type RoutineLog struct {
    id          uint
    routineID   uint
    completedAt time.Time
}

// LogCompletion is the domain expressing what "completing a routine" produces.
func LogCompletion(r *Routine) *RoutineLog {
    return &RoutineLog{routineID: r.ID(), completedAt: time.Now()}
}

// HydrateLog rebuilds a stored log (used by the repository).
func HydrateLog(id, routineID uint, completedAt time.Time) *RoutineLog {
    return &RoutineLog{id: id, routineID: routineID, completedAt: completedAt}
}

func (l *RoutineLog) ID() uint               { return l.id }
func (l *RoutineLog) RoutineID() uint        { return l.routineID }
func (l *RoutineLog) CompletedAt() time.Time { return l.completedAt }
```

> **Why `Hydrate` AND `NewRoutine`?** `NewRoutine` is for *new* routines from user input — it
> validates. `Hydrate` is for routines coming *back from the database* — already valid, and it
> must be able to set the `id` (which `NewRoutine` never does). Two constructors, two jobs.

`internal/routine/domain/errors.go`:

```go
package domain

import "errors"

var (
    ErrNameRequired    = errors.New("routine name is required")
    ErrNameTooLong     = errors.New("routine name too long")
    ErrRoutineNotFound = errors.New("routine not found")
    ErrDuplicateName   = errors.New("routine name already exists")
)
```

> **Why unexported fields?** If `name` is public, any package can write `r.Name = ""` and break the
> invariant `NewRoutine` worked to guarantee. Hiding fields behind constructors and methods is how
> the entity *stays* valid for its whole life. This is the single biggest behavioral change from
> the anemic style.

> **Note on the trade-off:** unexported fields mean GORM can't scan straight into the domain
> struct, and `c.JSON(routine)` can't serialize it. That's not a bug — it's the boundary doing its
> job. Step 4 (infrastructure) and Step 5 (transport) each keep their own struct and translate.
> If that feels like too much ceremony for a small project, see the "Pragmatic dial" note at the end.

---

## Step 3 — Value objects: model concepts, not primitives

A **value object** is a small immutable type defined by its value, with no identity. It replaces a
naked primitive that's secretly carrying rules.

The finance context has a perfect candidate: `Amount int64`. An amount of money isn't just a
number — it can't be negative, it has a currency, it knows how to add. And `TransactionType` is
already half a value object (it's a `string` enum) but nothing validates it.

### Before

```go
// internal/models/transaction.go
type Transaction struct {
    Base
    Amount int64           `gorm:"not null" json:"amount"`
    Type   TransactionType `gorm:"not null;index;type:varchar(20)" json:"type"`
    ...
}
```

Validation that `Type` is `income`/`expense` lives in a *handler* tag
(`validate:"oneof=income expense"`) — i.e. the rule is enforced at the HTTP edge, and the domain
trusts whatever it's handed.

### After — `internal/finance/domain/money.go`

```go
package domain

import "errors"

var ErrNegativeAmount = errors.New("amount cannot be negative")

// Money is a value object: immutable, no identity, compared by value.
// Stored as integer minor units (cents) to dodge float rounding.
type Money struct {
    cents int64
}

func NewMoney(cents int64) (Money, error) {
    if cents < 0 {
        return Money{}, ErrNegativeAmount
    }
    return Money{cents: cents}, nil
}

func (m Money) Cents() int64 { return m.cents }
func (m Money) Add(o Money) Money { return Money{cents: m.cents + o.cents} }
```

And `TransactionType` becomes a self-validating value object:

```go
type TransactionType string

const (
    Income  TransactionType = "income"
    Expense TransactionType = "expense"
)

func (t TransactionType) Valid() bool {
    return t == Income || t == Expense
}
```

Now the rule "type must be income or expense" lives in the domain and is true *everywhere* —
not only when a request happens to pass through the validator. The handler tag becomes a
convenience (fail fast with a nice 400), not the sole guardian.

---

## Step 4 — Repositories: invert the database dependency

This is the structural change that frees the domain from GORM.

### Before — service is welded to GORM

```go
// internal/services/routine_service.go
type RoutineService struct {
    db *gorm.DB   // <-- domain logic holds a database handle
}

func (s *RoutineService) Get(ctx context.Context, id uint) (models.Routine, error) {
    var routine models.Routine
    if err := s.db.WithContext(ctx).First(&routine, id).Error; err != nil {
        return models.Routine{}, fmt.Errorf("failed to get routine: %w", err)
    }
    return routine, nil
}
```

You can't test this without a database, and `gorm.ErrRecordNotFound` leaks all the way out.

### After — the domain declares a port

`internal/routine/domain/repository.go`:

```go
package domain

import (
    "context"
    "time"
)

// DailyEntry is a READ MODEL (a query projection), not an aggregate. Query-side
// results are allowed to be plain data — they never get mutated or persisted, so
// they don't need behavior or unexported fields. Field names map to the SELECT
// columns by GORM's snake_case convention.
type DailyEntry struct {
    RoutineID   uint   `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Completed   bool   `json:"completed"`
}

// Repository is a PORT: the domain says WHAT it needs, not HOW.
// No gorm, no sql — just the domain's own types.
type Repository interface {
    Save(ctx context.Context, r *Routine) error
    FindByID(ctx context.Context, id uint) (*Routine, error)
    FindAll(ctx context.Context) ([]*Routine, error)
    Delete(ctx context.Context, id uint) error

    AddLog(ctx context.Context, log *RoutineLog) error
    DailyHistory(ctx context.Context, day time.Time) ([]DailyEntry, error)
}
```

The infrastructure layer provides the adapter — **the full file**, so `GormRepository` actually
satisfies the interface above. `internal/routine/infra/gorm_repository.go`:

```go
package infra

import (
    "context"
    "errors"
    "time"

    "github.com/Bayar101/ray-backend/internal/routine/domain"
    "gorm.io/gorm"
)

// ---- persistence models: GORM tags live HERE, never on the domain entity ----

type routineRecord struct {
    ID          uint           `gorm:"primaryKey"`
    Name        string         `gorm:"not null;uniqueIndex"`
    Description string         `gorm:"type:text"`
    CreatedAt   time.Time      `gorm:"autoCreateTime"`
    UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
    DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (routineRecord) TableName() string { return "routines" }

type routineLogRecord struct {
    ID          uint           `gorm:"primaryKey"`
    RoutineID   uint           `gorm:"not null;index"`
    CompletedAt time.Time      `gorm:"autoCreateTime"`
    CreatedAt   time.Time      `gorm:"autoCreateTime"`
    UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
    DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (routineLogRecord) TableName() string { return "routine_logs" }

// Models is what the composition root registers for AutoMigrate (see Step 7).
func Models() []any { return []any{&routineRecord{}, &routineLogRecord{}} }

// ---- mappers: the translation between the two shapes ----

func toDomain(rec routineRecord) *domain.Routine {
    return domain.Hydrate(rec.ID, rec.Name, rec.Description)
}

func toRecord(r *domain.Routine) routineRecord {
    return routineRecord{ID: r.ID(), Name: r.Name(), Description: r.Description()}
}

// ---- the repository ----

type GormRepository struct{ db *gorm.DB }

func NewGormRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) Save(ctx context.Context, ro *domain.Routine) error {
    rec := toRecord(ro)
    if err := r.db.WithContext(ctx).Save(&rec).Error; err != nil {
        return err
    }
    *ro = *toDomain(rec) // copy back the DB-generated ID into the caller's entity
    return nil
}

func (r *GormRepository) FindByID(ctx context.Context, id uint) (*domain.Routine, error) {
    var rec routineRecord
    if err := r.db.WithContext(ctx).First(&rec, id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, domain.ErrRoutineNotFound // translate infra error to DOMAIN error
        }
        return nil, err
    }
    return toDomain(rec), nil
}

func (r *GormRepository) FindAll(ctx context.Context) ([]*domain.Routine, error) {
    var recs []routineRecord
    if err := r.db.WithContext(ctx).Find(&recs).Error; err != nil {
        return nil, err
    }
    out := make([]*domain.Routine, len(recs))
    for i, rec := range recs {
        out[i] = toDomain(rec)
    }
    return out, nil
}

func (r *GormRepository) Delete(ctx context.Context, id uint) error {
    res := r.db.WithContext(ctx).Delete(&routineRecord{}, id)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return domain.ErrRoutineNotFound
    }
    return nil
}

func (r *GormRepository) AddLog(ctx context.Context, log *domain.RoutineLog) error {
    rec := routineLogRecord{RoutineID: log.RoutineID(), CompletedAt: log.CompletedAt()}
    if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
        return err
    }
    *log = *domain.HydrateLog(rec.ID, rec.RoutineID, rec.CompletedAt)
    return nil
}

func (r *GormRepository) DailyHistory(ctx context.Context, day time.Time) ([]domain.DailyEntry, error) {
    start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
    end := start.Add(24 * time.Hour)

    var entries []domain.DailyEntry
    err := r.db.WithContext(ctx).
        Model(&routineRecord{}).
        Select("routines.id AS routine_id, routines.name, routines.description, COUNT(routine_logs.id) > 0 AS completed").
        Joins(`LEFT JOIN routine_logs
            ON routine_logs.routine_id = routines.id
            AND routine_logs.completed_at >= ?
            AND routine_logs.completed_at < ?
            AND routine_logs.deleted_at IS NULL`, start, end).
        Group("routines.id").
        Scan(&entries).Error
    if err != nil {
        return nil, err
    }
    return entries, nil
}
```

Two payoffs:

- **`domain.ErrRoutineNotFound` replaces `gorm.ErrRecordNotFound`.** The leak is plugged at the
  one place that knows about GORM. Everything above just checks the domain error.
- **The app layer depends on `domain.Repository`, an interface.** In tests you pass a fake
  in-memory implementation — no SQLite, no Docker. The GORM version is just one adapter.

> **The `*ro = *toDomain(rec)` trick.** `Save` must return the DB-generated `id` to the caller,
> but the domain's `id` field is unexported — infra (a different package) can't write it directly.
> The fix: build a fresh entity via `Hydrate` (which *can* set `id`, same package as the field) and
> copy the whole struct over the caller's pointer. Struct assignment copies every field, exported or
> not. This is why the `Hydrate` factory from Step 2 isn't optional — `Save` and `AddLog` both need it.

---

## Step 5 — Application layer: thin use-case orchestration

The application service replaces the old fat service. It coordinates the domain and the repository
but contains no business rules itself and no SQL.

`internal/routine/app/service.go`:

`internal/routine/app/service.go` — **every** use case the handler will call, so the transport
layer in Step 6 has a method for each route:

```go
package app

import (
    "context"
    "time"

    "github.com/Bayar101/ray-backend/internal/routine/domain"
)

type Service struct {
    repo domain.Repository // depends on the PORT, not *gorm.DB
}

func NewService(repo domain.Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, name, description string) (*domain.Routine, error) {
    r, err := domain.NewRoutine(name, description) // domain enforces the invariant
    if err != nil {
        return nil, err
    }
    if err := s.repo.Save(ctx, r); err != nil { // infra persists it
        return nil, err
    }
    return r, nil
}

func (s *Service) List(ctx context.Context) ([]*domain.Routine, error) {
    return s.repo.FindAll(ctx)
}

func (s *Service) Get(ctx context.Context, id uint) (*domain.Routine, error) {
    return s.repo.FindByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uint, name, description string) (*domain.Routine, error) {
    r, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return nil, err
    }
    if name != "" {
        if err := r.Rename(name); err != nil { // mutate THROUGH the entity's behavior
            return nil, err
        }
    }
    if description != "" {
        r.Describe(description)
    }
    if err := s.repo.Save(ctx, r); err != nil {
        return nil, err
    }
    return r, nil
}

func (s *Service) Delete(ctx context.Context, id uint) error {
    return s.repo.Delete(ctx, id)
}

// Complete is a use case spanning two entities — orchestration belongs here.
func (s *Service) Complete(ctx context.Context, id uint) (*domain.RoutineLog, error) {
    r, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return nil, err // already domain.ErrRoutineNotFound from the repo
    }
    log := domain.LogCompletion(r) // domain decides what "completing" means
    if err := s.repo.AddLog(ctx, log); err != nil {
        return nil, err
    }
    return log, nil
}

func (s *Service) DailyHistory(ctx context.Context, day time.Time) ([]domain.DailyEntry, error) {
    return s.repo.DailyHistory(ctx, day)
}
```

`Update` calls `r.Rename(...)` — that's the partial-update logic from the old
`RoutineService.Update`, but now the *entity* owns the rule. Add the matching `Describe` mutator
to the domain entity (Step 2), since description has no invariant it's a plain setter:

```go
func (r *Routine) Describe(description string) { r.description = description }
```

Compare to the old `RoutineService.Complete` (which did the `First`, built the `RoutineLog`, and
called `Create` all against `s.db`). Same steps — but now "what completing means" is a domain call
and "how it's stored" is behind the repo. The use case reads like the business sentence.

---

## Step 6 — Transport layer: HTTP stays at the edge

The Fiber handler keeps doing exactly what `routine_handler.go` does today — parse, validate, call,
respond — but it calls the **app service** and maps **domain errors** to status codes.

`internal/routine/transport/http.go` — **the whole file**, including `Register` and every route
handler (the old `routine_handler.go` had eight; they all come across):

```go
package transport

import (
    "errors"
    "strconv"
    "time"

    "github.com/Bayar101/ray-backend/internal/routine/app"
    "github.com/Bayar101/ray-backend/internal/routine/domain"
    "github.com/gofiber/fiber/v3"
)

type Handler struct{ svc *app.Service }

func NewHandler(svc *app.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(rg fiber.Router) {
    rg.Post("/create", h.Create)
    rg.Get("/list", h.List)
    rg.Get("/get/:id", h.Get)
    rg.Put("/update/:id", h.Update)
    rg.Delete("/delete/:id", h.Delete)
    rg.Post("/complete/:id", h.Complete)
    rg.Get("/history", h.DailyHistory)
}

// DTOs stay here — JSON tags belong to transport, not the domain.
type createInput struct {
    Name        string `json:"name" validate:"required,min=1,max=100"`
    Description string `json:"description" validate:"max=1000"`
}

type updateInput struct {
    Name        string `json:"name" validate:"omitempty,min=1,max=100"`
    Description string `json:"description" validate:"omitempty,max=1000"`
}

type routineResponse struct {
    ID          uint   `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

func toResponse(r *domain.Routine) routineResponse {
    return routineResponse{ID: r.ID(), Name: r.Name(), Description: r.Description()}
}

func (h *Handler) Create(c fiber.Ctx) error {
    var in createInput
    if err := c.Bind().Body(&in); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
    }
    r, err := h.svc.Create(c.Context(), in.Name, in.Description)
    if err != nil {
        return mapError(c, err)
    }
    return c.Status(fiber.StatusCreated).JSON(toResponse(r))
}

func (h *Handler) List(c fiber.Ctx) error {
    routines, err := h.svc.List(c.Context())
    if err != nil {
        return mapError(c, err)
    }
    out := make([]routineResponse, len(routines))
    for i, r := range routines {
        out[i] = toResponse(r)
    }
    return c.Status(fiber.StatusOK).JSON(out)
}

func (h *Handler) Get(c fiber.Ctx) error {
    id, err := strconv.Atoi(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
    }
    r, err := h.svc.Get(c.Context(), uint(id))
    if err != nil {
        return mapError(c, err)
    }
    return c.Status(fiber.StatusOK).JSON(toResponse(r))
}

func (h *Handler) Update(c fiber.Ctx) error {
    id, err := strconv.Atoi(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
    }
    var in updateInput
    if err := c.Bind().Body(&in); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
    }
    r, err := h.svc.Update(c.Context(), uint(id), in.Name, in.Description)
    if err != nil {
        return mapError(c, err)
    }
    return c.Status(fiber.StatusOK).JSON(toResponse(r))
}

func (h *Handler) Delete(c fiber.Ctx) error {
    id, err := strconv.Atoi(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
    }
    if err := h.svc.Delete(c.Context(), uint(id)); err != nil {
        return mapError(c, err)
    }
    return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "routine deleted"})
}

func (h *Handler) Complete(c fiber.Ctx) error {
    id, err := strconv.Atoi(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
    }
    log, err := h.svc.Complete(c.Context(), uint(id))
    if err != nil {
        return mapError(c, err)
    }
    return c.Status(fiber.StatusCreated).JSON(fiber.Map{
        "id": log.ID(), "routine_id": log.RoutineID(), "completed_at": log.CompletedAt(),
    })
}

func (h *Handler) DailyHistory(c fiber.Ctx) error {
    dateStr := c.Query("date")
    if dateStr == "" {
        dateStr = time.Now().Format("2006-01-02")
    }
    day, err := time.Parse("2006-01-02", dateStr)
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid date"})
    }
    entries, err := h.svc.DailyHistory(c.Context(), day)
    if err != nil {
        return mapError(c, err)
    }
    return c.Status(fiber.StatusOK).JSON(entries)
}

// mapError is the single place domain errors become HTTP status codes.
func mapError(c fiber.Ctx, err error) error {
    switch {
    case errors.Is(err, domain.ErrRoutineNotFound):
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "routine not found"})
    case errors.Is(err, domain.ErrDuplicateName):
        return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "routine name already exists"})
    case errors.Is(err, domain.ErrNameRequired), errors.Is(err, domain.ErrNameTooLong):
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    default:
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
    }
}
```

Notice the old handler scattered `errors.Is(err, gorm.ErrRecordNotFound)` across five methods. Now
there's **one** `mapError`, and it speaks the domain's language, not GORM's. The `default` case also
stops leaking raw `err.Error()` (which today exposes SQL details) — a security win that fell out of
the restructure for free.

> `Register(rg fiber.Router)` keeps the same shape the old handler had — that pattern was already
> clean and survives the migration untouched. The validation tags (`validate:"required,..."`) still
> fire via Fiber's `StructValidator` (wired in `main.go`) before the service is called.

---

## Step 7 — Composition root: wire it in `main.go`

The dependency graph is now explicit and points inward. `main` is the **composition root** — the
one place allowed to know about every layer, where you plug the concrete adapters into the ports.

### Before

```go
svc := services.NewRoutineService(db)
handler := handlers.NewRoutineHandler(svc)
txSvc := services.NewTransactionService(db)
txHandler := handlers.NewTransactionHandler(txSvc)
```

### After

> **This is where your build broke.** `main.go` references `routineinfra`, `routineapp`, etc. —
> but every one of those packages is literally named `package infra` / `package app` /
> `package transport`. Three contexts × three layers would give you six packages all called `app`
> and `infra`, which won't compile. The fix is **named imports (aliases)** — give each import path a
> unique local name. The aliases aren't magic identifiers; they're declared in the import block:

```go
package main

import (
    "log"

    "github.com/Bayar101/ray-backend/internal/platform/config"
    "github.com/Bayar101/ray-backend/internal/platform/database"
    "github.com/Bayar101/ray-backend/internal/routes"

    // routine context — alias each layer so the names in main() resolve
    routineapp "github.com/Bayar101/ray-backend/internal/routine/app"
    routineinfra "github.com/Bayar101/ray-backend/internal/routine/infra"
    routinetransport "github.com/Bayar101/ray-backend/internal/routine/transport"

    // finance context
    financeapp "github.com/Bayar101/ray-backend/internal/finance/app"
    financeinfra "github.com/Bayar101/ray-backend/internal/finance/infra"
    financetransport "github.com/Bayar101/ray-backend/internal/finance/transport"

    "github.com/go-playground/validator/v10"
    "github.com/gofiber/fiber/v3"
)

type structValidator struct{ v *validator.Validate }

func (s structValidator) Validate(out any) error { return s.v.Struct(out) }

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("config load failed: %v", err)
    }

    db, err := database.Connect(cfg.DB)
    if err != nil {
        log.Fatalf("database connection failed: %v", err)
    }

    // routine context
    routineRepo := routineinfra.NewGormRepository(db) // infra adapter (implements domain.Repository)
    routineSvc := routineapp.NewService(routineRepo)  // app layer depends on the interface
    routineHTTP := routinetransport.NewHandler(routineSvc)

    // finance context
    financeRepo := financeinfra.NewGormRepository(db)
    financeSvc := financeapp.NewService(financeRepo)
    financeHTTP := financetransport.NewHandler(financeSvc)

    app := fiber.New(fiber.Config{
        StructValidator: structValidator{v: validator.New()},
    })

    routes.Register(app, routineHTTP, financeHTTP)

    log.Fatal(app.Listen(":" + cfg.App.Port))
}
```

Read the routine block top to bottom: GORM adapter → into the app service (as an interface) →
into the handler. The arrows of the dependency rule, written out as constructor calls.

`routes.Register` changes signature — it now takes the two **transport** handlers instead of the
old `*handlers.RoutineHandler`. `internal/routes/routes.go`:

```go
package routes

import (
    financetransport "github.com/Bayar101/ray-backend/internal/finance/transport"
    routinetransport "github.com/Bayar101/ray-backend/internal/routine/transport"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/cors"
    "github.com/gofiber/fiber/v3/middleware/logger"
    "github.com/gofiber/fiber/v3/middleware/recover"
)

func Register(app *fiber.App, rh *routinetransport.Handler, fh *financetransport.Handler) {
    app.Use(logger.New())
    app.Use(recover.New())
    app.Use(cors.New())

    app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })

    api := app.Group("/api")
    rh.Register(api.Group("/routines"))
    fh.Register(api.Group("/transactions"))
}
```

**The migration registry** (the Step 1 loose end): instead of `models.AllModels()`, each context
exposes the records *it* owns. The cleanest place to gather them is `database.Connect`, but that
would make `platform/database` import both contexts — the backwards dependency we wanted to avoid.
So pass the model list *in* as an argument instead. Change `Connect` to accept it:

```go
// internal/platform/database/database.go
func Connect(cfg config.DB, models ...any) (*gorm.DB, error) {
    db, err := gorm.Open(postgres.Open(cfg.DSN()))
    if err != nil {
        return nil, err
    }
    if err := db.AutoMigrate(models...); err != nil {
        return nil, err
    }
    // ... pool config unchanged
    return db, nil
}
```

Then `main` gathers them — `main` is allowed to import every context, that's its whole job:

```go
db, err := database.Connect(cfg.DB,
    append(routineinfra.Models(), financeinfra.Models()...)...,
)
```

Now `platform/database` imports no domain (delete its old `internal/models` import), and adding a
context means adding one `Models()` call here — not editing a central `models.go`.

---

## Step 8 — The finance context, in full

This is the context most likely still missing from your tree — you may have written
`finance/domain/` but not `app/`, `infra/`, or `transport/`. Here are all four layers. The shape
mirrors the routine context exactly; the differences are called out as you go.

### `internal/finance/domain/` — entities, value object, errors, ports

`money.go` (the value object from Step 3) and `transaction.go` you already have. Note the
**ubiquitous-language constant is `Expense`, not `Expence`** — fix that typo or every comparison
silently breaks. The full entity keeps `categoryID` (reference another aggregate **by identity**,
never embed it):

```go
// transaction.go
package domain

import "time"

type TransactionType string

const (
    Income  TransactionType = "income"
    Expense TransactionType = "expense"
)

func (t TransactionType) Valid() bool { return t == Income || t == Expense }

type Transaction struct {
    id         uint
    categoryID uint
    amount     Money
    txType     TransactionType
    note       string
    date       time.Time
}

func NewTransaction(categoryID uint, amount Money, txType TransactionType, note string, date time.Time) (*Transaction, error) {
    if categoryID == 0 {
        return nil, ErrCategoryRequired
    }
    if !txType.Valid() {
        return nil, ErrInvalidType
    }
    return &Transaction{categoryID: categoryID, amount: amount, txType: txType, note: note, date: date}, nil
}

func HydrateTransaction(id, categoryID uint, amount Money, txType TransactionType, note string, date time.Time) *Transaction {
    return &Transaction{id: id, categoryID: categoryID, amount: amount, txType: txType, note: note, date: date}
}

func (t *Transaction) ID() uint            { return t.id }
func (t *Transaction) CategoryID() uint    { return t.categoryID }
func (t *Transaction) Amount() Money       { return t.amount }
func (t *Transaction) Type() TransactionType { return t.txType }
func (t *Transaction) Note() string        { return t.note }
func (t *Transaction) Date() time.Time     { return t.date }
```

`category.go` — the second aggregate, its own root:

```go
package domain

type Category struct {
    id   uint
    name string
}

func NewCategory(name string) (*Category, error) {
    if name == "" {
        return nil, ErrCategoryNameRequired
    }
    return &Category{name: name}, nil
}

func HydrateCategory(id uint, name string) *Category { return &Category{id: id, name: name} }

func (c *Category) ID() uint     { return c.id }
func (c *Category) Name() string { return c.name }
func (c *Category) Rename(name string) error {
    if name == "" {
        return ErrCategoryNameRequired
    }
    c.name = name
    return nil
}
```

`errors.go`:

```go
package domain

import "errors"

var (
    ErrCategoryRequired     = errors.New("category is required")
    ErrInvalidType          = errors.New("transaction type must be income or expense")
    ErrTransactionNotFound  = errors.New("transaction not found")
    ErrCategoryNameRequired = errors.New("category name is required")
    ErrCategoryNotFound     = errors.New("category not found")
    ErrDuplicateCategory    = errors.New("category name already exists")
)
```

`repository.go` — **two** ports, because there are two aggregates:

```go
package domain

import "context"

type TransactionRepository interface {
    Save(ctx context.Context, t *Transaction) error
    SaveMany(ctx context.Context, ts []*Transaction) error // bulk, all-or-nothing
    FindByID(ctx context.Context, id uint) (*Transaction, error)
    FindAll(ctx context.Context) ([]*Transaction, error)
    Delete(ctx context.Context, id uint) error
}

type CategoryRepository interface {
    Save(ctx context.Context, c *Category) error
    FindByID(ctx context.Context, id uint) (*Category, error)
    FindAll(ctx context.Context) ([]*Category, error)
    Delete(ctx context.Context, id uint) error
}
```

### `internal/finance/infra/gorm_repository.go` — adapters for both ports

```go
package infra

import (
    "context"
    "errors"
    "time"

    "github.com/Bayar101/ray-backend/internal/finance/domain"
    "github.com/jackc/pgx/v5/pgconn"
    "gorm.io/gorm"
)

type transactionRecord struct {
    ID         uint           `gorm:"primaryKey"`
    CategoryID uint           `gorm:"not null;index"`
    Amount     int64          `gorm:"not null"` // Money stored as cents
    Type       string         `gorm:"not null;index;type:varchar(20)"`
    Note       string         `gorm:"type:text"`
    Date       time.Time      `gorm:"not null;index"`
    CreatedAt  time.Time      `gorm:"autoCreateTime"`
    UpdatedAt  time.Time      `gorm:"autoUpdateTime"`
    DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func (transactionRecord) TableName() string { return "transactions" }

type categoryRecord struct {
    ID        uint           `gorm:"primaryKey"`
    Name      string         `gorm:"not null;uniqueIndex"`
    CreatedAt time.Time      `gorm:"autoCreateTime"`
    UpdatedAt time.Time      `gorm:"autoUpdateTime"`
    DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (categoryRecord) TableName() string { return "transaction_categories" }

func Models() []any { return []any{&categoryRecord{}, &transactionRecord{}} } // category first (FK target)

// ---- mappers (Money <-> int64 cents) ----

func txToDomain(rec transactionRecord) *domain.Transaction {
    amount, _ := domain.NewMoney(rec.Amount) // stored value already valid
    return domain.HydrateTransaction(rec.ID, rec.CategoryID, amount,
        domain.TransactionType(rec.Type), rec.Note, rec.Date)
}

func txToRecord(t *domain.Transaction) transactionRecord {
    return transactionRecord{
        ID: t.ID(), CategoryID: t.CategoryID(), Amount: t.Amount().Cents(),
        Type: string(t.Type()), Note: t.Note(), Date: t.Date(),
    }
}

func catToDomain(rec categoryRecord) *domain.Category { return domain.HydrateCategory(rec.ID, rec.Name) }
func catToRecord(c *domain.Category) categoryRecord   { return categoryRecord{ID: c.ID(), Name: c.Name()} }

// ---- TransactionRepository ----

type TransactionGormRepository struct{ db *gorm.DB }

func NewTransactionGormRepository(db *gorm.DB) *TransactionGormRepository {
    return &TransactionGormRepository{db: db}
}

func (r *TransactionGormRepository) Save(ctx context.Context, t *domain.Transaction) error {
    rec := txToRecord(t)
    if err := r.db.WithContext(ctx).Save(&rec).Error; err != nil {
        return err
    }
    *t = *txToDomain(rec)
    return nil
}

// SaveMany wraps the batch in one DB transaction — partial failure rolls back.
func (r *TransactionGormRepository) SaveMany(ctx context.Context, ts []*domain.Transaction) error {
    recs := make([]transactionRecord, len(ts))
    for i, t := range ts {
        recs[i] = txToRecord(t)
    }
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := tx.Create(&recs).Error; err != nil {
            return err
        }
        for i := range ts {
            *ts[i] = *txToDomain(recs[i])
        }
        return nil
    })
}

func (r *TransactionGormRepository) FindByID(ctx context.Context, id uint) (*domain.Transaction, error) {
    var rec transactionRecord
    if err := r.db.WithContext(ctx).First(&rec, id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, domain.ErrTransactionNotFound
        }
        return nil, err
    }
    return txToDomain(rec), nil
}

func (r *TransactionGormRepository) FindAll(ctx context.Context) ([]*domain.Transaction, error) {
    var recs []transactionRecord
    if err := r.db.WithContext(ctx).Find(&recs).Error; err != nil {
        return nil, err
    }
    out := make([]*domain.Transaction, len(recs))
    for i, rec := range recs {
        out[i] = txToDomain(rec)
    }
    return out, nil
}

func (r *TransactionGormRepository) Delete(ctx context.Context, id uint) error {
    res := r.db.WithContext(ctx).Delete(&transactionRecord{}, id)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return domain.ErrTransactionNotFound
    }
    return nil
}

// ---- CategoryRepository ----

type CategoryGormRepository struct{ db *gorm.DB }

func NewCategoryGormRepository(db *gorm.DB) *CategoryGormRepository {
    return &CategoryGormRepository{db: db}
}

func (r *CategoryGormRepository) Save(ctx context.Context, c *domain.Category) error {
    rec := catToRecord(c)
    if err := r.db.WithContext(ctx).Save(&rec).Error; err != nil {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique violation
            return domain.ErrDuplicateCategory
        }
        return err
    }
    *c = *catToDomain(rec)
    return nil
}

func (r *CategoryGormRepository) FindByID(ctx context.Context, id uint) (*domain.Category, error) {
    var rec categoryRecord
    if err := r.db.WithContext(ctx).First(&rec, id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, domain.ErrCategoryNotFound
        }
        return nil, err
    }
    return catToDomain(rec), nil
}

func (r *CategoryGormRepository) FindAll(ctx context.Context) ([]*domain.Category, error) {
    var recs []categoryRecord
    if err := r.db.WithContext(ctx).Find(&recs).Error; err != nil {
        return nil, err
    }
    out := make([]*domain.Category, len(recs))
    for i, rec := range recs {
        out[i] = catToDomain(rec)
    }
    return out, nil
}

func (r *CategoryGormRepository) Delete(ctx context.Context, id uint) error {
    res := r.db.WithContext(ctx).Delete(&categoryRecord{}, id)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return domain.ErrCategoryNotFound
    }
    return nil
}
```

> Importing `pgconn` for the `23505` translation? Run `go mod tidy` — it's currently an indirect
> dependency and the import promotes it to direct.

### `internal/finance/app/service.go` — one service holding both ports

```go
package app

import (
    "context"

    "github.com/Bayar101/ray-backend/internal/finance/domain"
)

type Service struct {
    tx  domain.TransactionRepository
    cat domain.CategoryRepository
}

func NewService(tx domain.TransactionRepository, cat domain.CategoryRepository) *Service {
    return &Service{tx: tx, cat: cat}
}

func (s *Service) Create(ctx context.Context, categoryID uint, cents int64, txType domain.TransactionType, note string, date time.Time) (*domain.Transaction, error) {
    amount, err := domain.NewMoney(cents)
    if err != nil {
        return nil, err
    }
    t, err := domain.NewTransaction(categoryID, amount, txType, note, date)
    if err != nil {
        return nil, err
    }
    if err := s.tx.Save(ctx, t); err != nil {
        return nil, err
    }
    return t, nil
}

func (s *Service) List(ctx context.Context) ([]*domain.Transaction, error) { return s.tx.FindAll(ctx) }
func (s *Service) Get(ctx context.Context, id uint) (*domain.Transaction, error) {
    return s.tx.FindByID(ctx, id)
}
func (s *Service) Delete(ctx context.Context, id uint) error { return s.tx.Delete(ctx, id) }

func (s *Service) CreateCategory(ctx context.Context, name string) (*domain.Category, error) {
    c, err := domain.NewCategory(name)
    if err != nil {
        return nil, err
    }
    if err := s.cat.Save(ctx, c); err != nil {
        return nil, err
    }
    return c, nil
}

func (s *Service) ListCategories(ctx context.Context) ([]*domain.Category, error) {
    return s.cat.FindAll(ctx)
}
```

> Add `import "time"` to the app file — `Create` takes a `time.Time`. (Add `Update`, `BulkCreate`,
> `GetCategory`, `UpdateCategory`, `DeleteCategory` the same way; they mirror the routine context.)

### `internal/finance/transport/http.go` — handler + DTOs

Same shape as the routine handler: DTOs with `validate` tags, a `toResponse`, one `mapError` that
translates `domain.ErrTransactionNotFound` → 404, `domain.ErrDuplicateCategory` → 409,
`domain.ErrInvalidType`/`ErrCategoryRequired` → 400. The constructor is
`NewHandler(svc *app.Service) *Handler` and `Register` mounts the same routes the old
`transaction_handler.go` had. The one new mapping detail:

```go
func mapError(c fiber.Ctx, err error) error {
    switch {
    case errors.Is(err, domain.ErrTransactionNotFound), errors.Is(err, domain.ErrCategoryNotFound):
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
    case errors.Is(err, domain.ErrDuplicateCategory):
        return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
    case errors.Is(err, domain.ErrInvalidType), errors.Is(err, domain.ErrCategoryRequired),
        errors.Is(err, domain.ErrCategoryNameRequired), errors.Is(err, domain.ErrNegativeAmount):
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    default:
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
    }
}
```

Wire it in `main.go` — finance has **two** repos, so its constructor takes both:

```go
txRepo  := financeinfra.NewTransactionGormRepository(db)
catRepo := financeinfra.NewCategoryGormRepository(db)
financeSvc  := financeapp.NewService(txRepo, catRepo)
financeHTTP := financetransport.NewHandler(financeSvc)
```

### Why this differs from the routine context

- **Two aggregates, two repositories.** `Transaction` references `Category` **by `categoryID`, not
  by embedding** — that's why the old `Category TransactionCategory` association field is gone.
  Need the category name in a response? The app layer fetches both and composes a DTO.
- **`Money` replaces `Amount int64`.** The record column stays `int64` cents; the mappers convert
  with `Money.Cents()` and `NewMoney()`.
- **`BulkCreate` → `SaveMany`,** wrapped in `db.Transaction(...)` so a partial batch rolls back.
  All-or-nothing is what the app layer asks for; the *how* (a DB transaction) is infra's business.
- **Duplicate category name → `domain.ErrDuplicateCategory` → 409.** The DB unique index is the
  race-safe guard; infra catches Postgres `23505` and translates it. The handler's old
  `validate:"unique"` tag never actually did this — the repository is the correct place.

---

## Migration order (do it incrementally, keep it green)

Don't big-bang this. One context at a time, building after each step:

1. **Step 1** — move `config`/`database` to `platform/`. `go build ./...`. Commit.
2. **Routine context, vertically:**
   a. Create `routine/domain/` (entity, value objects, errors, repository interface).
   b. Create `routine/infra/` (GORM repo + record + mappers). `go build ./...`.
   c. Create `routine/app/` (use cases calling the repo interface).
   d. Create `routine/transport/` (handler + DTOs + `mapError`).
   e. Rewire `main.go` for routines; delete old `models/routine.go`, `services/routine_service.go`,
      `handlers/routine_handler.go`. `go build ./... && go test ./...`. Commit.
3. **Finance context** — repeat 2a–2e. Commit.
4. **Delete the now-empty `internal/models`, `internal/services`, `internal/handlers`.** Move the
   shared validator into `platform/httpx`. Final `go build ./... && go test ./...`. Commit.

Keeping the build green between contexts means you can stop and ship at any point — the routine
context can be fully DDD while finance is still legacy, because they don't import each other.

---

## Testing got easier (the proof DDD worked)

The old `routines_test.go` needed an in-memory SQLite DB (`gorm.Open(sqlite...)`) just to test
`Create`. With the repository interface, the **domain and app layers test with no database at all**:

```go
// a fake repo — 20 lines, no SQLite, no Docker
type fakeRepo struct{ store map[uint]*domain.Routine; seq uint }

func (f *fakeRepo) Save(_ context.Context, r *domain.Routine) error { /* assign id, store */ }
func (f *fakeRepo) FindByID(_ context.Context, id uint) (*domain.Routine, error) {
    if r, ok := f.store[id]; ok { return r, nil }
    return nil, domain.ErrRoutineNotFound
}

func TestComplete_UnknownRoutine(t *testing.T) {
    svc := app.NewService(&fakeRepo{store: map[uint]*domain.Routine{}})
    _, err := svc.Complete(context.Background(), 999)
    if !errors.Is(err, domain.ErrRoutineNotFound) {
        t.Fatalf("want ErrRoutineNotFound, got %v", err)
    }
}
```

Invariant tests are even simpler — pure functions, no service, no repo:

```go
func TestNewRoutine_RejectsEmptyName(t *testing.T) {
    if _, err := domain.NewRoutine("", ""); !errors.Is(err, domain.ErrNameRequired) {
        t.Fatalf("want ErrNameRequired, got %v", err)
    }
}
```

Keep one **integration** test per context that runs the real GORM repo against Postgres
(testcontainers) — that's where the `23505` duplicate-name path and the `DailyHistory` JOIN must be
verified, since a fake repo can't catch SQL bugs.

---

## Concept map: old → new

| Today | DDD home | Why it moved |
|-------|----------|--------------|
| `models.Routine` (struct + GORM/JSON tags) | `routine/domain/routine.go` (behavior) **+** `routine/infra` record **+** `transport` DTO | One struct was doing three jobs (business, storage, wire). Split by responsibility. |
| `RoutineService` holding `*gorm.DB` | `routine/app/service.go` (orchestration) **+** `routine/infra/gorm_repository.go` (SQL) | Business logic must not depend on the DB driver. |
| `name == ""` check inside service | `domain.NewRoutine` constructor | Invariants belong on the entity, enforced at construction. |
| `Amount int64`, validator tag on `Type` | `finance/domain/money.go`, `TransactionType.Valid()` | Primitives hiding rules → value objects that own the rules. |
| `gorm.ErrRecordNotFound` checked in handlers | `domain.ErrRoutineNotFound`, mapped once in `mapError` | Stop leaking the persistence technology upward. |
| `models.AllModels()` central registry | each `infra.Models()`, gathered in composition root | Shared code must not import domains; contexts own their tables. |
| `internal/config`, `internal/database` | `internal/platform/...` | Domain-agnostic plumbing, shared by all contexts. |

---

## The pragmatic dial (don't over-engineer a side project)

Full DDD has real ceremony: three structs per concept (domain/record/DTO), mapping functions,
rehydration factories. For a learning project that's a lot. Dials you can turn *down* without
losing the spirit:

- **Skip separate DTOs at first.** Let transport serialize the domain entity via small accessor
  methods or a single `ToResponse()`. Add DTOs when the wire shape and domain shape actually diverge.
- **Reuse one struct for domain + record** if you can tolerate GORM tags on the entity — you keep
  the *repository interface* (the valuable part: DB independence + testability) without the
  mapping boilerplate. You lose strict invariant protection; that's the trade.
- **Don't build value objects for primitives that carry no rules.** `Description` is just a string.
  Only `Amount` and `Type` earned the `Money`/`TransactionType` treatment.

The non-negotiables — the things that *are* DDD and pay for themselves even here:

1. **Group by bounded context** (`routine/`, `finance/`), not by technical layer.
2. **A repository interface** so the domain doesn't import GORM and tests don't need a database.
3. **Domain errors** instead of leaking `gorm.ErrRecordNotFound`.
4. **Invariants live with the data** they constrain.

Start with those four. Add the rest when the project's size justifies it.

---

## Migration status — DONE (verified 2026-06-29)

Steps 1–8 complete. `go build ./...` clean, `go test ./...` passes. Final tree:

```
internal/
├── platform/   config, database, httpx       ✅
├── routine/    domain, app, infra, transport  ✅
└── finance/    domain, app, infra, transport  ✅
```

- Old `internal/{config,database,models,services,handlers}` deleted ✅
- `database.Connect(cfg, models...)` takes model list; `main` gathers via `Models()` ✅
- finance fully built: transaction + category CRUD, `BulkCreate`/`SaveMany`, `23505` → `ErrDuplicateCategory` ✅
- `go mod tidy` run — `pgconn` resolved ✅

---

## Next steps

### 1. Commit the migration (do this first)
Build is green. Stage and commit before adding anything new — keeps the refactor isolated.
```bash
git add -A && git commit   # message: "refactor: migrate to DDD bounded contexts (routine, finance)"
```

### 2. Replace the SQLite test with DB-free unit tests
`internal/routes/routines_test.go` still spins up in-memory SQLite to test `app.Service.Create`.
The whole point of the repo interface (Step 4) is that app/domain test with **no DB**:
- Add `internal/routine/app/service_test.go` with a `fakeRepo` (the ~20-line stub from the
  "Testing got easier" section) — covers `Create`, `Complete` (unknown id → `ErrRoutineNotFound`).
- Add `internal/routine/domain/routine_test.go` — pure invariant tests (`NewRoutine("")` →
  `ErrNameRequired`, name-too-long). No service, no repo.
- Mirror for finance: `domain.NewMoney(-1)` → `ErrNegativeAmount`, `TransactionType.Valid()`,
  `NewTransaction` with `categoryID==0`.
- Then delete/relocate the SQLite `routines_test.go` (or downgrade it to an integration test, below).

### 3. One integration test per context (real GORM + Postgres)
Fakes can't catch SQL bugs. Use testcontainers-go (or a disposable Postgres) to verify the paths
that only exist in infra:
- routine: `DailyHistory` JOIN returns correct `completed` flag across the day boundary.
- finance: duplicate category name actually hits `23505` → `ErrDuplicateCategory`;
  `SaveMany` rolls back fully on a partial-batch failure.

### 4. Loose ends to confirm
- `platform/httpx/validate.go` — confirm both transport layers use the shared validator and
  `main.go` wires `StructValidator`.
- Grep for any lingering `gorm.ErrRecordNotFound` outside `infra/` — should be zero.
- Decide whether `routes_test.go` belongs in `routes` or moves next to each context.
