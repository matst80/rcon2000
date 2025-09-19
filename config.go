package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gorcon/rcon"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type RconConfig struct {
	HostName string
	Port     string
	Game     string
	Password string
}

type K8sConfig struct {
	DeploymentName string
	Namespace      string
}

func (k *K8sConfig) getDefaultNamespace() string {
	ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "game" //"default"
	} else {
		return string(ns)
	}
}

func (k *K8sConfig) Connect() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Unable to get in-cluster config, using fallback to default: %v", err)
		// Fallback for local development
		home := homedir.HomeDir()
		if home != "" {
			config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
		}
		if err != nil {
			return nil, err
		}
	} else {
		log.Printf("Successfully loaded in-cluster config")
		k.Namespace = k.getDefaultNamespace()
	}

	return kubernetes.NewForConfig(config)
}

type Config struct {
	RCon RconConfig
	K8s  *K8sConfig
}

func (c RconConfig) RconConnectionString() string {
	return fmt.Sprintf("%s:%s", c.HostName, c.Port)
}

func (c RconConfig) Conenct() (*rcon.Conn, error) {
	return rcon.Dial(c.RconConnectionString(), c.Password)
}

var CurrentConfig = Config{
	RCon: RconConfig{
		HostName: "localhost",
		Port:     "25575",
		Game:     "minecraft",
	},
	K8s: nil,
}

func init() {
	if port, ok := os.LookupEnv("RCON_PORT"); ok {
		CurrentConfig.RCon.Port = port
		if port != "25575" {
			CurrentConfig.RCon.Game = "counter-strike"
		}
	}
	if host, ok := os.LookupEnv("RCON_HOST"); ok {
		CurrentConfig.RCon.HostName = host
	}
	if password, ok := os.LookupEnv("RCON_PASSWORD"); ok {
		CurrentConfig.RCon.Password = password
	}
	if gameType, ok := os.LookupEnv("GAME_TYPE"); ok {
		CurrentConfig.RCon.Game = gameType
	}
	if k8sDeployment, ok := os.LookupEnv("RCON_DEPLOYMENT"); ok {
		namespace := "game" //"default"
		if ns, ok := os.LookupEnv("RCON_NAMESPACE"); ok {
			namespace = ns
		}
		CurrentConfig.K8s = &K8sConfig{
			DeploymentName: k8sDeployment,
			Namespace:      namespace,
		}
	}

}
