package geo

import (
	"math"
	"testing"
)

func TestDistanceBetweenSamePointIsZero(t *testing.T) {
	p := Point{Lat: -1.95, Lon: 30.06}
	if d := DistanceMeters(p, p); d != 0 {
		t.Fatalf("want 0, got %f", d)
	}
}

// Kigali (Nyabugogo) to Butare (Huye) is about 133 km. Allow a 2 % margin.
func TestKigaliToButareIsAbout133km(t *testing.T) {
	kigali := Point{Lat: -1.9441, Lon: 30.0619}
	butare := Point{Lat: -2.6038, Lon: 29.7392}
	d := DistanceMeters(kigali, butare)
	expected := 133_000.0
	if math.Abs(d-expected)/expected > 0.02 {
		t.Fatalf("expected ~%f m, got %f m", expected, d)
	}
}

func TestGeofenceContains(t *testing.T) {
	g := Geofence{Name: "Kigali CBD", Centre: Point{Lat: -1.9441, Lon: 30.0619}, RadiusM: 2000}
	inside := Point{Lat: -1.9450, Lon: 30.0625}
	outside := Point{Lat: -1.9700, Lon: 30.0900}
	if !g.Contains(inside) {
		t.Fatal("point just south of centre should be inside a 2 km fence")
	}
	if g.Contains(outside) {
		t.Fatal("point several km away should be outside")
	}
}
