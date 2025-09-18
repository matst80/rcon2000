package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var clientset *kubernetes.Clientset
var namespace string

func initKube() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Unable to get in-cluster config, using fallback to default: %v", err)
		// Fallback for local development
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
		if err != nil {
			log.Fatalf("Failed to get kubeconfig: %v", err)
		}
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		log.Printf("Could not read namespace, using 'default': %v", err)
		namespace = "game"
	} else {
		namespace = string(ns)
	}
	log.Printf("Operating in namespace: %s", namespace)
}

func getGameServerDeploymentName() string {
	rconHost := getEnv("RCON_DEPLOYMENT", "minecraft")
	if rconHost == "" {
		return ""
	}
	// Assuming RCON_HOST is like 'minecraft-rcon' or 'cs2-rcon'
	// and deployment is 'minecraft' or 'cs2'
	return rconHost
}

func handleGameServer(w http.ResponseWriter, r *http.Request) {
	if clientset == nil {
		http.Error(w, "Kubernetes client not initialized", http.StatusInternalServerError)
		return
	}

	deploymentName := getGameServerDeploymentName()
	if deploymentName == "" {
		http.Error(w, "Game server deployment not configured", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case "GET":
		getGameServerStatus(w, r, deploymentName)
	case "POST":
		scaleGameServer(w, r, deploymentName, 1) // Start
	case "DELETE":
		scaleGameServer(w, r, deploymentName, 0) // Stop
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getGameServerStatus(w http.ResponseWriter, r *http.Request, deploymentName string) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get deployment: %v", err), http.StatusInternalServerError)
		return
	}

	status := "stopped"
	if deployment.Status.ReadyReplicas > 0 {
		status = "running"
	}

	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func scaleGameServer(w http.ResponseWriter, r *http.Request, deploymentName string, replicas int32) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get deployment: %v", err), http.StatusInternalServerError)
		return
	}

	deployment.Spec.Replicas = &replicas
	_, err = clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to scale deployment: %v", err), http.StatusInternalServerError)
		return
	}

	action := "started"
	if replicas == 0 {
		action = "stopped"
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("Game server %s", action)})
}
