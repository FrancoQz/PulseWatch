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
}