package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mugishajc/fleetpulse/services/drivers-service/internal/store"
)

type Publisher interface {
	Publish(subject string, data []byte) error
}

type Server struct {
	Store     store.Repo
	Publisher Publisher
}

func New(s store.Repo, p Publisher) *Server { return &Server{Store: s, Publisher: p} }

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.healthz)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("POST /drivers", s.register)
	mux.HandleFunc("GET /drivers", s.list)
	mux.HandleFunc("GET /drivers/{id}", s.get)
	mux.HandleFunc("POST /drivers/{id}/assign", s.assign)
	mux.HandleFunc("POST /drivers/{id}/end-assignment", s.end)
	mux.HandleFunc("GET /vehicles/{vehicleId}/driver", s.activeForVehicle)
	return mux
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var d store.Driver
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, "bad body", 400)
		return
	}
	if d.ID == "" || d.LicenseNumber == "" || d.FullName == "" {
		http.Error(w, "id, full_name and license_number required", 400)
		return
	}
	if err := s.Store.Register(d); err != nil {
		if errors.Is(err, store.ErrDuplicateLicense) {
			http.Error(w, err.Error(), 409)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	if payload, err := json.Marshal(d); err == nil {
		_ = s.Publisher.Publish("fleet.driver.registered", payload)
	}
	writeJSON(w, 201, d)
}

func (s *Server) list(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.Store.List())
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	d, err := s.Store.Get(r.PathValue("id"))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, d)
}

type assignBody struct {
	VehicleID string `json:"vehicle_id"`
}

func (s *Server) assign(w http.ResponseWriter, r *http.Request) {
	var b assignBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.VehicleID == "" {
		http.Error(w, "vehicle_id required", 400)
		return
	}
	driverID := r.PathValue("id")
	if err := s.Store.Assign(driverID, b.VehicleID, time.Now().UTC()); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			http.Error(w, err.Error(), 404)
		case errors.Is(err, store.ErrAlreadyAssigned):
			http.Error(w, err.Error(), 409)
		default:
			http.Error(w, err.Error(), 500)
		}
		return
	}
	event := map[string]string{"driver_id": driverID, "vehicle_id": b.VehicleID}
	if payload, err := json.Marshal(event); err == nil {
		_ = s.Publisher.Publish("fleet.driver.assigned", payload)
	}
	w.WriteHeader(204)
}

func (s *Server) end(w http.ResponseWriter, r *http.Request) {
	var b assignBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.VehicleID == "" {
		http.Error(w, "vehicle_id required", 400)
		return
	}
	driverID := r.PathValue("id")
	if err := s.Store.EndAssignment(driverID, b.VehicleID, time.Now().UTC()); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) activeForVehicle(w http.ResponseWriter, r *http.Request) {
	a := s.Store.ActiveAssignmentsFor(r.PathValue("vehicleId"))
	if len(a) == 0 {
		writeJSON(w, 200, map[string]any{"active": false})
		return
	}
	writeJSON(w, 200, map[string]any{"active": true, "driver_id": a[0].DriverID, "started_at": a[0].StartedAt})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
