package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/dana-team/capp-monitoring/internal/checker"
	"github.com/dana-team/capp-monitoring/internal/webstatic"
)

// ComponentStatus is the JSON representation of one component's health.
type ComponentStatus struct {
	Name    string         `json:"name"`
	Group   string         `json:"group"`
	Status  checker.Status `json:"status"`
	Message string         `json:"message,omitempty"`
}

// StatusResponse is the JSON body returned by GET /api/status.
type StatusResponse struct {
	Overall    checker.Status    `json:"overall"`
	Components []ComponentStatus `json:"components"`
}

// Server handles HTTP requests for the status page.
type Server struct {
	checker *checker.Checker
	mux     *http.ServeMux
	upGauge *prometheus.GaugeVec
	reg     *prometheus.Registry
}

// New creates a Server and registers Prometheus metrics on a fresh registry.
func New(c *checker.Checker) *Server {
	reg := prometheus.NewRegistry()
	upGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "capp_component_up",
		Help: "1 if the component deployment has ready replicas, 0 otherwise.",
	}, []string{"component", "group"})
	reg.MustRegister(upGauge)

	s := &Server{checker: c, mux: http.NewServeMux(), upGauge: upGauge, reg: reg}

	// Serve embedded HTML/CSS assets
	sub, err := fs.Sub(webstatic.Assets, "web")
	if err != nil {
		log.Fatalf("server: failed to sub embedded FS: %v", err)
	}
	s.mux.Handle("/", http.FileServer(http.FS(sub)))
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// RunChecksOnce triggers a synchronous check cycle (used in tests to pre-populate state).
func (s *Server) RunChecksOnce(ctx context.Context) {
	s.updateGauges(s.checker.CheckOnce(ctx))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	results := s.checker.Results()
	if len(results) == 0 {
		// No cached results yet — run synchronously
		results = s.checker.CheckOnce(r.Context())
	}

	resp := StatusResponse{Overall: checker.StatusOperational}
	for _, res := range results {
		cs := ComponentStatus{
			Name:    res.Component.Name,
			Group:   res.Component.Group,
			Status:  res.Status,
			Message: res.Message,
		}
		resp.Components = append(resp.Components, cs)
		s.upGauge.WithLabelValues(res.Component.Name, res.Component.Group).Set(gaugeVal(res.Status))
		resp.Overall = worstStatus(resp.Overall, res.Status)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("handleStatus: failed to encode response: %v", err)
	}
}

func (s *Server) updateGauges(results []checker.Result) {
	for _, res := range results {
		s.upGauge.WithLabelValues(res.Component.Name, res.Component.Group).Set(gaugeVal(res.Status))
	}
}

func gaugeVal(s checker.Status) float64 {
	if s == checker.StatusOperational {
		return 1
	}
	return 0
}

func worstStatus(current, incoming checker.Status) checker.Status {
	if incoming == checker.StatusDown {
		return checker.StatusDown
	}
	if incoming == checker.StatusDegraded && current == checker.StatusOperational {
		return checker.StatusDegraded
	}
	return current
}
