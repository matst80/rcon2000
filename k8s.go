package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
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
		home := homedir.HomeDir()
		if home != "" {
			config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
		}
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

func handlePodLogs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to websocket: %v", err)
		return
	}
	defer ws.Close()

	deploymentName := getEnv("RCON_DEPLOYMENT", "")
	if deploymentName == "" {
		log.Println("Game server deployment not configured for logs")
		ws.WriteMessage(websocket.TextMessage, []byte("Game server deployment not configured."))
		return
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + deploymentName,
	})
	if err != nil || len(pods.Items) == 0 {
		log.Printf("Could not find pod for deployment %s: %v", deploymentName, err)
		ws.WriteMessage(websocket.TextMessage, []byte("Could not find running game server pod."))
		return
	}
	podName := pods.Items[0].Name

	log.Printf("Streaming logs for pod %s", podName)

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow: true,
	})
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		log.Printf("Error streaming logs: %v", err)
		ws.WriteMessage(websocket.TextMessage, []byte("Error streaming logs."))
		return
	}
	defer podLogs.Close()

	scanner := bufio.NewScanner(podLogs)
	for scanner.Scan() {
		err := ws.WriteMessage(websocket.TextMessage, scanner.Bytes())
		if err != nil {
			log.Printf("Websocket write error, closing log stream: %v", err)
			break
		}
	}
}

func handleGameServer(w http.ResponseWriter, r *http.Request) {
	if clientset == nil {
		http.Error(w, "Kubernetes client not initialized", http.StatusInternalServerError)
		return
	}

	deploymentName := getEnv("RCON_DEPLOYMENT", "")
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
