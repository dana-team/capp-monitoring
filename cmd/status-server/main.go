package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dana-team/capp-monitoring/internal/checker"
	"github.com/dana-team/capp-monitoring/internal/server"
)

func main() {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("failed to get in-cluster config: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		log.Fatalf("failed to add appsv1 to scheme: %v", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("failed to create k8s client: %v", err)
	}

	clusterComponents := []checker.ClusterComponent{
		{Component: checker.Component{Name: "CAPP Backend API", Group: checker.CoreGroup},
			Namespace: env("NS_CAPP", "capp-platform-system"), Deployment: "capp-backend"},
		{Component: checker.Component{Name: "CAPP Frontend", Group: checker.CoreGroup},
			Namespace: env("NS_CAPP", "capp-platform-system"), Deployment: "capp-frontend"},
		{Component: checker.Component{Name: "Knative Serving", Group: checker.CoreGroup},
			Namespace: env("NS_KNATIVE", "knative-serving"), Deployment: "controller"},
		{Component: checker.Component{Name: "Container-App-Operator", Group: checker.CoreGroup},
			Namespace: env("NS_CAPP", "container-app-operator-system"), Deployment: "container-app-operator-controller-manager"},
		{Component: checker.Component{Name: "cert-manager", Group: checker.InfrastructureGroup},
			Namespace: env("NS_CERT_MANAGER", "cert-manager"), Deployment: "cert-manager"},
		{Component: checker.Component{Name: "logging-operator", Group: checker.InfrastructureGroup},
			Namespace: env("NS_LOGGING", "logging-operator"), Deployment: "logging-operator"},
		{Component: checker.Component{Name: "nfspvc-operator", Group: checker.InfrastructureGroup},
			Namespace: env("NS_NFSPVC", "nfspvc-operator"), Deployment: "nfspvc-operator-controller-manager"},
		{Component: checker.Component{Name: "provider-dns", Group: checker.InfrastructureGroup},
			Namespace: env("NS_PROVIDER_DNS", "provider-dns"), Deployment: "provider-dns-v2"},
		{Component: checker.Component{Name: "cert-external-issuer", Group: checker.InfrastructureGroup},
			Namespace: env("NS_CERT_MANAGER", "cert-manager"), Deployment: "cert-external-issuer-controller-manager"},
	}

	chk := checker.New(k8sClient, nil, clusterComponents, nil, 30*time.Second)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go chk.Start(ctx)

	srv := server.New(chk)
	httpSrv := &http.Server{Addr: ":" + env("PORT", "8080"), Handler: srv}

	go func() {
		log.Printf("capp-status-server listening on %s", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
