package api

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/FrancoQz/PulseWatch/internal/storage"
)

// Server agrupa las dependencias de la API y su router.
type Server struct {
	store *storage.Storage
	mux   *http.ServeMux
}

func NewServer(store *storage.Storage) *Server {
	s := &Server{
		store: store,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

// ServeHTTP hace que Server sea un http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	withCORS(s.mux).ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/services", s.handleListServices)
	s.mux.HandleFunc("POST /api/services", s.handleCreateService)
	s.mux.HandleFunc("GET /api/services/{id}/checks", s.handleListChecks)
}

// ---------- tipos de respuesta (DTOs) ----------

type serviceStatusResponse struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	LastCheckedAt  *time.Time `json:"last_checked_at"`
	LastStatusCode *int       `json:"last_status_code"`
	LastLatencyMs  *int       `json:"last_latency_ms"`
	IsUp           *bool      `json:"is_up"`
	Uptime24h      float64    `json:"uptime_24h"`
	Checks24h      int        `json:"checks_24h"`
}

type checkResponse struct {
	CheckedAt  time.Time `json:"checked_at"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int       `json:"latency_ms"`
	IsUp       bool      `json:"is_up"`
	Error      string    `json:"error,omitempty"`
}

type createServiceRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ---------- handlers ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "base de datos no disponible")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	statuses, err := s.store.ListServiceStatus(r.Context())
	if err != nil {
		log.Printf("listando servicios: %v", err)
		writeError(w, http.StatusInternalServerError, "error interno")
		return
	}

	out := make([]serviceStatusResponse, 0, len(statuses))
	for _, st := range statuses {
		out = append(out, serviceStatusResponse{
			ID:             st.ID,
			Name:           st.Name,
			URL:            st.URL,
			LastCheckedAt:  st.LastCheckedAt,
			LastStatusCode: st.LastStatusCode,
			LastLatencyMs:  st.LastLatencyMs,
			IsUp:           st.LastIsUp,
			Uptime24h:      round2(st.Uptime24h),
			Checks24h:      st.Checks24h,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateService(w http.ResponseWriter, r *http.Request) {
	var req createServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "el nombre es obligatorio")
		return
	}
	if !validURL(req.URL) {
		writeError(w, http.StatusBadRequest, "la URL debe ser http o https y estar bien formada")
		return
	}

	svc, err := s.store.CreateService(r.Context(), req.Name, req.URL)
	if err != nil {
		log.Printf("creando servicio: %v", err)
		writeError(w, http.StatusInternalServerError, "error interno")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":   svc.ID,
		"name": svc.Name,
		"url":  svc.URL,
	})
}

func (s *Server) handleListChecks(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}

	exists, err := s.store.ServiceExists(r.Context(), id)
	if err != nil {
		log.Printf("verificando servicio: %v", err)
		writeError(w, http.StatusInternalServerError, "error interno")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "servicio no encontrado")
		return
	}

	hours := queryInt(r, "hours", 24, 1, 720)
	limit := queryInt(r, "limit", 100, 1, 1000)

	checks, err := s.store.ListChecks(r.Context(), id, hours, limit)
	if err != nil {
		log.Printf("listando checks: %v", err)
		writeError(w, http.StatusInternalServerError, "error interno")
		return
	}

	out := make([]checkResponse, 0, len(checks))
	for _, c := range checks {
		out = append(out, checkResponse{
			CheckedAt:  c.CheckedAt,
			StatusCode: c.StatusCode,
			LatencyMs:  c.LatencyMs,
			IsUp:       c.IsUp,
			Error:      c.Error,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("escribiendo respuesta: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func validURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func queryInt(r *http.Request, key string, def, min, max int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < min || n > max {
		return def
	}
	return n
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// withCORS permite que el dashboard (que corre en otro puerto) consuma la API.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}