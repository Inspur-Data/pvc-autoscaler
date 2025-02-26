package utils

import (
	"encoding/json"
	"k8s.io/client-go/rest"
	clientCmd "k8s.io/client-go/tools/clientcmd"
)

type KubernetesOptions struct {
	ClientConfig *rest.Config `json:"clientConfig" yaml:"clientConfig"`
	// kubeconfig path, if not specified, will use
	// in cluster way to create clientset
	KubeConfig string `json:"kubeconfig" yaml:"kubeconfig"`

	// kubernetes apiserver public address, used to generate kubeconfig
	// for downloading, default to host defined in kubeconfig
	// +optional
	Master string `json:"master,omitempty" yaml:"master"`

	// kubernetes clientset qps
	// +optional
	QPS float32 `json:"qps,omitemtpy" yaml:"qps"`

	// kubernetes clientset burst
	// +optional
	Burst int `json:"burst,omitempty" yaml:"burst"`
}

// NewKubernetesOptions returns a `zero` instance
func NewKubernetesOptions(runtimeCredentialContent string) *KubernetesOptions {
	var err error
	clientConfig := &rest.Config{}

	if json.Valid([]byte(runtimeCredentialContent)) {
		var mapResult map[string]string
		json.Unmarshal([]byte(runtimeCredentialContent), &mapResult)
		if mapResult["userName"] == "" {
			clientConfig, err = clientCmd.RESTConfigFromKubeConfig([]byte(runtimeCredentialContent))
			if err != nil {
				return nil
			}
		} else {
			clientConfig.Username = mapResult["userName"]
			clientConfig.Password = mapResult["password"]
			clientConfig.TLSClientConfig.CAData = Base64Decode(mapResult["caCertData"])
			clientConfig.Host = mapResult["apiServerUrl"]
		}
	} else {
		clientConfig, err = clientCmd.RESTConfigFromKubeConfig([]byte(runtimeCredentialContent))
		if err != nil {
			return nil
		}
	}
	return &KubernetesOptions{
		ClientConfig: clientConfig,
		KubeConfig:   "",
		QPS:          1e6,
		Burst:        1e6,
	}
}
