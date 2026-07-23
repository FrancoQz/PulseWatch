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
		`INSERT INTO checks (service_id, status_code, latency_ms, is_up, error)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''))`,
		c.ServiceID, c.StatusCode, c.LatencyMs, c.IsUp, c.Error)
	if err != nil {
		return fmt.Errorf("guardando check: %w", err)
	}
	return nil
}

// CreateService da de alta un servicio nuevo.
func (s *Storage) CreateService(ctx context.Context, name, url string) (Service, error) {
	var svc Service
	err := s.pool.QueryRow(ctx,
		`INSERT INTO services (name, url)
		 VALUES ($1, $2)
		 RETURNING id, name, url`,
		name, url).Scan(&svc.ID, &svc.Name, &svc.URL)
	if err != nil {
		return Service{}, fmt.Errorf("creando service: %w", err)
	}
	return svc, nil
}

// ListServiceStatus trae cada servicio con su último chequeo y su uptime de 24h.
func (s *Storage) ListServiceStatus(ctx context.Context) ([]ServiceStatus, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			s.id, s.name, s.url,
			last.checked_at, last.status_code, last.latency_ms, last.is_up,
			COALESCE(stats.uptime, 0), COALESCE(stats.total, 0)
		FROM services s
		LEFT JOIN LATERAL (
			SELECT checked_at, status_code, latency_ms, is_up
			FROM checks
			WHERE service_id = s.id
			ORDER BY checked_at DESC
			LIMIT 1
		) last ON true
		LEFT JOIN LATERAL (
			SELECT
				avg(CASE WHEN is_up THEN 1.0 ELSE 0.0 END) * 100 AS uptime,
				count(*) AS total
			FROM checks
			WHERE service_id = s.id
			  AND checked_at > now() - interval '24 hours'
		) stats ON true
		ORDER BY s.id`)
	if err != nil {
		return nil, fmt.Errorf("consultando estado de services: %w", err)
	}
	defer rows.Close()

	var out []ServiceStatus
	for rows.Next() {
		var st ServiceStatus
		if err := rows.Scan(
			&st.ID, &st.Name, &st.URL,
			&st.LastCheckedAt, &st.LastStatusCode, &st.LastLatencyMs, &st.LastIsUp,
			&st.Uptime24h, &st.Checks24h,
		); err != nil {
			return nil, fmt.Errorf("escaneando estado: %w", err)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// ListChecks trae el historial reciente de un servicio.
func (s *Storage) ListChecks(ctx context.Context, serviceID, hours, limit int) ([]Check, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT service_id, checked_at, COALESCE(status_code, 0),
		       COALESCE(latency_ms, 0), is_up, COALESCE(error, '')
		FROM checks
		WHERE service_id = $1
		  AND checked_at > now() - make_interval(hours => $2)
		ORDER BY checked_at DESC
		LIMIT $3`,
		serviceID, hours, limit)
	if err != nil {
		return nil, fmt.Errorf("consultando checks: %w", err)
	}
	defer rows.Close()

	var out []Check
	for rows.Next() {
		var c Check
		if err := rows.Scan(&c.ServiceID, &c.CheckedAt, &c.StatusCode,
			&c.LatencyMs, &c.IsUp, &c.Error); err != nil {
			return nil, fmt.Errorf("escaneando check: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ServiceExists dice si un servicio existe.
func (s *Storage) ServiceExists(ctx context.Context, id int) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM services WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("verificando service: %w", err)
	}
	return exists, nil
}

// Ping verifica que la base responda.
func (s *Storage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
