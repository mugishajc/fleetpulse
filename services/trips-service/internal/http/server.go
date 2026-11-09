package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mugishajc/fleetpulse/services/trips-service/internal/geo"
	"github.com/mugishajc/fleetpulse/services/trips-service/internal/store"
)

type Publisher interface {
	Publish(subject string, data []byte) error
}

type Server struct {
	Store     *store.Store
	Publisher Publisher
	Fences    []geo.Geofence // read-only once set
}

func New(s *store.Store, p Publisher, fences []geo.Geofence) *Server {
	return &Server{Store: s, Publisher: p, Fences: fences}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ready"}) })
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("POST /trips", s.start)
	mux.HandleFunc("POST /trips/{id}/samples", s.addSample)
	mux.HandleFunc("POST /trips/{id}/end", s.end)
	mux.HandleFunc("GET /trips/{id}", s.get)
	mux.HandleFunc("GET /vehicles/{vehicleId}/trips", s.listByVehicle)
	return mux
}

type startBody struct {
	ID        string `json:"id"`
	VehicleID string `json:"vehicle_id"`
	DriverID  string `json:"driver_id"`
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	var b startBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.ID == "" || b.VehicleID == "" {
		http.Error(w, "id and vehicle_id required", 400)
		return
	}
	t, _ := s.Store.Start(b.ID, b.VehicleID, b.DriverID, time.Now().UTC())
	if payload, err := json.Marshal(t); err == nil {
		_ = s.Publisher.Publish("fleet.trip.started", payload)
	}
	writeJSON(w, 201, t)
}

func (s *Server) addSample(w http.ResponseWriter, r *http.Request) {
	var sample store.Sample
	if err := json.NewDecoder(r.Body).Decode(&sample); err != nil {
		http.Error(w, "bad body", 400)
		return
	}
	sample.TripID = r.PathValue("id")
	if sample.At.IsZero() {
		sample.At = time.Now().UTC()
	}
	if err := s.Store.AppendSample(sample); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			http.Error(w, err.Error(), 404)
		case errors.Is(err, store.ErrClosed):
			http.Error(w, err.Error(), 409)
		default:
			http.Error(w, err.Error(), 500)
		}
		return
	}

	// Geofence breach check. Emit an event for each fence the point sits outside
	// relative to its name. In production we'd also track enter/exit transitions.
	point := geo.Point{Lat: sample.Lat, Lon: sample.Lon}
	for _, f := range s.Fences {
		if !f.Contains(point) {
			event := map[string]any{
				"trip_id":  sample.TripID,
				"fence":    f.Name,
				"lat":      point.Lat,
				"lon":      point.Lon,
				"detected": sample.At,
			}
			if payload, err := json.Marshal(event); err == nil {
				_ = s.Publisher.Publish("fleet.trip.geofence_breach", payload)
			}
		}
	}
	w.WriteHeader(202)
}

func (s *Server) end(w http.ResponseWriter, r *http.Request) {
	t, err := s.Store.End(r.PathValue("id"), time.Now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, err.Error(), 404)
		} else {
			http.Error(w, err.Error(), 409)
		}
		return
	}
	if payload, err := json.Marshal(t); err == nil {
		_ = s.Publisher.Publish("fleet.trip.ended", payload)
	}
	writeJSON(w, 200, t)
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	t, err := s.Store.Get(r.PathValue("id"))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, t)
}

func (s *Server) listByVehicle(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.Store.ListByVehicle(r.PathValue("vehicleId")))
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
