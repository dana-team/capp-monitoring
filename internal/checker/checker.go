package checker

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Status string

const (
	StatusOperational Status = "operational"
	StatusDegraded    Status = "degraded"
	StatusDown        Status = "down"
)

type Component struct {
	Name       string
	Group      string // "core" or "infrastructure"
	Namespace  string
	Deployment string
}

type Result struct {
	Component Component
	Status    Status
	Message   string
}

type Checker struct {
	client     client.Client
	components []Component
	cache      []Result
	mu         sync.RWMutex
	interval   time.Duration
}

func New(c client.Client, components []Component, interval time.Duration) *Checker {
	return &Checker{client: c, components: components, interval: interval}
}

// Start runs checks immediately then on every interval tick until ctx is cancelled.
func (c *Checker) Start(ctx context.Context) {
	c.runChecks(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.runChecks(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// CheckOnce runs all checks synchronously and returns results. Used in tests.
func (c *Checker) CheckOnce(ctx context.Context) []Result {
	c.runChecks(ctx)
	return c.Results()
}

// Results returns the last cached check results.
func (c *Checker) Results() []Result {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Result, len(c.cache))
	copy(out, c.cache)
	return out
}

func (c *Checker) runChecks(ctx context.Context) {
	results := make([]Result, len(c.components))
	var wg sync.WaitGroup
	for i, comp := range c.components {
		wg.Add(1)
		go func(i int, comp Component) {
			defer wg.Done()
			results[i] = c.checkDeployment(ctx, comp)
		}(i, comp)
	}
	wg.Wait()
	c.mu.Lock()
	c.cache = results
	c.mu.Unlock()
}

func (c *Checker) checkDeployment(ctx context.Context, comp Component) Result {
	var dep appsv1.Deployment
	if err := c.client.Get(ctx, types.NamespacedName{
		Name:      comp.Deployment,
		Namespace: comp.Namespace,
	}, &dep); err != nil {
		return Result{Component: comp, Status: StatusDown, Message: err.Error()}
	}
	desired := dep.Status.Replicas
	ready := dep.Status.ReadyReplicas
	if desired == 0 {
		return Result{Component: comp, Status: StatusDown, Message: "no replicas configured"}
	}
	if ready == 0 {
		return Result{Component: comp, Status: StatusDown, Message: "0/0 replicas ready"}
	}
	if ready < desired {
		return Result{Component: comp, Status: StatusDegraded,
			Message: fmt.Sprintf("%d/%d replicas ready", ready, desired)}
	}
	return Result{Component: comp, Status: StatusOperational}
}
