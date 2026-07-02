# ray-backend — remaining fixes & next steps

Tree is currently red: one build break, five failing unit tests, three bugs the compiler/unit suite
miss. Fix A1–A6 in order, verify with A7, then Part B.

---

## Part A — Fix what's broken

### A1. Build break: `SummaryBetween` return type mismatch (blocker)

Port declares `SummaryBetween(...) (*Summary, error)` (pointer); infra returns `(domain.Summary, error)`
(value), so `*TransactionGormRepository` no longer satisfies the interface — `main.go:53` won't
compile. Make infra return the pointer (`NewSummary` already returns `*Summary`).

**File:** `internal/finance/infra/gorm_repository.go`, `SummaryBetween` (~line 131).

```go
// before — value return, three places
func (r *TransactionGormRepository) SummaryBetween(ctx context.Context, from, to time.Time) (domain.Summary, error) {
	...
	return domain.Summary{}, err               // error path
	return domain.Summary{}, domain.ErrInvalidType
	summary := domain.NewSummary(totalIncome, totalExpense, categories)
	return *summary, nil
}

// after — pointer return, matches the interface
func (r *TransactionGormRepository) SummaryBetween(ctx context.Context, from, to time.Time) (*domain.Summary, error) {
	...
	return nil, err
	return nil, domain.ErrInvalidType
	return domain.NewSummary(totalIncome, totalExpense, categories), nil
}
```

`h.s.Summary` already returns `*domain.Summary`; the handler's `summaryToResponse(*summary)` still
works.

### A2. Five `Create` unit tests fail: wrong validation order (blocker)

`Service.Create` calls `cat.FindByID` **before** `NewTransaction`, so the reject-tests
(categoryID=1 vs empty repo) get `NotFound` instead of the domain error, and `categoryID=0` gets
`NotFound` instead of `Required`. Construct first (validates all invariants incl. `categoryID==0`),
then check the reference — makes every current test pass with no test edits.

**File:** `internal/finance/app/service.go`, `Create` (~line 24).

```go
func (s *Service) Create(ctx context.Context, categoryID uint, amount int64, txType domain.TransactionType, note string, date time.Time) (*domain.Transaction, error) {
	t, err := domain.NewTransaction(categoryID, amount, txType, note, date)
	if err != nil {
		return nil, err // ErrTransactionCategoryRequired / ErrInvalidAmount / ...
	}
	if _, err := s.cat.FindByID(ctx, categoryID); err != nil {
		return nil, err // ErrTransactionCategoryNotFound for a non-zero, unknown id
	}
	if err := s.tx.Save(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}
```

```text
RejectsInvalidCategoryID  cat=0           → ErrTransactionCategoryRequired  ✓
RejectsInvalidAmount      cat=1, amt=0    → ErrInvalidAmount                ✓
RejectsUnknownCategory    cat=999 valid   → FindByID → NotFound             ✓
PersistsAndAssignsID      seeded cat      → FindByID ok → Save              ✓
```

`Update` already guards `categoryID != 0` before `FindByID` — leave it.

### A3. `httpx.Error` emits the literal string `"msg"` (runtime bug)

Helper ignores its `msg` argument — every error body is `{"error":"msg"}`.

**File:** `internal/platform/httpx/respond.go:6`.

```go
// before
	return c.Status(status).JSON(fiber.Map{"error": "msg"})   // literal — bug
// after
	return c.Status(status).JSON(fiber.Map{"error": msg})     // the argument
```

Proof: `curl -s localhost:8080/api/transactions/get/999` → `{"error":"not found"}`, not `"msg"`.

### A4. `SummaryBetween` per-category totals always zero and unnamed (feature bug)

Query is fine, but the loop never copies `row.Total` into `CategorySummary`, never sets `Name`, and
groups by `(type, category_id)` — so a category appears twice, both zeroed. Top-line totals are
correct; the breakdown is garbage.

**File:** `internal/finance/infra/gorm_repository.go`, inside `for _, row := range rows`.

```go
// before — Total discarded, Name never set, one entry per (type,category) row
category := domain.CategorySummary{ID: row.CategoryID, TotalIncome: 0, TotalExpense: 0}
categories = append(categories, category)

// after — declare `byCat := map[uint]*domain.CategorySummary{}` above the loop, then:
c, ok := byCat[row.CategoryID]
if !ok {
	c = &domain.CategorySummary{ID: row.CategoryID}
	byCat[row.CategoryID] = c
}
switch row.Type {
case domain.Income:
	c.TotalIncome += row.Total
case domain.Expense:
	c.TotalExpense += row.Total
}
// after the loop, flatten byCat into []domain.CategorySummary for NewSummary(...)
```

`Name` isn't on `transactionRecord` — resolve it via the seam chosen in B4.

### A5. CI workflow is malformed YAML and pins the wrong Go version

`.github/workflows/ci.yml` won't parse: `-uses:`/`-run:` missing the space (`- uses:`), `with:` not
nested under its step, `action/checkout` typo (→ `actions/checkout`), and Go pinned `1.23` while
`go.mod` says `1.26.4`.

**File:** `.github/workflows/ci.yml` — replace with:

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'          # match go.mod (go 1.26.4)
      - run: go vet ./...
      - run: make test                # unit
      - run: make test-integration    # integration (Docker is on the runner)
```

Validate: `python3 -c 'import yaml; yaml.safe_load(open(".github/workflows/ci.yml"))' && echo OK`

### A6. (Optional) `Delete` uses a value receiver

`func (r TransactionGormRepository) Delete(...)` — only method not on `*TransactionGormRepository`.
Compiles, but inconsistent.

**File:** `internal/finance/infra/gorm_repository.go:120`.

```go
// before
func (r TransactionGormRepository) Delete(ctx context.Context, id uint) error {
// after
func (r *TransactionGormRepository) Delete(ctx context.Context, id uint) error {
```

### A7. Verify green, then commit separately

```bash
go build ./...                      # must compile (A1)
go test ./...                       # unit — all ok (A2)
go test -tags=integration ./...     # integration — needs Docker (A4)
git add -A && git commit -m "fix: summary pointer return, Create order, httpx envelope, per-category totals, ci"
```

---

## Part B — Next steps

### B1. Summary integration test (closes the A4 loop)

`finance/infra/integration_test.go` has nothing exercising `SummaryBetween` — the exact query A4
rewrote, which a fake can't validate. Add one against the real container (`newPG(t)` exists).

```go
func TestSummaryBetween_AggregatesByType(t *testing.T) {
	db := newPG(t)
	repo := infra.NewTransactionGormRepository(db)
	ctx := context.Background()

	mustSave := func(catID uint, amount int64, typ domain.TransactionType, day string) {
		d, _ := time.Parse("2006-01-02", day)
		tx, err := domain.NewTransaction(catID, amount, typ, "", d)
		if err != nil { t.Fatal(err) }
		if err := repo.Save(ctx, tx); err != nil { t.Fatal(err) }
	}
	mustSave(1, 50000, domain.Income, "2026-06-10")
	mustSave(1, 20000, domain.Expense, "2026-06-12")
	mustSave(2, 12000, domain.Expense, "2026-06-15")
	mustSave(1, 99999, domain.Income, "2026-07-05") // outside window — must be excluded

	from, _ := time.Parse("2006-01-02", "2026-06-01")
	to, _ := time.Parse("2006-01-02", "2026-07-01")
	s, err := repo.SummaryBetween(ctx, from, to)
	if err != nil { t.Fatal(err) }
	if s.TotalIncome() != 50000 || s.TotalExpense() != 32000 {
		t.Fatalf("want income=50000 expense=32000, got %d/%d", s.TotalIncome(), s.TotalExpense())
	}
}
```

> Boundary: infra uses `date BETWEEN ? AND ?` (inclusive), routine handler uses `>= from AND < to`
> (half-open). Pick one and align the `Where` clause + this test. Half-open (`< to`) is the safer
> default for date ranges.

### B2. App-layer unit tests for the new read model

`Create`'s referential check and `List`/`Get` category composition lack happy-path tests.

```go
func TestList_ComposesCategoryName(t *testing.T) {
	cat := newFakeCategoryRepo()
	food, _ := domain.NewTransactionCategory("Food")
	_ = cat.Save(context.Background(), food)

	svc := app.NewService(newFakeRepo(), cat)
	_, _ = svc.Create(context.Background(), food.ID(), 100, domain.Expense, "lunch", time.Now())

	views, err := svc.List(context.Background())
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(views) != 1 || views[0].CategoryName != "Food" {
		t.Fatalf("want one view named Food, got %+v", views)
	}
}
```

### B3. HTTP contract test for the summary endpoint

`GET /api/transactions/summary` 400s on a bad `from`/`to`, but nothing asserts 400-not-500 or the
`{total_income,total_expense,categories}` shape. Use `app.Test(httptest.NewRequest(...))` (no DB,
fake-backed service):

```go
req := httptest.NewRequest("GET", "/api/transactions/summary?from=nope&to=2026-07-01", nil)
resp, _ := app.Test(req)
// want 400 and body {"error":"invalid from date"}   (also proves A3)
```

### B4. Choose the CategorySummary name-resolution seam

A4 leaves category `Name` empty. Pick one:

- **JOIN in infra:** `SummaryBetween` joins `transaction_categories`, selects `name`. One query, but
  the aggregate query now spans two tables.
- **Compose in app** (mirrors the B2 read model): infra returns ids+totals; `Service.Summary`
  fetches `cat.FindAll` once and fills `Name` from a `map[uint]string`. Single-table query; app owns
  cross-aggregate reads. **Recommended** — consistent with how `List` already resolves names.

### B5. Harden the edges

- **Log the drain result.** `main.go` does `_ = app.ShutdownWithTimeout(10 * time.Second)` — a
  timed-out drain looks clean. `if err := ...; err != nil { log.Printf("shutdown: %v", err) }`.
- **Consistent 400 in finance `mapError`.** It maps `ErrInvalidType`/`ErrInvalidAmount`/
  `ErrTransactionCategoryNameRequired` to 400 but omits `ErrNoteTooLong`, `ErrInvalidDate`,
  `ErrTransactionCategoryRequired` — those fall through to 500.

```go
case errors.Is(err, domain.ErrInvalidType),
	errors.Is(err, domain.ErrInvalidAmount),
	errors.Is(err, domain.ErrNoteTooLong),
	errors.Is(err, domain.ErrInvalidDate),
	errors.Is(err, domain.ErrTransactionCategoryRequired),
	errors.Is(err, domain.ErrTransactionCategoryNameRequired):
	return httpx.Error(c, fiber.StatusBadRequest, err.Error())
```

---

## Definition of done

- [ ] A1 — `go build ./...` compiles.
- [ ] A2 — `go test ./...` green, tests unchanged.
- [ ] A3 — error bodies show the real message.
- [ ] A4 — per-category totals real and de-duplicated.
- [ ] A5 — CI YAML parses, pins Go 1.26, runs vet + unit + integration.
- [ ] Fixes committed separately from Part B.
- [ ] `SummaryBetween` integration test (B1); summary HTTP contract test (B3).
- [ ] Name-resolution seam chosen, names populated (B4).
- [ ] `mapError` 400s all domain-validation errors; shutdown error logged (B5).
