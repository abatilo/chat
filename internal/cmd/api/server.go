package api

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/pprof"

	gosundheit "github.com/AppsFlyer/go-sundheit"
	healthhttp "github.com/AppsFlyer/go-sundheit/http"
	"github.com/abatilo/multiregion-chat-experiment/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
)

const (
	// FlagPortName is the name of the flag that sets which port the main server runs on
	FlagPortName = "port"

	// FlagAdminPortName is the name of the flag that sets which port the admin server runs on
	FlagAdminPortName = "admin-port"
)

// ServerConfig is all configuration for running the application.
//
// We use a config struct so that we can statically type and check configuration values
type ServerConfig struct {
	Port      int
	AdminPort int
}

// Server represents the service itself and all of its dependencies.
//
// This pattern is heavily based on the following blog post:
// https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html
type Server struct {
	adminServer *http.Server
	config      *ServerConfig
	logger      zerolog.Logger
	router      *chi.Mux
	server      *http.Server
	metrics     metrics.Client
}

// ServerOption lets you functionally control construction of the web server
type ServerOption func(s *Server)

// NewServer creates a new api server
func NewServer(cfg *ServerConfig, options ...ServerOption) *Server {
	router := chi.NewRouter()
	s := &Server{
		config: cfg,
		logger: zerolog.New(ioutil.Discard),
		router: router,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: cors.Default().Handler(router),
		},
		metrics: &metrics.NoopMetrics{},
	}

	for _, option := range options {
		option(s)
	}

	s.registerRoutes()

	// We register this last so that we can use things like s.Logger inside of the `createAdminServer`
	if s.adminServer == nil {
		s.adminServer = s.createAdminServer()
	}

	return s
}

// Start starts the main web server and starts a goroutine with the admin
// server
func (s *Server) Start() error {
	go s.adminServer.ListenAndServe()
	return s.server.ListenAndServe()
}

// Shutdown calls for a graceful shutdown on the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.adminServer.Shutdown(ctx)
	return s.server.Shutdown(ctx)
}

func (s *Server) createAdminServer() *http.Server {
	// Healthchecks
	h := gosundheit.New()

	// err := h.RegisterCheck(
	// 	checks.NewHostResolveCheck("api.twilio.com", 1),
	// 	gosundheit.ExecutionPeriod(60*time.Second),
	// 	gosundheit.ExecutionTimeout(2*time.Second),
	// )

	// if err != nil {
	// 	s.logger.Panic().Err(err).Msg("couldn't register healthcheck")
	// }

	mux := http.NewServeMux()
	mux.Handle("/healthz", healthhttp.HandleHealthJSON(h))
	mux.Handle("/metrics", promhttp.Handler())

	// pprof
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	adminSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.AdminPort),
		Handler: mux,
	}

	return adminSrv
}

// WithLogger sets the logger of the server
func WithLogger(logger zerolog.Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

// WithMetrics lets you override the metrics client that's used
func WithMetrics(m metrics.Client) ServerOption {
	return func(s *Server) {
		s.metrics = m
	}
}
