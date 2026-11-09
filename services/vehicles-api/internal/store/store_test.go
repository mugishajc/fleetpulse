package store

import (
	"testing"
	"time"
)

func TestRegisterAndGet(t *testing.T) {
	s := New()
	v := Vehicle{ID: "v1", Plate: "RAD123A", Make: "Toyota", Model: "Hiace", Owner: "Jean"}
	if err := s.Register(v); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, err := s.Get("v1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Plate != "RAD123A" || got.Make != "Toyota" {
		t.Fatalf("unexpected vehicle: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set")
	}
}

func TestDuplicatePlateRejected(t *testing.T) {
	s := New()
	_ = s.Register(Vehicle{ID: "a", Plate: "RAD999Z"})
	if err := s.Register(Vehicle{ID: "b", Plate: "RAD999Z"}); err != ErrDuplicatePlate {
		t.Fatalf("expected ErrDuplicatePlate, got %v", err)
	}
}

func TestUnknownIdIsNotFound(t *testing.T) {
	s := New()
	_, err := s.Get("nope")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRecordPingUpdatesLastSeen(t *testing.T) {
	s := New()
	now := time.Now().UTC()
	s.RecordPing(Ping{VehicleID: "v1", Lat: -1.95, Lon: 30.06, At: now})
	ts, ok := s.LastSeen("v1")
	if !ok || !ts.Equal(now) {
		t.Fatalf("expected last seen %v, got %v ok=%v", now, ts, ok)
	}
}

func TestListReturnsAll(t *testing.T) {
	s := New()
	_ = s.Register(Vehicle{ID: "1", Plate: "A1"})
	_ = s.Register(Vehicle{ID: "2", Plate: "A2"})
	if n := len(s.List()); n != 2 {
		t.Fatalf("expected 2 vehicles, got %d", n)
	}
}
