package checker

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Status string

const (
	StatusOperational   Status = "operational"
	StatusDegraded      Status = "degraded"
	StatusDown          Status = "down"
	CoreGroup                  = "core"
	InfrastructureGroup        = "infrastructure"
)

type Component struct {
	Name  string
	Group string
}

type ClusterComponent struct {
	Component
	Namespace  string
	Deployment string
}

type NetworkComponent struct {
	Component
	URL string
}

type Result struct {
	Component Component
	Status    Status
	Message   string
}

type Checker struct {
	client            client.Client
	httpClient        *http.Client
	networkComponents []NetworkComponent
	clusterComponents []ClusterComponent
	cache             []Result
	mu                sync.RWMutex
	interval          time.Duration
}

func New(c client.Client, hc *http.Client, clusterComponents []ClusterComponent, networkComponents []NetworkComponent, interval time.Duration) *Checker {
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	return &Checker{client: c, httpClient: hc, clusterComponents: clusterComponents, networkComponents: networkComponents, interval: interval}
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
	results := make([]Result, len(c.clusterComponents)+len(c.networkComponents))
	var wg sync.WaitGroup
	for i, comp := range c.clusterComponents {
		wg.Add(1)
		go func(i int, comp ClusterComponent) {
			defer wg.Done()
			results[i] = c.checkDeployment(ctx, comp)
		}(i, comp)
	}
	for i, comp := range c.networkComponents {
		wg.Add(1)
		go func(i int, comp NetworkComponent) {
			defer wg.Done()
			results[i+len(c.clusterComponents)] = c.checkNetwork(ctx, comp)
		}(i, comp)
	}
	wg.Wait()
	c.mu.Lock()
	c.cache = results
	c.mu.Unlock()
}

func (c *Checker) checkNetwork(ctx context.Context, comp NetworkComponent) Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, comp.URL, nil)
	if err != nil {
		return Result{Component: comp.Component, Status: StatusDown, Message: "invalid URL: " + err.Error()}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{Component: comp.Component, Status: StatusDown, Message: err.Error()}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v", err)
		}
	}()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Result{Component: comp.Component, Status: StatusOperational}
	}
	if resp.StatusCode >= 500 {
		return Result{Component: comp.Component, Status: StatusDown,
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return Result{Component: comp.Component, Status: StatusDegraded,
		Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

func (c *Checker) checkDeployment(ctx context.Context, comp ClusterComponent) Result {
	var dep appsv1.Deployment
	if err := c.client.Get(ctx, types.NamespacedName{
		Name:      comp.Deployment,
		Namespace: comp.Namespace,
	}, &dep); err != nil {
		return Result{Component: comp.Component, Status: StatusDown, Message: err.Error()}
	}
	desired := dep.Status.Replicas
	ready := dep.Status.ReadyReplicas
	if desired == 0 {
		return Result{Component: comp.Component, Status: StatusDown, Message: "no replicas configured"}
	}
	if ready == 0 {
		return Result{Component: comp.Component, Status: StatusDown,
			Message: fmt.Sprintf("0/%d replicas ready", desired)}
	}
	if ready < desired {
		return Result{Component: comp.Component, Status: StatusDegraded,
			Message: fmt.Sprintf("%d/%d replicas ready", ready, desired)}
	}
	return Result{Component: comp.Component, Status: StatusOperational}
}
