package utils

import (
	"fmt"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var clusterClient ExpiredMap

type Client interface {
	Kubernetes() kubernetes.Interface
	Dynamic() dynamic.Interface
	Master() string
	Config() *rest.Config
}

type kubernetesClient struct {
	k8s     kubernetes.Interface
	dynamic dynamic.Interface
	master  string
	config  *rest.Config
}

// NewKubernetesClient creates a kubernetesClient
func NewKubernetesClient(runtimeCredentialContent string) (Client, error) {
	key := Md5(runtimeCredentialContent)

	if len(runtimeCredentialContent) == 0 {
		return nil, fmt.Errorf("runtimeCredentialContent is not right")
	}

	var err error
	var k kubernetesClient

	options := NewKubernetesOptions(runtimeCredentialContent)
	if options == nil {
		return nil, fmt.Errorf("runtimeCredentialContent is not right")
	}
	config := options.ClientConfig
	config.QPS = options.QPS
	config.Burst = options.Burst
	v, ok := clusterClient.Load(key)
	if ok {
		return v.(*kubernetesClient), nil
	}

	k.k8s, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	k.dynamic, err = dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	k.master = options.Master
	k.config = config

	clusterClient.Store(key, &k, 1*time.Hour)
	return &k, nil
}

func (k *kubernetesClient) Kubernetes() kubernetes.Interface {
	return k.k8s
}

func (k *kubernetesClient) Dynamic() dynamic.Interface {
	return k.dynamic
}

func (k *kubernetesClient) Master() string {
	return k.master
}

func (k *kubernetesClient) Config() *rest.Config {
	return k.config
}
