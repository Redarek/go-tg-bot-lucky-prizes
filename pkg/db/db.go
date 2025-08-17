package db

import (
	"context"
	"fmt"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/config"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(cfg *config.Config) *pgxpool.Pool {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)

	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("parse pool config error: %v", err)
	}

	// Адекватные дефолты (можно вынести в env при желании)
	pcfg.MaxConns = 50
	pcfg.MinConns = 5
	pcfg.MaxConnLifetime = time.Hour
	pcfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), pcfg)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	return pool
}
