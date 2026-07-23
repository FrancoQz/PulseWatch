-- Servicios que vamos a monitorear
CREATE TABLE services (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    url         TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Cada chequeo individual (la serie temporal)
CREATE TABLE checks (
    service_id  INTEGER NOT NULL REFERENCES services(id),
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status_code INTEGER,
    latency_ms  INTEGER,
    is_up       BOOLEAN NOT NULL
);

-- Convertir checks en hypertable (TimescaleDB)
SELECT create_hypertable('checks', 'checked_at');

-- Servicios de prueba iniciales
INSERT INTO services (name, url) VALUES
    ('Google', 'https://www.google.com'),
    ('GitHub', 'https://github.com'),
    ('UADE',   'https://www.uade.edu.ar');