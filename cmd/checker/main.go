package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/FrancoQz/PulseWatch/internal/storage"
)

const defaultInterval = 60 * time.Second

// httpClient se crea una sola vez y se comparte entre todas las goroutines.
// El http.Client es seguro para uso concurrente y mantiene un pool de
// conexiones reutilizables, así que crear uno por chequeo sería un desperdicio.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func check(ctx context.Context, svc storage.Service) storage.Check {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.URL, nil)
	if err != nil {
		return storage.Check{
			ServiceID: svc.ID,
			IsUp:      false,
			Error:     err.Error(), // Captura si falla armar la request
		}
	}
	req.Header.Set("User-Agent", "PulseWatch/1.0 (+https://github.com/FrancoQz/PulseWatch)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return storage.Check{
			ServiceID: svc.ID,
			IsUp:      false,
			Error:     err.Error(), // Captura si falla la red / conexión / timeout
		}
	}
	defer resp.Body.Close()

	// Si llegó hasta acá, la petición HTTP completó sin error de red
	return storage.Check{
		ServiceID:  svc.ID,
		StatusCode: resp.StatusCode,
		LatencyMs:  int(time.Since(start).Milliseconds()),
		IsUp:       resp.StatusCode < 400,
		Error:      "", // No hubo error de red (si dio 404/500, IsUp es false pero el status_code lo explica)
	}
}

// runCycle chequea todos los servicios una vez y guarda los resultados.
func runCycle(ctx context.Context, store *storage.Storage) {
	services, err := store.ListServices(ctx)
	if err != nil {
		log.Printf("no pude leer los servicios: %v", err)
		return
	}

	if len(services) == 0 {
		log.Println("no hay servicios cargados")
		return
	}

	var wg sync.WaitGroup
	for _, svc := range services {
		wg.Add(1)
		go func(s storage.Service) {
			defer wg.Done()

			result := check(ctx, s)

			if err := store.SaveCheck(ctx, result); err != nil {
				log.Printf("error guardando %s: %v", s.Name, err)
				return
			}

			estado := "UP"
			if !result.IsUp {
				estado = "DOWN"
			}
			log.Printf("%-12s %-4s %4dms", s.Name, estado, result.LatencyMs)
		}(svc)
	}
	wg.Wait()
}

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

	interval := defaultInterval
	if v := os.Getenv("CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		} else {
			log.Printf("CHECK_INTERVAL invalido (%q), uso %v", v, interval)
		}
	}

	log.Printf("PulseWatch arrancando — chequeos cada %v", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runCycle(ctx, store) // primera vuelta inmediata

	for {
		select {
		case <-ticker.C:
			runCycle(ctx, store)
		case <-ctx.Done():
			log.Println("apagando...")
			return
		}
	}
}