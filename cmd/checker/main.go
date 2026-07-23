package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/FrancoQz/PulseWatch/internal/storage"
)

func check(svc storage.Service) storage.Check {
	start := time.Now()
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(svc.URL)
	if err != nil {
		return storage.Check{ServiceID: svc.ID, IsUp: false}
	}
	defer resp.Body.Close()

	return storage.Check{
		ServiceID:  svc.ID,
		StatusCode: resp.StatusCode,
		LatencyMs:  int(time.Since(start).Milliseconds()),
		IsUp:       resp.StatusCode < 400,
	}
}

func main() {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://pulsewatch:pulsewatch_dev@localhost:5432/pulsewatch"
	}

	store, err := storage.New(ctx, dsn)
	if err != nil {
		log.Fatalf("no pude conectar a la base: %v", err)
	}
	defer store.Close()

	services, err := store.ListServices(ctx)
	if err != nil {
		log.Fatalf("no pude leer los servicios: %v", err)
	}

	var wg sync.WaitGroup
	for _, svc := range services {
		wg.Add(1)
		go func(s storage.Service) {
			defer wg.Done()

			result := check(s)

			if err := store.SaveCheck(ctx, result); err != nil {
				log.Printf("error guardando %s: %v", s.Name, err)
				return
			}

			estado := "UP"
			if !result.IsUp {
				estado = "DOWN"
			}
			fmt.Printf("%-10s %-4s %dms\n", s.Name, estado, result.LatencyMs)
		}(svc)
	}
	wg.Wait()
}