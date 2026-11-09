package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mugishajc/fleetpulse/services/drivers-service/internal/store"
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

func newServer() (*Server, *fakePub) {
	p := &fakePub{}
	return New(store.NewMemory(), p), p
}

func post(h http.Handler, path string, body any) *httptest.ResponseRecorder {
	buf := &bytes.Buffer{}
	_ = json.NewEncoder(buf).Encode(body)
	r := httptest.NewRequest("POST", path, buf)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestFullAssignmentFlow(t *testing.T) {
	s, pub := newServer()
	h := s.Routes()

	if w := post(h, "/drivers", map[string]string{"id": "d1", "full_name": "Alice", "license_number": "RW1"}); w.Code != 201 {
		t.Fatalf("register: %d %s", w.Code, w.Body)
	}
	if w := post(h, "/drivers/d1/assign", map[string]string{"vehicle_id": "v9"}); w.Code != 204 {
		t.Fatalf("assign: %d %s", w.Code, w.Body)
	}
	if w := get(h, "/vehicles/v9/driver"); w.Code != 200 || !strings.Contains(w.Body.String(), "\"active\":true") {
		t.Fatalf("active lookup: %d %s", w.Code, w.Body)
	}
	if w := post(h, "/drivers/d1/end-assignment", map[string]string{"vehicle_id": "v9"}); w.Code != 204 {
		t.Fatalf("end: %d %s", w.Code, w.Body)
	}

	wanted := map[string]bool{"fleet.driver.registered": false, "fleet.driver.assigned": false}
	for _, sub := range pub.subs {
		if _, ok := wanted[sub]; ok {
			wanted[sub] = true
		}
	}
	for k, v := range wanted {
		if !v {
			t.Fatalf("expected event %s", k)
		}
	}
}

func TestDoubleAssignIs409(t *testing.T) {
	s, _ := newServer()
	h := s.Routes()
	_ = post(h, "/drivers", map[string]string{"id": "a", "full_name": "A", "license_number": "1"})
	_ = post(h, "/drivers", map[string]string{"id": "b", "full_name": "B", "license_number": "2"})
	_ = post(h, "/drivers/a/assign", map[string]string{"vehicle_id": "v1"})
	if w := post(h, "/drivers/b/assign", map[string]string{"vehicle_id": "v1"}); w.Code != 409 {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestDuplicateLicenseIs409(t *testing.T) {
	s, _ := newServer()
	h := s.Routes()
	_ = post(h, "/drivers", map[string]string{"id": "a", "full_name": "A", "license_number": "X"})
	if w := post(h, "/drivers", map[string]string{"id": "b", "full_name": "B", "license_number": "X"}); w.Code != 409 {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}
