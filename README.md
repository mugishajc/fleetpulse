# fleetpulse

Event-driven fleet operations on Kubernetes. Four Go microservices, communicating through NATS pub/sub, deployed through Kustomize overlays or a Helm chart, reconciled by Argo CD. Observable in Prometheus and Grafana. Loadable with k6.

Built as a working reference of how I ship microservice systems: domain-aligned service boundaries, asynchronous messaging between services, metrics and health endpoints on every pod, infrastructure-as-code from the base manifests up to the GitOps application, CI that validates everything on every PR.

## Services

| Service | Purpose | Port |
|---|---|---|
| vehicles-api | vehicle registry, GPS ping ingestion, publishes `fleet.vehicle.registered` and `fleet.speed.over_limit` | 8080 |
| drivers-service | driver registry, vehicle assignments, publishes `fleet.driver.registered` and `fleet.driver.assigned` | 8080 |
| trips-service | trip state, GPS sample stream with distance accumulation, geofence breach detection, publishes `fleet.trip.*` | 8080 |
| alerts-worker | subscribes to speed and trip events, produces alerts, exposes `fleet_alerts_total` metric | 9090 |

## Why NATS and not HTTP between services

Any service can be down while another service accepts work. A trip can be started while the alerts worker is being redeployed; the alert will be produced when the worker comes back. A sync HTTP call would couple their uptimes.

The alerts worker subscribes to a queue group, so scaling it to N replicas does not produce N copies of each alert.

## Geofencing

trips-service carries a small `geo` package with the haversine distance formula and a circular `Geofence.Contains`. Default fence is a 12 km radius around Kigali's CBD. Each GPS sample is checked; a point outside the fence publishes `fleet.trip.geofence_breach`. Tested against known distances (Kigali to Butare is ~133 km).

## Run it locally

```bash
docker compose up --build
./scripts/seed.sh        # 50 vehicles, 20 drivers, 200 trips
k6 run --vus 20 --duration 30s scripts/k6/smoke.js
```

Endpoints, abbreviated:

```
# vehicles-api on :8080
POST /vehicles                    register
POST /vehicles/{id}/pings         record a GPS ping

# drivers-service on :8081
POST /drivers
POST /drivers/{id}/assign         body: {"vehicle_id": "..."}
POST /drivers/{id}/end-assignment
GET  /vehicles/{vehicleId}/driver

# trips-service on :8082
POST /trips                       body: {"id","vehicle_id","driver_id"}
POST /trips/{id}/samples
POST /trips/{id}/end
GET  /vehicles/{vehicleId}/trips
```

Each service also exposes `/healthz`, `/readyz` and `/metrics`.

## Tests

Each service is independently tested. Fakes stand in for NATS so no broker is needed.

```bash
cd services/vehicles-api    && go test -race ./...
cd services/drivers-service && go test -race ./...
cd services/trips-service   && go test -race ./...
cd services/alerts-worker   && go test -race ./...
```

What is covered:

- vehicles-api: store round-trips, duplicate-plate rejection, overspeed publishes an event, normal speed does not, ping on unknown vehicle is 404
- drivers-service: register and retrieve, duplicate license is 409, double booking a vehicle is 409, end-assignment frees the vehicle
- trips-service: start sample end flow, geofence breach event, samples on an ended trip are 409, distance accumulates across samples, max speed is tracked
- geo: zero distance for same point, Kigali to Butare is ~133 km, geofence Contains with inside and outside points
- alerts-worker: overspeed warning vs critical by threshold, missing vehicle id rejected, bad JSON rejected

## Deploy

Kustomize overlays:

```bash
kubectl kustomize deploy/k8s/overlays/staging
kubectl kustomize deploy/k8s/overlays/production
```

Helm:

```bash
helm install fleetpulse deploy/helm/fleetpulse -n fleetpulse --create-namespace
```

GitOps with Argo CD:

```bash
kubectl apply -f deploy/argocd/application.yaml
```

Staging is auto-synced with prune and self-heal. Production is manual. Promote a tag by updating `deploy/k8s/overlays/production/kustomization.yaml` and merging to `main`.

## Observability

Every pod exposes `/metrics` in Prometheus text format and carries the `prometheus.io/scrape` annotation. The Grafana dashboard at `observability/grafana/fleetpulse-dashboard.json` includes:

- request rate per service
- alerts produced per minute by severity
- p95 HTTP latency per service
- active over-limit alerts in the last 5 minutes

## Load testing

`scripts/k6/smoke.js` drives real traffic through the whole system: register vehicle, register driver, assign, start trip, 5 GPS samples, end trip. Thresholds fail the run if more than 1 % of requests fail or p95 latency goes above 200 ms.

## CI/CD

`ci.yml` runs on every PR:
- `go vet`, `go test -race` for each of the four services
- `kubeconform` against both overlays
- `helm lint` on the chart

`build-and-push.yml` runs on `main`:
- builds each service with Buildx
- pushes to `ghcr.io/mugishajc/fleetpulse-<service>` tagged with the commit SHA

## Design notes

- Each service owns its data. No cross-service database reads.
- Non-root containers, read-only root filesystem, `drop: [ALL]` capabilities, resource requests and limits on every workload.
- Queue-group NATS subscriptions, not fanout, on consumers that scale horizontally.
- `/metrics`, `/healthz`, `/readyz` on every pod; readiness gates the Service while startup is still in progress.
