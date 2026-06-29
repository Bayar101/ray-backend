package transport

import (
	"errors"
	"strconv"
	"time"

	"github.com/Bayar101/ray-backend/internal/finance/app"
	"github.com/Bayar101/ray-backend/internal/finance/domain"
	"github.com/gofiber/fiber/v3"
)

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
	Date       time.Time              `json:"date" validate:"required"`
}

type transactionResponse struct {
	ID         uint                   `json:"id"`
	CategoryID uint                   `json:"category_id"`
	Amount     int64                  `json:"amount"`
	Type       domain.TransactionType `json:"type"`
	Note       string                 `json:"note"`
	Date       time.Time              `json:"date"`
}

func transactionToResponse(t *domain.Transaction) transactionResponse {
	return transactionResponse{
		ID:         t.ID(),
		CategoryID: t.TransactionCategoryID(),
		Amount:     t.Amount().Cents(),
		Type:       t.Type(),
		Note:       t.Note(),
		Date:       t.Date(),
	}
}

func (h *Handler) Create(c fiber.Ctx) error {
	var in transactionInput
	if err := c.Bind().Body(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invaldi body"})
	}
	r, err := h.s.Create(c.Context(), in.CategoryID, in.Amount, in.Type, in.Note, in.Date)
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(transactionToResponse(r))
}

func (h *Handler) BulkCreate(c fiber.Ctx) error {
	var inputs []transactionInput
	if err := c.Bind().Body(&inputs); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	list := make([]*domain.Transaction, len(inputs))
	for i, in := range inputs {
		amount, err := domain.NewMoney(in.Amount)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid amount"})
		}
		list[i], err = domain.NewTransaction(in.CategoryID, amount, in.Type, in.Note, in.Date)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid transaction"})
		}
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
		out[i] = transactionToResponse(t)
	}
	return c.Status(fiber.StatusOK).JSON(out)
}

func (h *Handler) Get(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	t, err := h.s.Get(c.Context(), uint(id))
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(transactionToResponse(t))
}

func (h *Handler) Update(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var in transactionInput
	if err := c.Bind().Body(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	t, err := h.s.Update(c.Context(), uint(id), in.CategoryID, in.Amount, in.Type, in.Note, in.Date)
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(transactionToResponse(t))
}

func (h *Handler) Delete(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.s.Delete(c.Context(), uint(id)); err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "transaction deleted"})
}

// Category routes
type transactionCategoryInput struct {
	Name string `json:"name" validate:"required,min=1,max=100,unique"`
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var in transactionCategoryInput
	if err := c.Bind().Body(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.s.DeleteCategory(c.Context(), uint(id)); err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "transaction category deleted"})
}

func mapError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrTransactionNotFound), errors.Is(err, domain.ErrTransactionCategoryNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	case errors.Is(err, domain.ErrDuplicateTransactionCategory):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "duplicate category"})
	case errors.Is(err, domain.ErrInvalidType), errors.Is(err, domain.ErrTransactionCategoryNameRequired), errors.Is(err, domain.ErrNegativeAmount):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}
