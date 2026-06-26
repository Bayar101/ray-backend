package main

import (
	"log"

	"github.com/Bayar101/ray-backend/internal/config"
	"github.com/Bayar101/ray-backend/internal/database"
	"github.com/Bayar101/ray-backend/internal/handlers"
	"github.com/Bayar101/ray-backend/internal/routes"
	"github.com/Bayar101/ray-backend/internal/services"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

type structValidator struct{ v *validator.Validate }

func (s structValidator) Validate(out any) error { return s.v.Struct(out) }

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

	txSvc := services.NewTransactionService(db)
	txHandler := handlers.NewTransactionHandler(txSvc)

	app := fiber.New(fiber.Config{
		StructValidator: structValidator{v: validator.New()},
	})

	routes.Register(app, handler, txHandler)

	log.Fatal(app.Listen(":" + cfg.App.Port))
}
