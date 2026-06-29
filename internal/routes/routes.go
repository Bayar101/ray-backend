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

	app.Get("/health", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	api := app.Group("/api")
	rh.Register(api.Group("/routines"))
	fh.Register(api.Group("/transactions"))
}
