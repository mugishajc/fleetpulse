package alerts

import "testing"

func TestOverspeedWarning(t *testing.T) {
	a, err := FromOverspeedEvent([]byte(`{"vehicle_id":"v1","speed_kph":120}`))
	if err != nil {
		t.Fatal(err)
	}
	if a.Severity != "warning" || a.Kind != "overspeed" {
		t.Fatalf("unexpected alert: %+v", a)
	}
}

func TestOverspeedCritical(t *testing.T) {
	a, _ := FromOverspeedEvent([]byte(`{"vehicle_id":"v9","speed_kph":160}`))
	if a.Severity != "critical" {
		t.Fatalf("expected critical, got %s", a.Severity)
	}
}

func TestMissingVehicleIsAnError(t *testing.T) {
	_, err := FromOverspeedEvent([]byte(`{"speed_kph":200}`))
	if err == nil {
		t.Fatal("expected error for missing vehicle_id")
	}
}

func TestBadJsonIsAnError(t *testing.T) {
	if _, err := FromOverspeedEvent([]byte(`{bad`)); err == nil {
		t.Fatal("expected JSON error")
	}
}
