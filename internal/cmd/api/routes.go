package api

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

func (s *Server) registerRoutes() {
	s.router.Get("/ping", s.ping())
}

func (s *Server) ping() http.HandlerFunc {
	metric := s.metrics.NewCounter(prometheus.CounterOpts{
		Name: "chat_ping_total",
		Help: "Number of requests to the ping endpoint",
	})

	return func(w http.ResponseWriter, r *http.Request) {
		metric.Inc()
		fmt.Fprintf(w, "pong")
	}
}
