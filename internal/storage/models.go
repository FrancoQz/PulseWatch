package storage

import "time"

// Service es algo que monitoreamos.
type Service struct {
	ID   int
	Name string
	URL  string
}

// Check es el resultado de un chequeo puntual.
type Check struct {
	ServiceID  int
	CheckedAt  time.Time
	StatusCode int
	LatencyMs  int
	IsUp       bool
	Error      string // Campo para registrar la razón de la falla.
}

// ServiceStatus es un servicio junto con su estado más reciente
// y su uptime de las últimas 24 horas.
type ServiceStatus struct {
	Service

	// Punteros porque un servicio recién creado todavía no tiene chequeos.
	LastCheckedAt  *time.Time
	LastStatusCode *int
	LastLatencyMs  *int
	LastIsUp       *bool

	Uptime24h float64 // porcentaje 0-100
	Checks24h int
}