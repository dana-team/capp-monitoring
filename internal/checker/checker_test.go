package checker_test

import (
	"context"
	"testing"
	"time"

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

func TestCheckOnce_Operational(t *testing.T) {
	dep := newDeployment("capp-backend", "capp-system", 2, 2)
	cl := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(dep).Build()

	comp := checker.Component{
		Name: "CAPP Backend API", Group: "core",
		Namespace: "capp-system", Deployment: "capp-backend",
	}
	c := checker.New(cl, []checker.Component{comp}, time.Second)
	results := c.CheckOnce(context.Background())

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != checker.StatusOperational {
		t.Errorf("expected operational, got %s: %s", results[0].Status, results[0].Message)
	}
}

func TestCheckOnce_Degraded(t *testing.T) {
	dep := newDeployment("capp-backend", "capp-system", 3, 1)
	cl := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(dep).Build()

	comp := checker.Component{
		Name: "CAPP Backend API", Group: "core",
		Namespace: "capp-system", Deployment: "capp-backend",
	}
	c := checker.New(cl, []checker.Component{comp}, time.Second)
	results := c.CheckOnce(context.Background())

	if results[0].Status != checker.StatusDegraded {
		t.Errorf("expected degraded, got %s", results[0].Status)
	}
}

func TestCheckOnce_Down_Missing(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(newScheme()).Build() // no deployments

	comp := checker.Component{
		Name: "CAPP Backend API", Group: "core",
		Namespace: "capp-system", Deployment: "capp-backend",
	}
	c := checker.New(cl, []checker.Component{comp}, time.Second)
	results := c.CheckOnce(context.Background())

	if results[0].Status != checker.StatusDown {
		t.Errorf("expected down, got %s", results[0].Status)
	}
}

func TestCheckOnce_Down_NoReplicas(t *testing.T) {
	dep := newDeployment("capp-backend", "capp-system", 0, 0)
	cl := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(dep).Build()

	comp := checker.Component{
		Name: "CAPP Backend API", Group: "core",
		Namespace: "capp-system", Deployment: "capp-backend",
	}
	c := checker.New(cl, []checker.Component{comp}, time.Second)
	results := c.CheckOnce(context.Background())

	if results[0].Status != checker.StatusDown {
		t.Errorf("expected down, got %s", results[0].Status)
	}
}

func TestCheckOnce_MultipleComponents(t *testing.T) {
	deps := []appsv1.Deployment{
		*newDeployment("capp-backend", "capp-system", 1, 1),
		*newDeployment("cert-manager", "cert-manager", 1, 0),
	}
	cl := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(&deps[0], &deps[1]).Build()

	components := []checker.Component{
		{Name: "CAPP Backend", Group: "core", Namespace: "capp-system", Deployment: "capp-backend"},
		{Name: "cert-manager", Group: "infrastructure", Namespace: "cert-manager", Deployment: "cert-manager"},
	}
	c := checker.New(cl, components, time.Second)
	results := c.CheckOnce(context.Background())

	if results[0].Status != checker.StatusOperational {
		t.Errorf("backend: expected operational, got %s", results[0].Status)
	}
	if results[1].Status != checker.StatusDown {
		t.Errorf("cert-manager: expected down, got %s", results[1].Status)
	}
}
