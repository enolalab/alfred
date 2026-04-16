package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type HealthDependency struct {
	Name     string
	Required bool
	Check    func(context.Context) error
}

type ServerConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	Dependencies    []HealthDependency
	Features        map[string]bool
	MetricsSnapshot func() map[string]any
	MetricsText     func() string
}

type Server struct {
	httpServer    *http.Server
	mux           *http.ServeMux
	router        *Router
	queue         *Queue
	logger        *slog.Logger
	deps          []HealthDependency
	features      map[string]bool
	metricsFn     func() map[string]any
	metricsTextFn func() string
}

func NewServer(
	cfg ServerConfig,
	router *Router,
	queue *Queue,
	logger *slog.Logger,
) *Server {
	mux := http.NewServeMux()

	s := &Server{
		mux:           mux,
		router:        router,
		queue:         queue,
		logger:        logger,
		deps:          append([]HealthDependency(nil), cfg.Dependencies...),
		metricsFn:     cfg.MetricsSnapshot,
		metricsTextFn: cfg.MetricsText,
		features: func() map[string]bool {
			if cfg.Features == nil {
				return map[string]bool{}
			}
			cloned := make(map[string]bool, len(cfg.Features))
			for k, v := range cfg.Features {
				cloned[k] = v
			}
			return cloned
		}(),
		httpServer: &http.Server{
			Addr:         cfg.Addr,
			Handler:      mux,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
	}

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /metrics.json", s.handleMetricsJSON)

	return s
}

func (s *Server) RegisterPlatformHandler(path string, handler http.Handler) {
	s.mux.Handle(path, handler)
	s.logger.Info("registered platform handler", "path", path)
}

func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("gateway server starting", "addr", s.httpServer.Addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("gateway server: %w", err)
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("gateway server shutting down")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type queueStatus struct {
		Depth        int    `json:"depth"`
		Capacity     int    `json:"capacity"`
		Workers      int    `json:"workers"`
		Status       string `json:"status"`
		UsagePercent int    `json:"usage_percent"`
	}
	type dependencyStatus struct {
		Status   string `json:"status"`
		Required bool   `json:"required"`
		Error    string `json:"error,omitempty"`
	}
	response := struct {
		Status       string                      `json:"status"`
		Queue        queueStatus                 `json:"queue"`
		Features     map[string]bool             `json:"features,omitempty"`
		Dependencies map[string]dependencyStatus `json:"dependencies,omitempty"`
	}{
		Status: "ok",
		Queue: queueStatus{
			Depth:        s.queue.Depth(),
			Capacity:     s.queue.Capacity(),
			Workers:      s.queue.Workers(),
			Status:       s.queue.Status(),
			UsagePercent: s.queue.UsagePercent(),
		},
		Features:     s.features,
		Dependencies: make(map[string]dependencyStatus, len(s.deps)),
	}

	if response.Queue.Status == "full" {
		response.Status = "degraded"
	}

	for _, dep := range s.deps {
		if dep.Check == nil {
			response.Dependencies[dep.Name] = dependencyStatus{
				Status:   "unknown",
				Required: dep.Required,
			}
			continue
		}
		if err := dep.Check(r.Context()); err != nil {
			response.Dependencies[dep.Name] = dependencyStatus{
				Status:   "down",
				Required: dep.Required,
				Error:    err.Error(),
			}
			s.logger.Warn("health dependency degraded",
				"dependency", dep.Name,
				"required", dep.Required,
				"error", err,
			)
			if dep.Required {
				response.Status = "degraded"
			}
			continue
		}
		response.Dependencies[dep.Name] = dependencyStatus{
			Status:   "ok",
			Required: dep.Required,
		}
	}

	if response.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if s.metricsTextFn == nil {
		_, _ = w.Write([]byte(""))
		return
	}
	_, _ = w.Write([]byte(s.metricsTextFn()))
}

func (s *Server) handleMetricsJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.metricsFn == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"counters": map[string]int64{},
			"timings":  map[string]any{},
		})
		return
	}
	_ = json.NewEncoder(w).Encode(s.metricsFn())
}
