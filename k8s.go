package main

import (
	"context"
	"errors"
	"io"
	"log"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var clientset *kubernetes.Clientset
var namespace string

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
	return w.getClient().AppsV1().Deployments(namespace).Get(context.TODO(), w.DeploymentName, metav1.GetOptions{})
}

func (w *GameWatcher) GetLogs() (io.ReadCloser, error) {
	pods, err := w.getClient().CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + w.DeploymentName,
	})
	if err != nil || len(pods.Items) == 0 {
		return nil, errors.New("Pod is missing")
	}
	podName := pods.Items[0].Name

	log.Printf("Streaming logs for pod %s", podName)

	req := w.getClient().CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
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
	_, err = clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	return err
}
