package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mugishajc/fleetpulse/services/vehicles-api/internal/store"
)

type fakePublisher struct {
	mu   sync.Mutex
	msgs []struct {
		subject string
		data    []byte
	}
}

func (f *fakePublisher) Publish(subject string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, struct {
		subject string
		data    []byte
	}{subject, append([]byte(nil), data...)})
	return nil
}

func newTestServer() (*Server, *fakePublisher) {
	p := &fakePublisher{}
	return New(store.New(), p), p
}

func doJSON(h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	buf := &bytes.Buffer{}
	if body != nil {
		_ = json.NewEncoder(buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, buf)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRegisterAndRetrieveVehicle(t *testing.T) {
	s, pub := newTestServer()
	h := s.Routes()

	w := doJSON(h, "POST", "/vehicles", map[string]string{"id": "v1", "plate": "RAD123A", "make": "Toyota"})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body)
	}

	if len(pub.msgs) != 1 || pub.msgs[0].subject != "fleet.vehicle.registered" {
		t.Fatalf("expected one vehicle.registered event, got %+v", pub.msgs)
	}

	w = doJSON(h, "GET", "/vehicles/v1", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "RAD123A") {
		t.Fatalf("get: %d %s", w.Code, w.Body)
	}
}

func TestOverspeedPublishesAlert(t *testing.T) {
	s, pub := newTestServer()
	h := s.Routes()
	_ = doJSON(h, "POST", "/vehicles", map[string]string{"id": "v1", "plate": "P1"})

	w := doJSON(h, "POST", "/vehicles/v1/pings", map[string]any{"lat": 0, "lon": 0, "speed_kph": 120})
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}

	var subjects []string
	for _, m := range pub.msgs {
		subjects = append(subjects, m.subject)
	}
	found := false
	for _, s := range subjects {
		if s == "fleet.speed.over_limit" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fleet.speed.over_limit event, got %v", subjects)
	}
}

func TestNormalSpeedDoesNotAlert(t *testing.T) {
	s, pub := newTestServer()
	h := s.Routes()
	_ = doJSON(h, "POST", "/vehicles", map[string]string{"id": "v1", "plate": "P1"})

	_ = doJSON(h, "POST", "/vehicles/v1/pings", map[string]any{"speed_kph": 60})
	for _, m := range pub.msgs {
		if m.subject == "fleet.speed.over_limit" {
			t.Fatalf("did not expect over-limit event at 60 kph")
		}
	}
}

func TestPingUnknownVehicleIs404(t *testing.T) {
	s, _ := newTestServer()
	w := doJSON(s.Routes(), "POST", "/vehicles/ghost/pings", map[string]any{"speed_kph": 50})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
