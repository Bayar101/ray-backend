package transport

import (
	"errors"
	"strconv"
	"time"

	"github.com/Bayar101/ray-backend/internal/routine/app"
	"github.com/Bayar101/ray-backend/internal/routine/domain"
	"github.com/gofiber/fiber/v3"
)

type Handler struct{ s *app.Service }

func NewHandler(s *app.Service) *Handler { return &Handler{s: s} }

func (h *Handler) Register(rg fiber.Router) {
	rg.Post("/create", h.Create)
	rg.Get("/list", h.List)
	rg.Get("/get/:id", h.Get)
	rg.Put("/update/:id", h.Update)
	rg.Delete("/delete/:id", h.Delete)
	rg.Post("/complete/:id", h.Complete)
	rg.Get("/history?date=YYYY-MM-DD", h.DailyHistory)
}

type routineInput struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=1000"`
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
	var in routineInput
	if err := c.Bind().Body(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	r, err := h.s.Create(c.Context(), in.Name, in.Description)
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toResponse(r))
}

func (h *Handler) List(c fiber.Ctx) error {
	routines, err := h.s.List(c.Context())
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
	r, err := h.s.Get(c.Context(), uint(id))
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
	var in routineInput
	if err := c.Bind().Body(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	r, err := h.s.Update(c.Context(), uint(id), in.Name, in.Description)
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
	if err := h.s.Delete(c.Context(), uint(id)); err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "routine deleted"})
}

func (h *Handler) Complete(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	log, err := h.s.Complete(c.Context(), uint(id))
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
	entries, err := h.s.DailyHistory(c.Context(), day)
	if err != nil {
		return mapError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(entries)
}

func mapError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrRoutineNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "routine not found"})
	case errors.Is(err, domain.ErrDuplicateName):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "routine name already exists"})
	case errors.Is(err, domain.ErrNameRequired), errors.Is(err, domain.ErrNameTooLong), errors.Is(err, domain.ErrDescriptionTooLong):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}
