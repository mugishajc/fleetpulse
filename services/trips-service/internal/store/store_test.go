package store

import (
	"testing"
	"time"
)

func TestTripAccumulatesDistance(t *testing.T) {
	s := New()
	start := time.Now().UTC()
	_, _ = s.Start("t1", "v1", "d1", start)

	for _, sample := range []Sample{
		{TripID: "t1", Lat: -1.9441, Lon: 30.0619, SpeedKph: 20, At: start.Add(0)},
		{TripID: "t1", Lat: -1.9450, Lon: 30.0625, SpeedKph: 40, At: start.Add(time.Minute)},
		{TripID: "t1", Lat: -1.9470, Lon: 30.0640, SpeedKph: 60, At: start.Add(2 * time.Minute)},
	} {
		if err := s.AppendSample(sample); err != nil {
			t.Fatal(err)
		}
	}

	got, _ := s.Get("t1")
	if got.Samples != 3 {
		t.Fatalf("want 3 samples, got %d", got.Samples)
	}
	if got.MaxSpeedKph != 60 {
		t.Fatalf("want max 60, got %f", got.MaxSpeedKph)
	}
	if got.DistanceM <= 0 {
		t.Fatal("distance should accumulate across samples")
	}
}

func TestEndedTripRejectsSamples(t *testing.T) {
	s := New()
	_, _ = s.Start("t1", "v1", "d1", time.Now().UTC())
	_, _ = s.End("t1", time.Now().UTC())
	if err := s.AppendSample(Sample{TripID: "t1", Lat: 0, Lon: 0}); err != ErrClosed {
		t.Fatalf("want ErrClosed, got %v", err)
	}
}

func TestListByVehicleReturnsNewestFirst(t *testing.T) {
	s := New()
	_, _ = s.Start("t1", "v1", "d1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	_, _ = s.Start("t2", "v1", "d1", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	got := s.ListByVehicle("v1")
	if len(got) != 2 || got[0].ID != "t2" {
		t.Fatalf("want newest first, got %+v", got)
	}
}
