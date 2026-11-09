// Small k6 smoke test that drives real traffic through all three services.
//
//   k6 run --vus 20 --duration 30s scripts/k6/smoke.js
//
// Targets default to localhost. Override with env vars.
import http from "k6/http";
import { check, sleep } from "k6";

const VEHICLES = __ENV.VEHICLES_URL || "http://localhost:8080";
const DRIVERS = __ENV.DRIVERS_URL || "http://localhost:8081";
const TRIPS = __ENV.TRIPS_URL || "http://localhost:8082";

export const options = {
  thresholds: {
    http_req_failed: ["rate<0.01"],       // fewer than 1% requests fail
    http_req_duration: ["p(95)<200"],     // 95% of requests under 200ms
  },
};

function uniq(prefix) {
  return `${prefix}-${__VU}-${__ITER}-${Date.now()}`;
}

export default function () {
  const vid = uniq("v");
  const did = uniq("d");

  const vehicleRes = http.post(
    `${VEHICLES}/vehicles`,
    JSON.stringify({ id: vid, plate: vid, make: "Toyota", model: "Hiace", owner: "k6" }),
    { headers: { "Content-Type": "application/json" } },
  );
  check(vehicleRes, { "vehicle created": (r) => r.status === 201 });

  const driverRes = http.post(
    `${DRIVERS}/drivers`,
    JSON.stringify({ id: did, full_name: "k6 driver", license_number: did, license_class: "B" }),
    { headers: { "Content-Type": "application/json" } },
  );
  check(driverRes, { "driver created": (r) => r.status === 201 });

  http.post(`${DRIVERS}/drivers/${did}/assign`, JSON.stringify({ vehicle_id: vid }), {
    headers: { "Content-Type": "application/json" },
  });

  const tid = uniq("t");
  const start = http.post(
    `${TRIPS}/trips`,
    JSON.stringify({ id: tid, vehicle_id: vid, driver_id: did }),
    { headers: { "Content-Type": "application/json" } },
  );
  check(start, { "trip started": (r) => r.status === 201 });

  for (let i = 0; i < 5; i++) {
    http.post(
      `${TRIPS}/trips/${tid}/samples`,
      JSON.stringify({ lat: -1.94 + i * 0.001, lon: 30.06 + i * 0.001, speed_kph: 50 + i * 15 }),
      { headers: { "Content-Type": "application/json" } },
    );
  }
  http.post(`${TRIPS}/trips/${tid}/end`);

  sleep(1);
}
