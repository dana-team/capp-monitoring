package main

import (
	"context"
	"log"
	"net/http"
	"os"
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

	components := []checker.Component{
		{Name: "CAPP Backend API", Group: "core",
			Namespace: env("NS_CAPP", "capp-system"), Deployment: "capp-backend"},
		{Name: "CAPP Frontend", Group: "core",
			Namespace: env("NS_CAPP", "capp-system"), Deployment: "capp-frontend"},
		{Name: "Knative Serving", Group: "core",
			Namespace: env("NS_KNATIVE", "knative-serving"), Deployment: "controller"},
		{Name: "Container-App-Operator", Group: "core",
			Namespace: env("NS_CAPP", "capp-system"), Deployment: "container-app-operator-manager"},
		{Name: "cert-manager", Group: "infrastructure",
			Namespace: env("NS_CERT_MANAGER", "cert-manager"), Deployment: "cert-manager"},
		{Name: "logging-operator", Group: "infrastructure",
			Namespace: env("NS_LOGGING", "logging-operator"), Deployment: "logging-operator"},
		{Name: "nfspvc-operator", Group: "infrastructure",
			Namespace: env("NS_NFSPVC", "nfspvc-operator"), Deployment: "nfspvc-operator"},
		{Name: "provider-dns", Group: "infrastructure",
			Namespace: env("NS_PROVIDER_DNS", "provider-dns"), Deployment: "provider-dns"},
		{Name: "cert-external-issuer", Group: "infrastructure",
			Namespace: env("NS_CERT_MANAGER", "cert-manager"), Deployment: "cert-external-issuer"},
	}

	chk := checker.New(k8sClient, components, 30*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go chk.Start(ctx)

	srv := server.New(chk)

	addr := ":" + env("PORT", "8080")
	log.Printf("capp-status-server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
