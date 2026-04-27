package checker_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/dana-team/capp-monitoring/internal/checker"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = appsv1.AddToScheme(s)
	return s
}

func newDeployment(name, ns string, desired, ready int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Status:     appsv1.DeploymentStatus{Replicas: desired, ReadyReplicas: ready},
	}
}

func TestChecker_ClusterComponents(t *testing.T) {
	tests := []struct {
		name        string
		deployment  *appsv1.Deployment // nil = missing from cluster
		wantStatus  checker.Status
		wantMessage string
	}{
		{
			name:       "operational when all replicas ready",
			deployment: newDeployment("capp-backend", "capp-system", 2, 2),
			wantStatus: checker.StatusOperational,
		},
		{
			name:        "degraded when some replicas ready",
			deployment:  newDeployment("capp-backend", "capp-system", 3, 1),
			wantStatus:  checker.StatusDegraded,
			wantMessage: "1/3 replicas ready",
		},
		{
			name:        "down when no replicas ready",
			deployment:  newDeployment("capp-backend", "capp-system", 2, 0),
			wantStatus:  checker.StatusDown,
			wantMessage: "0/2 replicas ready",
		},
		{
			name:        "down when zero desired replicas",
			deployment:  newDeployment("capp-backend", "capp-system", 0, 0),
			wantStatus:  checker.StatusDown,
			wantMessage: "no replicas configured",
		},
		{
			name:       "down when deployment missing",
			deployment: nil,
			wantStatus: checker.StatusDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(newScheme())
			if tt.deployment != nil {
				builder = builder.WithObjects(tt.deployment)
			}
			cl := builder.Build()

			comp := checker.ClusterComponent{
				Component:  checker.Component{Name: "CAPP Backend API", Group: checker.CoreGroup},
				Namespace:  "capp-system",
				Deployment: "capp-backend",
			}
			c := checker.New(cl, nil, []checker.ClusterComponent{comp}, nil, time.Second)
			results := c.CheckOnce(context.Background())

			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].Status != tt.wantStatus {
				t.Errorf("status: got %q, want %q (message: %q)", results[0].Status, tt.wantStatus, results[0].Message)
			}
			if tt.wantMessage != "" && results[0].Message != tt.wantMessage {
				t.Errorf("message: got %q, want %q", results[0].Message, tt.wantMessage)
			}
		})
	}
}

func TestChecker_NetworkComponents(t *testing.T) {
	tests := []struct {
		name       string
		mockStatus int
		mockErr    error // non-nil = connection failure
		wantStatus checker.Status
	}{
		{
			name:       "operational on 200",
			mockStatus: http.StatusOK,
			wantStatus: checker.StatusOperational,
		},
		{
			name:       "operational on 201",
			mockStatus: http.StatusCreated,
			wantStatus: checker.StatusOperational,
		},
		{
			name:       "degraded on 404",
			mockStatus: http.StatusNotFound,
			wantStatus: checker.StatusDegraded,
		},
		{
			name:       "degraded on 400",
			mockStatus: http.StatusBadRequest,
			wantStatus: checker.StatusDegraded,
		},
		{
			name:       "down on 500",
			mockStatus: http.StatusInternalServerError,
			wantStatus: checker.StatusDown,
		},
		{
			name:       "down on connection error",
			mockErr:    errors.New("connection refused"),
			wantStatus: checker.StatusDown,
		},
	}

	const target = "http://example.com/health"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := &http.Client{}
			httpmock.ActivateNonDefault(hc)
			defer httpmock.DeactivateAndReset()

			if tt.mockErr != nil {
				httpmock.RegisterResponder(http.MethodGet, target,
					httpmock.NewErrorResponder(tt.mockErr))
			} else {
				httpmock.RegisterResponder(http.MethodGet, target,
					httpmock.NewStringResponder(tt.mockStatus, ""))
			}

			comp := checker.NetworkComponent{
				Component: checker.Component{Name: "Health Endpoint", Group: checker.CoreGroup},
				URL:       target,
			}
			cl := fake.NewClientBuilder().WithScheme(newScheme()).Build()
			c := checker.New(cl, hc, nil, []checker.NetworkComponent{comp}, time.Second)
			results := c.CheckOnce(context.Background())

			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].Status != tt.wantStatus {
				t.Errorf("status: got %q, want %q (message: %q)", results[0].Status, tt.wantStatus, results[0].Message)
			}
		})
	}
}

func TestChecker_Mixed(t *testing.T) {
	const target = "http://example.com/health"

	tests := []struct {
		name          string
		deployment    *appsv1.Deployment
		networkStatus int
		wantStatuses  []checker.Status // index 0 = cluster, index 1 = network
	}{
		{
			name:          "all operational",
			deployment:    newDeployment("capp-backend", "capp-system", 1, 1),
			networkStatus: http.StatusOK,
			wantStatuses:  []checker.Status{checker.StatusOperational, checker.StatusOperational},
		},
		{
			name:          "cluster down, network operational",
			deployment:    nil,
			networkStatus: http.StatusOK,
			wantStatuses:  []checker.Status{checker.StatusDown, checker.StatusOperational},
		},
		{
			name:          "cluster operational, network degraded",
			deployment:    newDeployment("capp-backend", "capp-system", 1, 1),
			networkStatus: http.StatusNotFound,
			wantStatuses:  []checker.Status{checker.StatusOperational, checker.StatusDegraded},
		},
		{
			name:          "cluster degraded, network down",
			deployment:    newDeployment("capp-backend", "capp-system", 2, 1),
			networkStatus: http.StatusInternalServerError,
			wantStatuses:  []checker.Status{checker.StatusDegraded, checker.StatusDown},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := &http.Client{}
			httpmock.ActivateNonDefault(hc)
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(http.MethodGet, target,
				httpmock.NewStringResponder(tt.networkStatus, ""))

			builder := fake.NewClientBuilder().WithScheme(newScheme())
			if tt.deployment != nil {
				builder = builder.WithObjects(tt.deployment)
			}
			cl := builder.Build()

			clusterComps := []checker.ClusterComponent{
				{Component: checker.Component{Name: "CAPP Backend", Group: checker.CoreGroup},
					Namespace: "capp-system", Deployment: "capp-backend"},
			}
			networkComps := []checker.NetworkComponent{
				{Component: checker.Component{Name: "Health Endpoint", Group: checker.CoreGroup},
					URL: target},
			}

			c := checker.New(cl, hc, clusterComps, networkComps, time.Second)
			results := c.CheckOnce(context.Background())

			if len(results) != 2 {
				t.Fatalf("expected 2 results, got %d", len(results))
			}
			for i, want := range tt.wantStatuses {
				if results[i].Status != want {
					t.Errorf("results[%d] (%s): got %q, want %q (message: %q)",
						i, results[i].Component.Name, results[i].Status, want, results[i].Message)
				}
			}
		})
	}
}
