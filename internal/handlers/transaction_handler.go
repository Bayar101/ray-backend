package handlers

import (
	"strconv"
	"time"

	"github.com/Bayar101/ray-backend/internal/models"
	"github.com/Bayar101/ray-backend/internal/services"
	"github.com/gofiber/fiber/v3"
)

type TransactionHandler struct {
	svc *services.TransactionService
}

func NewTransactionHandler(svc *services.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

func (h *TransactionHandler) Register(rg fiber.Router) {
	rg.Post("/create", h.Create)
	rg.Post("/bulk-create", h.BulkCreate)
	rg.Post("/create-category", h.CreateCategory)
	rg.Get("/list", h.List)
	rg.Get("/list-categories", h.ListCategories)
	rg.Get("/get/:id", h.Get)
	rg.Get("/get-category/:id", h.GetCategory)
	rg.Put("/update/:id", h.Update)
	rg.Put("/update-category/:id", h.UpdateCategory)
	rg.Delete("/delete/:id", h.Delete)
	rg.Delete("/delete-category/:id", h.DeleteCategory)
}

type transactionInput struct {
	CategoryID uint                   `json:"category_id" validate:"required"`
	Amount     int64                  `json:"amount" validate:"required"`
	Type       models.TransactionType `json:"type" validate:"required,oneof=income expense"`
	Note       string                 `json:"note" validate:"max=1000"`
	Date       time.Time              `json:"date" validate:"required"`
}

type createTransactionCategoryInput struct {
	Name string `json:"name" validate:"required,min=1,max=100,unique"`
}

// #region  [Create]
func (h *TransactionHandler) Create(c fiber.Ctx) error {
	var input transactionInput
	if err := c.Bind().Body(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validate.Struct(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": validationErrors(err)})
	}

	transaction, err := h.svc.Create(c.Context(), models.Transaction{
		CategoryID: input.CategoryID,
		Amount:     input.Amount,
		Type:       input.Type,
		Note:       input.Note,
		Date:       input.Date,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(transaction)
}

func (h *TransactionHandler) BulkCreate(c fiber.Ctx) error {
	var input []transactionInput
	if err := c.Bind().Body(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validate.Struct(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": validationErrors(err)})
	}

	list := make([]models.Transaction, len(input))
	for i, in := range input {
		list[i] = models.Transaction{
			CategoryID: in.CategoryID,
			Amount:     in.Amount,
			Type:       in.Type,
			Note:       in.Note,
			Date:       in.Date,
		}
	}
	transactions, err := h.svc.BulkCreate(c.Context(), list)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(transactions)
}

func (h *TransactionHandler) CreateCategory(c fiber.Ctx) error {
	var input createTransactionCategoryInput
	if err := c.Bind().Body(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validate.Struct(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": validationErrors(err)})
	}

	category, err := h.svc.CreateCategory(c.Context(), models.TransactionCategory{
		Name: input.Name,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(category)
}

// #region [List]
func (h *TransactionHandler) List(c fiber.Ctx) error {
	transactions, err := h.svc.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(transactions)
}

func (h *TransactionHandler) ListCategories(c fiber.Ctx) error {
	categories, err := h.svc.ListCategories(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(categories)
}

// #region [Get]
func (h *TransactionHandler) Get(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	transaction, err := h.svc.Get(c.Context(), uint(id))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(transaction)
}

func (h *TransactionHandler) GetCategory(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	category, err := h.svc.GetCategory(c.Context(), uint(id))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(category)
}

// #region [Update]
func (h *TransactionHandler) Update(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var input transactionInput
	if err := c.Bind().Body(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validate.Struct(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": validationErrors(err)})
	}

	transaction, err := h.svc.Update(c.Context(), uint(id), models.Transaction{
		CategoryID: input.CategoryID,
		Amount:     input.Amount,
		Type:       input.Type,
		Note:       input.Note,
		Date:       input.Date,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(transaction)
}

func (h *TransactionHandler) UpdateCategory(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var input createTransactionCategoryInput
	if err := c.Bind().Body(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validate.Struct(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": validationErrors(err)})
	}
	category, err := h.svc.UpdateCategory(c.Context(), uint(id), models.TransactionCategory{
		Name: input.Name,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(category)
}

// #region [Delete]
func (h *TransactionHandler) Delete(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.svc.Delete(c.Context(), uint(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "transaction deleted"})
}

func (h *TransactionHandler) DeleteCategory(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.svc.DeleteCategory(c.Context(), uint(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "transaction category deleted"})
}
