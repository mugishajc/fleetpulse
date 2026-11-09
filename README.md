# fleetpulse

Event-driven fleet monitoring on Kubernetes. Two Go microservices talk through NATS pub/sub. A GPS ping that crosses a speed threshold becomes an alert in another service without the two ever calling each other over HTTP.

Built as a working example of how I lay out a production microservice system: service boundaries by domain, async messaging for cross-service workflows, Prometheus metrics on every pod, Kubernetes manifests with Kustomize overlays per environment, a Helm chart for single-command installs, and Argo CD applications so `main` is what the cluster runs.

## Services

```
services/
  vehicles-api/    Go REST API: register vehicles, record GPS pings, publishes events to NATS
  alerts-worker/   Go worker: subscribes to speed events and produces Alerts, exposes /metrics
```

Events on NATS:

- `fleet.vehicle.registered` emitted when a vehicle is created
- `fleet.speed.over_limit` emitted when a ping arrives with speed > 100 kph

## Run it locally

```bash
docker compose up --build
# register a vehicle
curl -X POST localhost:8080/vehicles \
  -H 'content-type: application/json' \
  -d '{"id":"v1","plate":"RAD123A","make":"Toyota","model":"Hiace","owner":"Jean"}'
# send a ping above the limit
curl -X POST localhost:8080/vehicles/v1/pings \
  -H 'content-type: application/json' \
  -d '{"lat":-1.95,"lon":30.06,"speed_kph":135}'
# alerts-worker logs the alert, and fleet_alerts_total is scrapable on :9090/metrics
```

## Tests

Each service runs independently. The HTTP server is unit-tested against a fake NATS publisher so no broker is needed:

```bash
cd services/vehicles-api && go test -race ./...
cd services/alerts-worker && go test -race ./...
```

Covered:

- store: register, duplicate-plate rejection, not-found, ping-updates-last-seen
- HTTP: register + retrieve, overspeed ping publishes an event, normal speed does not, ping on unknown vehicle is 404
- alerts: overspeed warning vs critical by speed, missing vehicle id is rejected, bad JSON is rejected

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

Staging is auto-synced with prune and self-heal. Production is manual: promote by bumping the image tag in the production overlay and merging to `main`.

## CI/CD

`ci.yml` runs on every PR:
- `go vet`, `go test -race` for each service
- `kubeconform` against both overlays
- `helm lint` on the chart

`build-and-push.yml` runs on `main`:
- builds each service with Buildx
- tags images with the commit SHA
- pushes to `ghcr.io/mugishajc/fleetpulse-<service>`

## Design notes

- NATS over HTTP between services. A vehicle can be registered while the alerts worker is down; it will see the event later. HTTP would couple their uptimes.
- Queue-group subscription on `alerts-workers`, so scaling the worker to N replicas does not cause N copies of every alert.
- `/metrics`, `/healthz` and `/readyz` on every Pod. Readiness gates a Pod out of the Service while startup is still in progress.
- Non-root containers, read-only root filesystem, dropped capabilities, resource requests and limits on every workload.
