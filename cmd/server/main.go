package main

import (
	"log"

	"github.com/Bayar101/ray-backend/internal/config"
	"github.com/Bayar101/ray-backend/internal/database"
	"github.com/Bayar101/ray-backend/internal/handlers"
	"github.com/Bayar101/ray-backend/internal/routes"
	"github.com/Bayar101/ray-backend/internal/services"
	"github.com/gofiber/fiber/v3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	db, err := database.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	svc := services.NewRoutineService(db)
	handler := handlers.NewRoutineHandler(svc)

	app := fiber.New()
	routes.Register(app, handler)

	log.Fatal(app.Listen(":" + cfg.App.Port))
}
