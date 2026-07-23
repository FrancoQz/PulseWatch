package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FrancoQz/PulseWatch/internal/api"
	"github.com/FrancoQz/PulseWatch/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://pulsewatch:pulsewatch_dev@localhost:5432/pulsewatch"
	}

	store, err := storage.New(ctx, dsn)
	if err != nil {
		log.Fatalf("no pude conectar a la base: %v", err)
	}
	defer store.Close()

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      api.NewServer(store),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// El servidor corre en su propia goroutine para que main pueda
	// quedarse esperando la señal de apagado.
	go func() {
		log.Printf("API escuchando en %s", addr)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("servidor caido: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("apagando API...")

	// Le damos 10s a las requests en curso para terminar.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("apagado forzado: %v", err)
	}
}