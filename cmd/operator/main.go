package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = klog.Background()
)

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	// Initialize Kubernetes client
	config, err := getKubeConfig()
	if err != nil {
		setupLog.Error(err, "unable to get kubeconfig")
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create kubernetes client")
		os.Exit(1)
	}

	setupLog.Info("Starting operator", "version", "v0.1.0")

	// Simple operator loop
	ctx := context.Background()
	for {
		select {
		case <-ctx.Done():
			setupLog.Info("Shutting down operator")
			return
		default:
			// Check if we can list pods (simple health check)
			_, err := clientset.CoreV1().Pods("").List(ctx, nil)
			if err != nil {
				setupLog.Error(err, "unable to list pods")
			} else {
				setupLog.V(1).Info("Operator is healthy and can access Kubernetes API")
			}

			// Sleep for a while
			time.Sleep(30 * time.Second)
		}
	}
}

func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}
