# ray-backend — fix-ups, then next steps

The DDD migration (Steps 1–8) is done and the tests have been written. But `go build ./...` being
clean is hiding a red test suite: `go test ./...` currently **fails**. This doc:

1. **Part A** — fixes everything broken or missing right now, with the exact file, line, and a
   before/after for each.
2. **Part B** — the actual next steps to push the project forward, each with a worked example.

> A note on what changed since the migration guide. The code took the **pragmatic dial**: there is
> **no `Money` value object** — `Transaction.amount` is a plain `int64`, and `money.go` was deleted.
> The finance errors/constructors are also named for the ubiquitous language: `NewTransactionCategory`,
> `ErrTransactionCategoryRequired`, `ErrDuplicateTransactionCategory` (not `NewCategory` /
> `ErrCategoryRequired`). Any older example mentioning `Money`/`NewMoney`/`NewCategory` is obsolete —
> the snippets below match the code that's actually in the repo.

---

## Current state (verified 2026-06-30)

```text
go build ./...   ✅ clean
go test ./...    ❌ FAILS:
  finance/app    — TestCreate_RejectsInvalidCategoryID, TestCreate_RejectsInvalidDate
  finance/domain — TestTransaction_SetDate_RejectsZero
  */infra        — all integration tests error (bad Docker image tag)
routine/app, routine/domain   ✅ pass
```

Four distinct problems sit behind those failures. Fix them in order.

---

## Part A — Fix what's broken or missing

### A1. `NewTransaction` is missing the date invariant (real code bug)

`SetDate` rejects a zero date, but the **constructor does not** — so `Create(..., time.Time{})`
sails through and `TestCreate_RejectsInvalidDate` fails (`want ErrInvalidDate, got <nil>`). An
invariant must hold at construction, not only on later mutation.

**File:** `internal/finance/domain/transaction.go`, in `NewTransaction` (after the note check, ~line 37).

```go
// before — date is never validated
	if len(note) > 1000 {
		return nil, ErrNoteTooLong
	}
	return &Transaction{ ... }, nil

// after — same guard SetDate already uses
	if len(note) > 1000 {
		return nil, ErrNoteTooLong
	}
	if date.IsZero() {
		return nil, ErrInvalidDate
	}
	return &Transaction{ ... }, nil
```

`ErrInvalidDate` already exists in `errors.go`, and every passing test calls `NewTransaction` with
`time.Now()`, so this is safe — only the zero-date case flips from `nil` to the error the test wants.

### A2. `TestCreate_RejectsInvalidCategoryID` is mislabeled (test bug)

The test passes `categoryID = 0` and expects `ErrTransactionCategoryNotFound`, but `0` means
"no category was given" → `ErrTransactionCategoryRequired`. It can't be `NotFound`: the other two
Create tests (`TestCreate_PersistsAndAssignsID`, `TestCreate_RejectsInvalidAmount`) pass
`categoryID = 1` against an **empty** fake category repo and expect success — i.e. the suite assumes
`Create` does **not** look the category up. So `0` is a *required* failure, not a *not-found* one.

**File:** `internal/finance/app/service_test.go`, `TestCreate_RejectsInvalidCategoryID` (~line 143).

```go
// before
	if _, err := svc.Create(context.Background(), 0, 100, domain.Income, "test", time.Now()); !errors.Is(err, domain.ErrTransactionCategoryNotFound) {
		t.Fatalf("want ErrTransactionCategoryNotFound, got %v", err)
	}

// after
	if _, err := svc.Create(context.Background(), 0, 100, domain.Income, "test", time.Now()); !errors.Is(err, domain.ErrTransactionCategoryRequired) {
		t.Fatalf("want ErrTransactionCategoryRequired, got %v", err)
	}
```

> Want a *real* not-found check (transaction referencing a category that doesn't exist)? That's a
> genuine missing feature, but it changes behavior and the other tests — do it deliberately in
> **B2**, not as a quick patch here.

### A3. `TestTransaction_SetDate_RejectsZero` has a broken assertion (test bug)

The production `SetDate` is correct — it rejects the zero time. The test fails on its *second*
assertion: `tx.Date() != time.Now()`. The date was set at construction; `time.Now()` evaluated at
assertion time is a different instant, so this is **always** true and always fires a false failure.
Capture the original date and compare against that.

**File:** `internal/finance/domain/transaction_test.go`, `TestTransaction_SetDate_RejectsZero` (~line 136).

```go
// before
func TestTransaction_SetDate_RejectsZero(t *testing.T) {
	tx, _ := domain.NewTransaction(1, 100, domain.Income, "test note", time.Now())
	if err := tx.SetDate(time.Time{}); !errors.Is(err, domain.ErrInvalidDate) {
		t.Fatalf("want ErrInvalidDate, got %v", err)
	}
	if tx.Date() != time.Now() {            // BUG: time.Now() here ≠ the construction time
		t.Fatalf("date mutated on failed set: %v", tx.Date())
	}
}

// after
func TestTransaction_SetDate_RejectsZero(t *testing.T) {
	original := time.Now()
	tx, _ := domain.NewTransaction(1, 100, domain.Income, "test note", original)
	if err := tx.SetDate(time.Time{}); !errors.Is(err, domain.ErrInvalidDate) {
		t.Fatalf("want ErrInvalidDate, got %v", err)
	}
	if !tx.Date().Equal(original) {         // compare to what we actually set; use .Equal for time
		t.Fatalf("date mutated on failed set: %v", tx.Date())
	}
}
```

### A4. Integration tests use a non-existent Docker image tag

All three integration tests error before they test anything:
`docker.io/library/postgres:16-alphine: not found`. It's a typo — `alphine` → **`alpine`**.

**Files:** `internal/routine/infra/integration_test.go:17` and
`internal/finance/infra/integration_test.go:18`.

```go
// before
	pg, err := postgres.Run(ctx, "postgres:16-alphine",

// after
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
```

Docker is available on this machine, so once the tag is fixed the integration suite runs:

```bash
go test -tags=integration ./...
```

### A5. Verify green before moving on

After A1–A4, confirm both the unit and integration suites are actually green — don't claim it,
run it:

```bash
go test ./...                       # unit — must be all ok
go test -tags=integration ./...     # integration — needs Docker running
```

Then commit the fixes on their own so they're isolated from the new work in Part B:

```bash
git add -A && git commit -m "fix: date invariant, mislabeled/broken finance tests, postgres image tag"
```

---

## Part B — Next steps

### B1. Make `make test` real and add CI

The `Makefile` only has an `air` target, and there's no `.github/workflows/`. Right now "the tests
pass" depends on each person remembering the `-tags=integration` flag and having Docker up. Encode
it once.

**`Makefile`:**

```make
.PHONY: air test test-integration build vet

air:
	air

vet:
	go vet ./...

test:                       # fast: unit only, no Docker
	go test ./...

test-integration:           # needs Docker; spins real Postgres via testcontainers
	go test -tags=integration ./...

build:
	go build ./...
```

**`.github/workflows/ci.yml`** — testcontainers needs a Docker daemon, which GitHub's
`ubuntu-latest` runner provides, so the integration job works without a `services:` block:

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: go vet ./...
      - run: make test                 # unit
      - run: make test-integration     # integration (Docker is available on the runner)
```

### B2. Enforce referential integrity in the app layer (the "real" A2)

`Service.Create` trusts whatever `categoryID` it's handed — you can create a transaction pointing at
category `999` that doesn't exist. The fix that A2 deferred: look the category up first and surface
`ErrTransactionCategoryNotFound`. The lookup belongs in **app** (it spans two aggregates), not the
domain.

**`internal/finance/app/service.go`:**

```go
func (s *Service) Create(ctx context.Context, categoryID uint, amount int64, txType domain.TransactionType, note string, date time.Time) (*domain.Transaction, error) {
	if _, err := s.cat.FindByID(ctx, categoryID); err != nil {
		return nil, err // ErrTransactionCategoryNotFound from the repo
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
```

This **changes the test contract**, so update the fakes too: the Create tests must now seed a
category. Example:

```go
func TestCreate_PersistsAndAssignsID(t *testing.T) {
	cat := newFakeCategoryRepo()
	seeded, _ := domain.NewTransactionCategory("Food")
	_ = cat.Save(context.Background(), seeded)        // now category 1 exists

	svc := app.NewService(newFakeRepo(), cat)
	tx, err := svc.Create(context.Background(), seeded.ID(), 100, domain.Income, "test", time.Now())
	// ... unchanged assertions
}

func TestCreate_RejectsUnknownCategory(t *testing.T) {
	svc := app.NewService(newFakeRepo(), newFakeCategoryRepo()) // empty
	if _, err := svc.Create(context.Background(), 999, 100, domain.Income, "test", time.Now()); !errors.Is(err, domain.ErrTransactionCategoryNotFound) {
		t.Fatalf("want ErrTransactionCategoryNotFound, got %v", err)
	}
}
```

Apply the same `FindByID` guard in `Update` when `categoryID != 0`.

### B3. Compose the category name into transaction responses

Today `transactionToResponse` only emits `categoryID` — a client listing transactions has to make a
second call per row to show "Food" instead of `3`. Because `Transaction` references `Category`
**by id** (correct DDD — aggregates don't embed each other), the *app* layer composes the read model.

**Read model in `app` (a query projection, plain data — like `routine`'s `DailyEntry`):**

```go
type TransactionView struct {
	Transaction  *domain.Transaction
	CategoryName string
}

func (s *Service) ListWithCategory(ctx context.Context) ([]TransactionView, error) {
	txs, err := s.tx.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	cats, err := s.cat.FindAll(ctx) // one fetch, build a lookup — avoids N+1
	if err != nil {
		return nil, err
	}
	name := make(map[uint]string, len(cats))
	for _, c := range cats {
		name[c.ID()] = c.Name()
	}
	out := make([]TransactionView, len(txs))
	for i, t := range txs {
		out[i] = TransactionView{Transaction: t, CategoryName: name[t.TransactionCategoryID()]}
	}
	return out, nil
}
```

**Transport** then maps `TransactionView` to a DTO that carries `"category": "Food"` alongside the
existing fields — JSON tags stay in transport, never on the domain.

### B4. New capability: period summary (income vs expense)

A finance app's first real feature beyond CRUD: totals over a date range. This exercises every layer
end to end and is a good template for future reports.

**Domain — extend the port** (`internal/finance/domain/repository.go`):

```go
type Summary struct {
	TotalIncome  int64
	TotalExpense int64
}

type TransactionRepository interface {
	// ...existing methods...
	SummaryBetween(ctx context.Context, from, to time.Time) (Summary, error)
}
```

**Infra** — a single grouped query (this is exactly the kind of SQL the integration test in B5 must
cover, since a fake can't catch a bad `SUM`/`GROUP BY`):

```go
func (r *TransactionGormRepository) SummaryBetween(ctx context.Context, from, to time.Time) (domain.Summary, error) {
	type row struct {
		Type  string
		Total int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&transactionRecord{}).
		Select("type, SUM(amount) AS total").
		Where("date >= ? AND date < ?", from, to).
		Group("type").
		Scan(&rows).Error
	if err != nil {
		return domain.Summary{}, err
	}
	var s domain.Summary
	for _, x := range rows {
		switch domain.TransactionType(x.Type) {
		case domain.Income:
			s.TotalIncome = x.Total
		case domain.Expense:
			s.TotalExpense = x.Total
		}
	}
	return s, nil
}
```

**App** is a one-liner pass-through; **transport** adds `GET /api/transactions/summary?from=&to=`,
parses the dates (reuse the `2006-01-02` parsing from the routine handler's `DailyHistory`), and
returns `{ "income": 50000, "expense": 32000, "net": 18000 }`.

### B5. Harden the edges

Two cheap wins that the migration left for later:

**Graceful shutdown** — `main.go` currently does `log.Fatal(app.Listen(...))`; a SIGTERM kills
in-flight requests. Listen in a goroutine and drain on signal:

```go
go func() {
	if err := app.Listen(":" + cfg.App.Port); err != nil {
		log.Fatalf("listen: %v", err)
	}
}()

quit := make(chan os.Signal, 1)
signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
<-quit
_ = app.ShutdownWithTimeout(10 * time.Second)
```

**One error envelope** — every handler hand-writes `fiber.Map{"error": ...}`. Move that to
`platform/httpx` so the shape is consistent and changeable in one place:

```go
// internal/platform/httpx/respond.go
package httpx

import "github.com/gofiber/fiber/v3"

func Error(c fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg})
}
```

Then `mapError` and the `invalid id` / `invalid body` returns call `httpx.Error(c, fiber.StatusNotFound, "...")`.

---

## Definition of done

- [ ] A1–A4 applied; `go test ./...` and `go test -tags=integration ./...` both green.
- [ ] Fixes committed separately from Part B work.
- [ ] `make test` / `make test-integration` exist; CI runs both on push.
- [ ] Create/Update reject unknown categories (B2), with seeded-fake tests.
- [ ] Transaction responses carry the category name (B3).
- [ ] `GET /api/transactions/summary` returns income/expense/net, covered by an integration test (B4).
- [ ] Graceful shutdown + shared error envelope (B5).
