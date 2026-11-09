// Package store holds trip state and route samples. In-memory with a repo
// interface so a PostgreSQL implementation can take over.
package store

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/mugishajc/fleetpulse/services/trips-service/internal/geo"
)

type Trip struct {
	ID          string     `json:"id"`
	VehicleID   string     `json:"vehicle_id"`
	DriverID    string     `json:"driver_id"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
	DistanceM   float64    `json:"distance_m"`
	MaxSpeedKph float64    `json:"max_speed_kph"`
	Samples     int        `json:"samples"`
}

type Sample struct {
	TripID   string    `json:"trip_id"`
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
	SpeedKph float64   `json:"speed_kph"`
	At       time.Time `json:"at"`
}

var (
	ErrNotFound = errors.New("trip not found")
	ErrClosed   = errors.New("trip already ended")
)

type Store struct {
	mu      sync.RWMutex
	trips   map[string]*Trip
	samples map[string][]Sample
	last    map[string]geo.Point
}

func New() *Store {
	return &Store{
		trips:   make(map[string]*Trip),
		samples: make(map[string][]Sample),
		last:    make(map[string]geo.Point),
	}
}

func (s *Store) Start(id, vehicleID, driverID string, at time.Time) (*Trip, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := &Trip{ID: id, VehicleID: vehicleID, DriverID: driverID, StartedAt: at}
	s.trips[id] = t
	return t, nil
}

func (s *Store) AppendSample(sample Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.trips[sample.TripID]
	if !ok {
		return ErrNotFound
	}
	if t.EndedAt != nil {
		return ErrClosed
	}
	if prev, ok := s.last[sample.TripID]; ok {
		t.DistanceM += geo.DistanceMeters(prev, geo.Point{Lat: sample.Lat, Lon: sample.Lon})
	}
	s.last[sample.TripID] = geo.Point{Lat: sample.Lat, Lon: sample.Lon}
	if sample.SpeedKph > t.MaxSpeedKph {
		t.MaxSpeedKph = sample.SpeedKph
	}
	t.Samples++
	s.samples[sample.TripID] = append(s.samples[sample.TripID], sample)
	return nil
}

func (s *Store) End(id string, at time.Time) (Trip, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.trips[id]
	if !ok {
		return Trip{}, ErrNotFound
	}
	if t.EndedAt != nil {
		return Trip{}, ErrClosed
	}
	t.EndedAt = &at
	return *t, nil
}

func (s *Store) Get(id string) (Trip, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.trips[id]
	if !ok {
		return Trip{}, ErrNotFound
	}
	return *t, nil
}

func (s *Store) ListByVehicle(vehicleID string) []Trip {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Trip{}
	for _, t := range s.trips {
		if t.VehicleID == vehicleID {
			out = append(out, *t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}
