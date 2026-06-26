package routes

import (
	"github.com/Bayar101/ray-backend/internal/handlers"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

func Register(app *fiber.App, h *handlers.RoutineHandler, th *handlers.TransactionHandler) {
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New())

	app.Get("/health", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	api := app.Group("/api")

	h.Register(api.Group("/routines"))
	th.Register(api.Group("/transactions"))
}
