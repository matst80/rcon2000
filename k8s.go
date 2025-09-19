package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type GameWatcher struct {
	K8sConfig
	client *kubernetes.Clientset
}

func (w *GameWatcher) getClient() *kubernetes.Clientset {
	if w.client != nil {
		return w.client
	}
	c, err := w.Connect()
	if err != nil {
		log.Fatal(err)
	}
	w.client = c
	return c
}

func NewGameWatcher(config K8sConfig) (*GameWatcher, error) {
	client, err := config.Connect()
	if err != nil {
		return nil, err
	}
	return &GameWatcher{
		K8sConfig: config,
		client:    client,
	}, nil
}

func (w *GameWatcher) GetDeployment() (*v1.Deployment, error) {
	return w.getClient().AppsV1().Deployments(w.Namespace).Get(context.TODO(), w.DeploymentName, metav1.GetOptions{})
}

func (w *GameWatcher) GetLogs() (io.ReadCloser, error) {
	pods, err := w.getClient().CoreV1().Pods(w.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + w.DeploymentName,
	})
	if err != nil || len(pods.Items) == 0 {
		return nil, errors.New("pod is missing")
	}
	podName := pods.Items[0].Name

	log.Printf("Streaming logs for pod %s", podName)

	req := w.getClient().CoreV1().Pods(w.Namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow: true,
	})
	return req.Stream(context.TODO())
}

func (w *GameWatcher) Scale(replicas int32) error {
	deployment, err := w.GetDeployment()
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = &replicas
	_, err = w.getClient().AppsV1().Deployments(w.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	return err
}

func (gw *GameWatcher) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/gameserver", func(w http.ResponseWriter, r *http.Request) {
		d, err := gw.GetDeployment()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get deployment: %v", err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(d.Status)
	})
	mux.HandleFunc("POST /api/gameserver", func(w http.ResponseWriter, r *http.Request) {
		err := gw.Scale(1)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to scale deployment: %v", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("DELETE /api/gameserver", func(w http.ResponseWriter, r *http.Request) {
		err := gw.Scale(0)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to scale deployment: %v", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Failed to upgrade to websocket: %v", err)
			return
		}
		defer ws.Close()
		podLogs, err := gw.GetLogs()
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
	})
}
