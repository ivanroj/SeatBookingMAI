package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/mitrich772/SeatBookingMAI/backend/internal/app"
	"github.com/mitrich772/SeatBookingMAI/backend/internal/httpapi"
	"github.com/mitrich772/SeatBookingMAI/backend/internal/repo/postgres"
)

func main() {
	dbURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/seat_booking?sslmode=disable")
	port := getenv("APP_PORT", "8080")

	db, err := openDBWithRetry(dbURL, 30*time.Second)
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}
	defer db.Close()

	repo := postgres.New(db)
	service := app.NewService(repo)
	handler := httpapi.NewHandler(service)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("backend started on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func openDBWithRetry(dsn string, timeout time.Duration) (*sql.DB, error) {
	deadline := time.Now().Add(timeout)
	for {
		db, err := sql.Open("pgx", dsn)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err = db.PingContext(ctx)
			cancel()
			if err == nil {
				return db, nil
			}
			_ = db.Close()
		}

		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(time.Second)
	}
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
