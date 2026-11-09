// Package store holds the driver registry and vehicle assignments.
// Read-heavy in-memory store with a repo interface so swapping to Postgres
// is a drop-in change.
package store

import (
	"errors"
	"sync"
	"time"
)

type Driver struct {
	ID            string    `json:"id"`
	FullName      string    `json:"full_name"`
	LicenseNumber string    `json:"license_number"`
	LicenseClass  string    `json:"license_class"`
	Phone         string    `json:"phone"`
	HiredOn       time.Time `json:"hired_on"`
	CreatedAt     time.Time `json:"created_at"`
}

type Assignment struct {
	DriverID  string    `json:"driver_id"`
	VehicleID string    `json:"vehicle_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

var (
	ErrNotFound         = errors.New("driver not found")
	ErrDuplicateLicense = errors.New("license already registered")
	ErrAlreadyAssigned  = errors.New("vehicle already has an active driver")
)

type Repo interface {
	Register(Driver) error
	Get(id string) (Driver, error)
	List() []Driver
	Assign(driverID, vehicleID string, at time.Time) error
	EndAssignment(driverID, vehicleID string, at time.Time) error
	ActiveAssignmentsFor(vehicleID string) []Assignment
}

type Memory struct {
	mu          sync.RWMutex
	drivers     map[string]*Driver
	licenses    map[string]string // licenseNumber -> driverID
	assignments []Assignment
}

func NewMemory() *Memory {
	return &Memory{
		drivers:  make(map[string]*Driver),
		licenses: make(map[string]string),
	}
}

func (s *Memory) Register(d Driver) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.licenses[d.LicenseNumber]; ok {
		return ErrDuplicateLicense
	}
	cp := d
	cp.CreatedAt = time.Now().UTC()
	s.drivers[cp.ID] = &cp
	s.licenses[cp.LicenseNumber] = cp.ID
	return nil
}

func (s *Memory) Get(id string) (Driver, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.drivers[id]
	if !ok {
		return Driver{}, ErrNotFound
	}
	return *d, nil
}

func (s *Memory) List() []Driver {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Driver, 0, len(s.drivers))
	for _, d := range s.drivers {
		out = append(out, *d)
	}
	return out
}

func (s *Memory) Assign(driverID, vehicleID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.drivers[driverID]; !ok {
		return ErrNotFound
	}
	for _, a := range s.assignments {
		if a.VehicleID == vehicleID && a.EndedAt.IsZero() {
			return ErrAlreadyAssigned
		}
	}
	s.assignments = append(s.assignments, Assignment{
		DriverID: driverID, VehicleID: vehicleID, StartedAt: at,
	})
	return nil
}

func (s *Memory) EndAssignment(driverID, vehicleID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.assignments {
		a := &s.assignments[i]
		if a.DriverID == driverID && a.VehicleID == vehicleID && a.EndedAt.IsZero() {
			a.EndedAt = at
			return nil
		}
	}
	return ErrNotFound
}

func (s *Memory) ActiveAssignmentsFor(vehicleID string) []Assignment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Assignment
	for _, a := range s.assignments {
		if a.VehicleID == vehicleID && a.EndedAt.IsZero() {
			out = append(out, a)
		}
	}
	return out
}
