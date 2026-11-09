// Package store holds the in-memory vehicle registry.
//
// A real deployment would back this with PostgreSQL; the interface is kept
// small enough that swapping it out is straightforward.
package store

import (
	"errors"
	"sync"
	"time"
)

type Vehicle struct {
	ID        string    `json:"id"`
	Plate     string    `json:"plate"`
	Make      string    `json:"make"`
	Model     string    `json:"model"`
	Owner     string    `json:"owner"`
	CreatedAt time.Time `json:"created_at"`
}

type Ping struct {
	VehicleID string    `json:"vehicle_id"`
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	SpeedKph  float64   `json:"speed_kph"`
	At        time.Time `json:"at"`
}

var ErrNotFound = errors.New("vehicle not found")
var ErrDuplicatePlate = errors.New("plate already registered")

type Store struct {
	mu         sync.RWMutex
	byID       map[string]*Vehicle
	plates     map[string]string // plate -> id
	lastPingAt map[string]time.Time
}

func New() *Store {
	return &Store{
		byID:       make(map[string]*Vehicle),
		plates:     make(map[string]string),
		lastPingAt: make(map[string]time.Time),
	}
}

func (s *Store) Register(v Vehicle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.plates[v.Plate]; exists {
		return ErrDuplicatePlate
	}
	cp := v
	cp.CreatedAt = time.Now().UTC()
	s.byID[cp.ID] = &cp
	s.plates[cp.Plate] = cp.ID
	return nil
}

func (s *Store) Get(id string) (Vehicle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.byID[id]
	if !ok {
		return Vehicle{}, ErrNotFound
	}
	return *v, nil
}

func (s *Store) List() []Vehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Vehicle, 0, len(s.byID))
	for _, v := range s.byID {
		out = append(out, *v)
	}
	return out
}

func (s *Store) RecordPing(p Ping) {
	s.mu.Lock()
	s.lastPingAt[p.VehicleID] = p.At
	s.mu.Unlock()
}

func (s *Store) LastSeen(id string) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.lastPingAt[id]
	return t, ok
}
