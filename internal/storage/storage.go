package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Storage envuelve el pool de conexiones a la base.
type Storage struct {
	pool *pgxpool.Pool
}

// New abre el pool y verifica que la base responda.
func New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("creando pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pinging db: %w", err)
	}

	return &Storage{pool: pool}, nil
}

// Close libera las conexiones.
func (s *Storage) Close() {
	s.pool.Close()
}

// ListServices trae todos los servicios a monitorear.
func (s *Storage) ListServices(ctx context.Context) ([]Service, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, url FROM services ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("consultando services: %w", err)
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var svc Service
		if err := rows.Scan(&svc.ID, &svc.Name, &svc.URL); err != nil {
			return nil, fmt.Errorf("escaneando service: %w", err)
		}
		services = append(services, svc)
	}
	return services, rows.Err()
}

// SaveCheck persiste el resultado de un chequeo.
func (s *Storage) SaveCheck(ctx context.Context, c Check) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO checks (service_id, status_code, latency_ms, is_up)
		 VALUES ($1, $2, $3, $4)`,
		c.ServiceID, c.StatusCode, c.LatencyMs, c.IsUp)
	if err != nil {
		return fmt.Errorf("guardando check: %w", err)
	}
	return nil
}