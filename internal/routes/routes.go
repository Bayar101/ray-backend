package routes

import (
	"github.com/Bayar101/ray-backend/internal/handlers"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

func Register(app *fiber.App, h *handlers.RoutineHandler) {
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New())

	app.Get("/health", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	api := app.Group("/api")
	api.Post("/routines", h.Create)
	api.Get("/routines", h.List)
	api.Get("/routines/:id", h.Get)
	api.Post("/routines/:id/complete", h.Complete)
	api.Get("/history", h.DailyHistory)
}
