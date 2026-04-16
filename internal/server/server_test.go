package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/dana-team/capp-monitoring/internal/checker"
	"github.com/dana-team/capp-monitoring/internal/server"
)

func newTestServer(t *testing.T, deps ...*appsv1.Deployment) *server.Server {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, d := range deps {
		builder = builder.WithObjects(d)
	}
	cl := builder.Build()

	components := []checker.Component{
		{Name: "CAPP Backend API", Group: "core", Namespace: "capp-system", Deployment: "capp-backend"},
	}
	chk := checker.New(cl, components, time.Second)
	return server.New(chk)
}

func TestHandleStatus_ReturnsJSON(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "capp-backend", Namespace: "capp-system"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	}
	srv := newTestServer(t, dep)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json, got %s", ct)
	}
	var resp server.StatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Overall != checker.StatusOperational {
		t.Errorf("expected overall=operational, got %s", resp.Overall)
	}
	if len(resp.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(resp.Components))
	}
}

func TestHandleStatus_Overall_Down_When_Component_Down(t *testing.T) {
	// no deployment exists → component is down
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp server.StatusResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Overall != checker.StatusDown {
		t.Errorf("expected overall=down, got %s", resp.Overall)
	}
}

func TestHandleMetrics_ExposesGauge(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "capp-backend", Namespace: "capp-system"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	}
	srv := newTestServer(t, dep)

	// Trigger a status check first to populate the gauge
	srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/status", nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "capp_component_up") {
		t.Errorf("expected capp_component_up metric in output, got:\n%s", w.Body.String())
	}
}

func TestHandleIndex_ServesHTML(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html, got %s", ct)
	}
}
