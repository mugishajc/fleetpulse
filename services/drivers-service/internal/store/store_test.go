package store

import (
	"testing"
	"time"
)

func TestRegisterAndGet(t *testing.T) {
	s := NewMemory()
	d := Driver{ID: "d1", FullName: "Uwase Alice", LicenseNumber: "RW-123", LicenseClass: "B"}
	if err := s.Register(d); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("d1")
	if err != nil || got.LicenseNumber != "RW-123" {
		t.Fatalf("unexpected get: %+v err=%v", got, err)
	}
}

func TestDuplicateLicenseRejected(t *testing.T) {
	s := NewMemory()
	_ = s.Register(Driver{ID: "a", LicenseNumber: "RW-9"})
	if err := s.Register(Driver{ID: "b", LicenseNumber: "RW-9"}); err != ErrDuplicateLicense {
		t.Fatalf("want ErrDuplicateLicense, got %v", err)
	}
}

func TestAssignRejectsDoubleBooking(t *testing.T) {
	s := NewMemory()
	_ = s.Register(Driver{ID: "d1", LicenseNumber: "A"})
	_ = s.Register(Driver{ID: "d2", LicenseNumber: "B"})
	now := time.Now().UTC()
	if err := s.Assign("d1", "v1", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Assign("d2", "v1", now); err != ErrAlreadyAssigned {
		t.Fatalf("expected ErrAlreadyAssigned, got %v", err)
	}
}

func TestEndAssignmentFreesVehicle(t *testing.T) {
	s := NewMemory()
	_ = s.Register(Driver{ID: "d1", LicenseNumber: "A"})
	_ = s.Register(Driver{ID: "d2", LicenseNumber: "B"})
	now := time.Now().UTC()
	_ = s.Assign("d1", "v1", now)
	if err := s.EndAssignment("d1", "v1", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Assign("d2", "v1", now); err != nil {
		t.Fatalf("after end, re-assignment should succeed: %v", err)
	}
}
