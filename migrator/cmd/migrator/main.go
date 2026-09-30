package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/identicalaffiliation/web-go-project/migrator/config"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := sql.Open("postgres", cfg.DbUrl)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := pool.Close(); err != nil {
			log.Println(err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal(fmt.Errorf("failed to set goose dialect: %w", err))
	}

	if err := goose.UpContext(ctx, pool, cfg.MigrationsPath); err != nil {
		log.Fatal(err)
	}

	log.Println("migrations was add successfully")
}
