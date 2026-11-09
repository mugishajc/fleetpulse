package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mugishajc/fleetpulse/services/trips-service/internal/geo"
	"github.com/mugishajc/fleetpulse/services/trips-service/internal/store"
)

type fakePub struct {
	mu   sync.Mutex
	subs []string
}

func (f *fakePub) Publish(subject string, _ []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subs = append(f.subs, subject)
	return nil
}

func post(h http.Handler, path string, body any) *httptest.ResponseRecorder {
	buf := &bytes.Buffer{}
	_ = json.NewEncoder(buf).Encode(body)
	r := httptest.NewRequest("POST", path, buf)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestStartSampleEndFlow(t *testing.T) {
	fence := geo.Geofence{Name: "Kigali", Centre: geo.Point{Lat: -1.9441, Lon: 30.0619}, RadiusM: 5000}
	s := New(store.New(), &fakePub{}, []geo.Geofence{fence})
	h := s.Routes()

	if w := post(h, "/trips", map[string]string{"id": "t1", "vehicle_id": "v1", "driver_id": "d1"}); w.Code != 201 {
		t.Fatalf("start: %d %s", w.Code, w.Body)
	}
	if w := post(h, "/trips/t1/samples", map[string]any{"lat": -1.9450, "lon": 30.0625, "speed_kph": 40}); w.Code != 202 {
		t.Fatalf("sample: %d %s", w.Code, w.Body)
	}
	if w := post(h, "/trips/t1/end", nil); w.Code != 200 {
		t.Fatalf("end: %d %s", w.Code, w.Body)
	}
}

func TestOutsideFencePublishesBreach(t *testing.T) {
	pub := &fakePub{}
	fence := geo.Geofence{Name: "Kigali", Centre: geo.Point{Lat: -1.9441, Lon: 30.0619}, RadiusM: 2000}
	s := New(store.New(), pub, []geo.Geofence{fence})
	h := s.Routes()

	_ = post(h, "/trips", map[string]string{"id": "t2", "vehicle_id": "v1", "driver_id": "d1"})
	_ = post(h, "/trips/t2/samples", map[string]any{"lat": -2.6038, "lon": 29.7392, "speed_kph": 80}) // Butare, far from Kigali

	found := false
	for _, s := range pub.subs {
		if s == "fleet.trip.geofence_breach" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected geofence_breach event, got %v", pub.subs)
	}
}

func TestSampleOnEndedTripIs409(t *testing.T) {
	s := New(store.New(), &fakePub{}, nil)
	h := s.Routes()
	_ = post(h, "/trips", map[string]string{"id": "t3", "vehicle_id": "v1"})
	_ = post(h, "/trips/t3/end", nil)
	if w := post(h, "/trips/t3/samples", map[string]any{"lat": 0, "lon": 0}); w.Code != 409 {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}
