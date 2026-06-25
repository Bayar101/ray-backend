package handlers

import (
	"errors"
	"strconv"
	"time"

	"github.com/Bayar101/ray-backend/internal/services"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

type RoutineHandler struct {
	svc *services.RoutineService
}

func NewRoutineHandler(svc *services.RoutineService) *RoutineHandler {
	return &RoutineHandler{svc: svc}
}

func (h *RoutineHandler) Register(rg fiber.Router) {
	rg.Post("/routines", h.Create)
	rg.Get("/routines", h.List)
	rg.Get("/routines/:id", h.Get)
	rg.Post("/routines/:id/complete", h.Complete)
	rg.Get("/history?date=YYYY-MM-DD", h.DailyHistory)
}

type createRoutineInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *RoutineHandler) Create(c fiber.Ctx) error {
	var input createRoutineInput
	if err := c.Bind().Body(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	routine, err := h.svc.Create(c.Context(), input.Name, input.Description)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(routine)
}

func (h *RoutineHandler) List(c fiber.Ctx) error {
	routines, err := h.svc.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(routines)
}

func (h *RoutineHandler) Get(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	routine, err := h.svc.Get(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "routine not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(routine)
}

func (h *RoutineHandler) Complete(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	log, err := h.svc.Complete(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "routine not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(log)
}

func (h *RoutineHandler) DailyHistory(c fiber.Ctx) error {
	dateStr := c.Query("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	entries, err := h.svc.DailyHistory(c.Context(), date)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(entries)
}
