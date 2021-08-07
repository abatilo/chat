package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func (s *Server) registerRoutes() {
	s.router.Get("/check", s.ping())
}

func (s *Server) ping() http.HandlerFunc {
	duration := s.metrics.NewHistogram(prometheus.HistogramOpts{
		Name: "chat_check_duration_seconds",
		Help: "Histogram for check endpoint latency",
	})

	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		duration.Observe(time.Since(now).Seconds())
		fmt.Fprintf(w, "ok")
	}
}
