// Package alerts holds the business logic that turns raw fleet events into
// alerts, independent of NATS and HTTP so it can be unit tested cleanly.
package alerts

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Ping struct {
	VehicleID string    `json:"vehicle_id"`
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	SpeedKph  float64   `json:"speed_kph"`
	At        time.Time `json:"at"`
}

type Alert struct {
	Kind      string    `json:"kind"`
	VehicleID string    `json:"vehicle_id"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	At        time.Time `json:"at"`
}

// FromOverspeedEvent decodes a speed-over-limit event and builds an Alert.
// Severity scales with how far above the limit the vehicle is.
func FromOverspeedEvent(payload []byte) (Alert, error) {
	var p Ping
	if err := json.Unmarshal(payload, &p); err != nil {
		return Alert{}, fmt.Errorf("decode: %w", err)
	}
	if strings.TrimSpace(p.VehicleID) == "" {
		return Alert{}, fmt.Errorf("vehicle_id required")
	}
	sev := "warning"
	if p.SpeedKph > 140 {
		sev = "critical"
	}
	when := p.At
	if when.IsZero() {
		when = time.Now().UTC()
	}
	return Alert{
		Kind:      "overspeed",
		VehicleID: p.VehicleID,
		Severity:  sev,
		Message:   fmt.Sprintf("vehicle %s travelling at %.0f kph", p.VehicleID, p.SpeedKph),
		At:        when,
	}, nil
}
