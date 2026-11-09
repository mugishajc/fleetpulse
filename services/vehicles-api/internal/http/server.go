// Package http wires HTTP handlers for the vehicles API.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mugishajc/fleetpulse/services/vehicles-api/internal/store"
)

// Publisher is the minimum NATS surface the HTTP layer needs. Keeping it as
// an interface lets us test without a real NATS server.
type Publisher interface {
	Publish(subject string, data []byte) error
}

type Server struct {
	Store     *store.Store
	Publisher Publisher
}

func New(s *store.Store, p Publisher) *Server { return &Server{Store: s, Publisher: p} }

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("POST /vehicles", s.register)
	mux.HandleFunc("GET /vehicles", s.list)
	mux.HandleFunc("GET /vehicles/{id}", s.get)
	mux.HandleFunc("POST /vehicles/{id}/pings", s.ping)
	return mux
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) }
func (s *Server) readyz(w http.ResponseWriter, _ *http.Request)  { writeJSON(w, 200, map[string]string{"status": "ready"}) }

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var v store.Vehicle
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	v.Plate = strings.ToUpper(strings.TrimSpace(v.Plate))
	if v.ID == "" || v.Plate == "" {
		http.Error(w, "id and plate required", http.StatusBadRequest)
		return
	}
	if err := s.Store.Register(v); err != nil {
		if errors.Is(err, store.ErrDuplicatePlate) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if payload, err := json.Marshal(v); err == nil {
		_ = s.Publisher.Publish("fleet.vehicle.registered", payload)
	}
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) list(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.Store.List())
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := s.Store.Get(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Store.Get(id); err != nil {
		http.Error(w, "unknown vehicle", http.StatusNotFound)
		return
	}
	var p store.Ping
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	p.VehicleID = id
	s.Store.RecordPing(p)

	// Speed events drive downstream alerts. Threshold is coarse on purpose.
	if p.SpeedKph > 100 {
		if payload, err := json.Marshal(p); err == nil {
			_ = s.Publisher.Publish("fleet.speed.over_limit", payload)
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
