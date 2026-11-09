// Package geo holds coordinate helpers: great-circle distance and point-in-circle
// geofence checks. All distances are in metres.
package geo

import "math"

const earthRadiusM = 6_371_000.0

// Point is a WGS-84 coordinate.
type Point struct {
	Lat float64
	Lon float64
}

// DistanceMeters returns the great-circle distance between a and b in metres
// using the haversine formula. Precise enough for city-scale fleet work.
func DistanceMeters(a, b Point) float64 {
	phi1 := toRad(a.Lat)
	phi2 := toRad(b.Lat)
	dPhi := toRad(b.Lat - a.Lat)
	dLambda := toRad(b.Lon - a.Lon)

	h := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(dLambda/2)*math.Sin(dLambda/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusM * c
}

// Geofence is a circular boundary centred on a point with a radius in metres.
type Geofence struct {
	Name     string
	Centre   Point
	RadiusM  float64
}

// Contains reports whether p lies inside or on the fence boundary.
func (g Geofence) Contains(p Point) bool {
	return DistanceMeters(g.Centre, p) <= g.RadiusM
}

func toRad(d float64) float64 { return d * math.Pi / 180 }
