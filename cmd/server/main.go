package main

import (
	"log"

	"github.com/Bayar101/ray-backend/internal/platform/config"
	"github.com/Bayar101/ray-backend/internal/platform/database"
	"github.com/Bayar101/ray-backend/internal/routes"

	// Routine context
	routineapp "github.com/Bayar101/ray-backend/internal/routine/app"
	routineinfra "github.com/Bayar101/ray-backend/internal/routine/infra"
	routinetransport "github.com/Bayar101/ray-backend/internal/routine/transport"

	// Finance context
	financeapp "github.com/Bayar101/ray-backend/internal/finance/app"
	financeinfra "github.com/Bayar101/ray-backend/internal/finance/infra"
	financetransport "github.com/Bayar101/ray-backend/internal/finance/transport"

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

	db, err := database.Connect(cfg.DB,
		append(routineinfra.Models(), financeinfra.Models()...)...,
	)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	// routine context
	routineRepo := routineinfra.NewGormRepository(db)
	routineSvc := routineapp.NewService(routineRepo)
	routineHTTP := routinetransport.NewHandler(routineSvc)

	// finance context
	txRepo := financeinfra.NewTransactionGormRepository(db)
	txCatRepo := financeinfra.NewTransactionCategoryGormRepository(db)
	financeSvc := financeapp.NewService(txRepo, txCatRepo)
	financeHTTP := financetransport.NewHandler(financeSvc)

	app := fiber.New(fiber.Config{
		StructValidator: structValidator{v: validator.New()},
	})

	routes.Register(app, routineHTTP, financeHTTP)

	log.Fatal(app.Listen(":" + cfg.App.Port))
}
