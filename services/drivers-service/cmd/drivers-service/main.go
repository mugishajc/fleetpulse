package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nats-io/nats.go"

	apihttp "github.com/mugishajc/fleetpulse/services/drivers-service/internal/http"
	"github.com/mugishajc/fleetpulse/services/drivers-service/internal/store"
)

type natsPub struct{ nc *nats.Conn }

func (p *natsPub) Publish(subject string, data []byte) error { return p.nc.Publish(subject, data) }

func main() {
	natsURL := getenv("NATS_URL", nats.DefaultURL)
	addr := getenv("HTTP_ADDR", ":8080")

	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second), nats.ReconnectWait(2*time.Second))
	if err != nil {
		log.Fatalf("connect NATS: %v", err)
	}
	defer nc.Drain()

	srv := apihttp.New(store.NewMemory(), &natsPub{nc: nc})
	log.Printf("drivers-service listening on %s, NATS %s", addr, natsURL)
	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
