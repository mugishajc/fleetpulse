// Alerts worker: subscribes to fleet speed events on NATS and logs an Alert
// per event. A production version would push to a notifications service or
// a persistent alerts store; the shape of the code stays the same.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	alerts "github.com/mugishajc/fleetpulse/services/alerts-worker/internal"
)

var alertsCount = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "fleet_alerts_total",
	Help: "Alerts produced by severity",
}, []string{"severity"})

func main() {
	natsURL := getenv("NATS_URL", nats.DefaultURL)
	metricsAddr := getenv("METRICS_ADDR", ":9090")

	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second), nats.ReconnectWait(2*time.Second))
	if err != nil {
		log.Fatalf("connect NATS: %v", err)
	}
	defer nc.Drain()

	_, err = nc.QueueSubscribe("fleet.speed.over_limit", "alerts-workers", func(m *nats.Msg) {
		a, err := alerts.FromOverspeedEvent(m.Data)
		if err != nil {
			log.Printf("drop malformed event: %v", err)
			return
		}
		alertsCount.WithLabelValues(a.Severity).Inc()
		out, _ := json.Marshal(a)
		log.Printf("alert: %s", out)
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("alerts-worker subscribed to fleet.speed.over_limit, NATS %s", natsURL)

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	log.Printf("metrics on %s", metricsAddr)
	if err := http.ListenAndServe(metricsAddr, mux); err != nil {
		log.Fatal(err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
