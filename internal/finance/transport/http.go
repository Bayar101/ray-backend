package transport

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Bayar101/ray-backend/internal/finance/app"
	"github.com/Bayar101/ray-backend/internal/finance/domain"
	"github.com/Bayar101/ray-backend/internal/platform/httpx"
	"github.com/gofiber/fiber/v3"
)

const dateLayout = "2006-01-02"

// Date is a JSON date encoded as YYYY-MM-DD (no time component).
type Date struct{ time.Time }

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Time.Format(dateLayout) + `"`), nil
}

type Handler struct {
	s *app.Service
}

func NewHandler(s *app.Service) *Handler { return &Handler{s: s} }

func (h *Handler) Register(rg fiber.Router) {
	// Transaction routes
	rg.Post("/create", h.Create)
	rg.Post("/bulk-create", h.BulkCreate)
	rg.Get("/list", h.List)
	rg.Get("/get/:id", h.Get)
	rg.Put("/update/:id", h.Update)
	rg.Delete("/delete/:id", h.Delete)
	//
	rg.Get("/summary", h.Summary)
	// Category routes
	rg.Post("/create-category", h.CreateCategory)
	rg.Get("/list-categories", h.ListCategories)
	rg.Get("/get-category/:id", h.GetCategory)
	rg.Put("/update-category/:id", h.UpdateCategory)
	rg.Delete("/delete-category/:id", h.DeleteCategory)
}

// Transaction routes
type transactionInput struct {
	CategoryID uint                   `json:"category_id" validate:"required"`
	Amount     int64                  `json:"amount" validate:"required"`
	Type       domain.TransactionType `json:"type" validate:"required,oneof=income expense"`
	Note       string                 `json:"note" validate:"max=1000"`
	Date       Date                   `json:"date" validate:"required"`
}

type transactionResponse struct {
	ID         uint                   `json:"id"`
	CategoryID uint                   `json:"category_id"`
	Category   string                 `json:"category,omitempty"`
	Amount     int64                  `json:"amount"`
	Type       domain.TransactionType `json:"type"`
	Note       string                 `json:"note"`
	Date       Date                   `json:"date"`
}

func transactionToResponse(t *domain.Transaction) transactionResponse {
	return transactionResponse{
		ID:         t.ID(),
		CategoryID: t.CategoryID(),
		Amount:     t.Amount(),
		Type:       t.Type(),
		Note:       t.Note(),
		Date:       Date{t.Date()},
	}
}

type summaryResponse struct {
	TotalIncome  int64                     `json:"total_income"`
	TotalExpense int64                     `json:"total_expense"`
	Categories   []categorySummaryResponse `json:"categories"`
}

type categorySummaryResponse struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	TotalIncome  int64  `json:"total_income"`
	TotalExpense int64  `json:"total_expense"`
}

// transactionViewToResponse maps the app read model (transaction + resolved
// category name) onto the wire DTO.
func transactionViewToResponse(v app.TransactionView) transactionResponse {
	resp := transactionToResponse(v.Transaction)
	resp.Category = v.CategoryName
	return resp
}

func summaryToResponse(s domain.Summary) summaryResponse {
	resp := summaryResponse{
		TotalIncome:  s.TotalIncome(),
		TotalExpense: s.TotalExpense(),
		Categories:   make([]categorySummaryResponse, len(s.Categories())),
	}
	for i, cat := range s.Categories() {
		resp.Categories[i] = categorySummaryResponse{
			ID:           cat.ID,
			Name:         cat.Name,
			TotalIncome:  cat.TotalIncome,
			TotalExpense: cat.TotalExpense,
		}
	}
	return resp
}

// #region [Transaction]
func (h *Handler) Create(c fiber.Ctx) error {
	var in transactionInput
	if err := c.Bind().Body(&in); err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid body")
	}
	r, err := h.s.Create(c.Context(), in.CategoryID, in.Amount, in.Type, in.Note, in.Date.Time)
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(transactionToResponse(r))
}

func (h *Handler) BulkCreate(c fiber.Ctx) error {
	var inputs []transactionInput
	if err := c.Bind().Body(&inputs); err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid body")
	}
	list := make([]*domain.Transaction, len(inputs))
	for i, in := range inputs {
		t, err := domain.NewTransaction(in.CategoryID, in.Amount, in.Type, in.Note, in.Date.Time)
		if err != nil {
			return httpx.Error(c, fiber.StatusBadRequest, "invalid transaction")
		}
		list[i] = t
	}

	transactions, err := h.s.BulkCreate(c.Context(), list)
	if err != nil {
		return mapError(c, err)
	}
	out := make([]transactionResponse, len(transactions))
	for i, t := range transactions {
		out[i] = transactionToResponse(t)
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) List(c fiber.Ctx) error {
	ts, err := h.s.List(c.Context())
	if err != nil {
		return mapError(c, err)
	}
	out := make([]transactionResponse, len(ts))
	for i, t := range ts {
		out[i] = transactionViewToResponse(t)
	}
	return c.Status(fiber.StatusOK).JSON(out)
}

func (h *Handler) Get(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid id")
	}
	t, err := h.s.Get(c.Context(), uint(id))
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(transactionViewToResponse(t))
}

func (h *Handler) Update(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid id")
	}
	var in transactionInput
	if err := c.Bind().Body(&in); err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid body")
	}
	t, err := h.s.Update(c.Context(), uint(id), in.CategoryID, in.Amount, in.Type, in.Note, in.Date.Time)
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(transactionToResponse(t))
}

func (h *Handler) Delete(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid id")
	}
	if err := h.s.Delete(c.Context(), uint(id)); err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "transaction deleted"})
}

// #region [Summary]

func (h *Handler) Summary(c fiber.Ctx) error {
	from, err := time.Parse(dateLayout, c.Query("from"))
	if err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid from date")
	}
	to, err := time.Parse(dateLayout, c.Query("to"))
	if err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid to date")
	}
	summary, err := h.s.Summary(c.Context(), from, to)
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(summaryToResponse(*summary))
}

// #region [Category]
// Category routes
type transactionCategoryInput struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

type transactionCategoryResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func transactionCategoryToResponse(c *domain.TransactionCategory) transactionCategoryResponse {
	return transactionCategoryResponse{ID: c.ID(), Name: c.Name()}
}

func (h *Handler) CreateCategory(c fiber.Ctx) error {
	var in transactionCategoryInput
	if err := c.Bind().Body(&in); err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid body")
	}
	cat, err := h.s.CreateCategory(c.Context(), in.Name)
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(transactionCategoryToResponse(cat))
}

func (h *Handler) ListCategories(c fiber.Ctx) error {
	cats, err := h.s.ListCategories(c.Context())
	if err != nil {
		return mapError(c, err)
	}
	out := make([]transactionCategoryResponse, len(cats))
	for i, cat := range cats {
		out[i] = transactionCategoryToResponse(cat)
	}
	return c.Status(fiber.StatusOK).JSON(out)
}

func (h *Handler) GetCategory(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid id")
	}
	cat, err := h.s.GetCategory(c.Context(), uint(id))
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(transactionCategoryToResponse(cat))
}

func (h *Handler) UpdateCategory(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid id")
	}
	var in transactionCategoryInput
	if err := c.Bind().Body(&in); err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid body")
	}
	cat, err := h.s.UpdateCategory(c.Context(), uint(id), in.Name)
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(transactionCategoryToResponse(cat))
}

func (h *Handler) DeleteCategory(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return httpx.Error(c, fiber.StatusBadRequest, "invalid id")
	}
	if err := h.s.DeleteCategory(c.Context(), uint(id)); err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "transaction category deleted"}) // return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "transaction category deleted"})
}

func mapError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrTransactionNotFound), errors.Is(err, domain.ErrTransactionCategoryNotFound):
		return httpx.Error(c, fiber.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrDuplicateTransactionCategory):
		return httpx.Error(c, fiber.StatusConflict, "duplicate category")
	case errors.Is(err, domain.ErrInvalidType), errors.Is(err, domain.ErrTransactionCategoryNameRequired), errors.Is(err, domain.ErrInvalidAmount):
		return httpx.Error(c, fiber.StatusBadRequest, err.Error())
	default:
		return httpx.Error(c, fiber.StatusInternalServerError, "internal server error")
	}
}
