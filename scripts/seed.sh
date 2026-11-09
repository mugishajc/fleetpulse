#!/usr/bin/env bash
# Seed fleetpulse with 50 vehicles, 20 drivers, and 200 trips of GPS samples.
# Assumes vehicles-api on :8080, drivers-service on :8081, trips-service on :8082.
set -euo pipefail

VEHICLES_URL="${VEHICLES_URL:-http://localhost:8080}"
DRIVERS_URL="${DRIVERS_URL:-http://localhost:8081}"
TRIPS_URL="${TRIPS_URL:-http://localhost:8082}"

jq_available() { command -v jq >/dev/null; }
jq_available || { echo "jq is required"; exit 1; }

echo "Seeding 50 vehicles..."
for i in $(seq 1 50); do
  plate=$(printf "RAD%03dA" "$i")
  curl -sf -X POST "$VEHICLES_URL/vehicles" -H content-type:application/json -d @- >/dev/null <<EOF
{"id":"v$i","plate":"$plate","make":"Toyota","model":"Hiace","owner":"owner-$i"}
EOF
done

echo "Seeding 20 drivers..."
for i in $(seq 1 20); do
  curl -sf -X POST "$DRIVERS_URL/drivers" -H content-type:application/json -d @- >/dev/null <<EOF
{"id":"d$i","full_name":"Driver $i","license_number":"RW-LIC-$i","license_class":"B","phone":"078000$i"}
EOF
done

echo "Assigning each driver to a vehicle..."
for i in $(seq 1 20); do
  curl -sf -X POST "$DRIVERS_URL/drivers/d$i/assign" -H content-type:application/json -d "{\"vehicle_id\":\"v$i\"}" >/dev/null
done

echo "Starting and sampling 200 trips..."
for t in $(seq 1 200); do
  v=$((RANDOM % 20 + 1))
  curl -sf -X POST "$TRIPS_URL/trips" -H content-type:application/json -d "{\"id\":\"t$t\",\"vehicle_id\":\"v$v\",\"driver_id\":\"d$v\"}" >/dev/null
  # 10 GPS samples per trip, walking away from Kigali centre
  for s in $(seq 1 10); do
    lat=$(awk "BEGIN {print -1.9441 + ($s * 0.001 * ($t % 3 - 1))}")
    lon=$(awk "BEGIN {print 30.0619 + ($s * 0.001)}")
    speed=$((40 + RANDOM % 70))
    curl -sf -X POST "$TRIPS_URL/trips/t$t/samples" -H content-type:application/json \
      -d "{\"lat\":$lat,\"lon\":$lon,\"speed_kph\":$speed}" >/dev/null
  done
  curl -sf -X POST "$TRIPS_URL/trips/t$t/end" >/dev/null
done

echo "Done. Totals:"
echo "  vehicles: $(curl -sf "$VEHICLES_URL/vehicles" | jq length)"
echo "  drivers:  $(curl -sf "$DRIVERS_URL/drivers"   | jq length)"
echo "  trips:    $(curl -sf "$TRIPS_URL/vehicles/v1/trips" | jq length) (for vehicle v1)"
