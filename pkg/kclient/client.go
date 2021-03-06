/*
Copyright 2021 The OpenYurt Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kclient

import (
	"net"
	"net/url"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openyurtio/yurtcluster-operator/api/v1alpha1"
)

var (
	Scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(Scheme)
	_ = operatorv1alpha1.AddToScheme(Scheme)
}

var config *rest.Config
var ctlClient client.Client
var dynamicClient dynamic.Interface
var discoveryClient *discovery.DiscoveryClient

// GetConfigWithAPIServerAddress returns the *rest.Config based on the given api server address
func GetConfigWithAPIServerAddress(apiServerAddress string) (*rest.Config, error) {
	if len(apiServerAddress) == 0 {
		return GetConfig()
	}

	u, err := url.Parse(apiServerAddress)
	if err != nil {
		klog.Warningf("failed to parse URL from %q, %v", apiServerAddress, err)
		return GetConfig()
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	os.Setenv("KUBERNETES_SERVICE_HOST", host)
	os.Setenv("KUBERNETES_SERVICE_PORT", port)

	return GetConfig()
}

// GetConfig creates a *rest.Config for talking to a Kubernetes apiserver.
func GetConfig() (*rest.Config, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}

	// Set QPS and Burst to a threshold that ensures the controller runtime
	// client/client go doesn't generate throttling log messages
	config.QPS = 100
	config.Burst = 200

	return config, nil
}

// InitializeKubeClient initializes the Kubernetes Client
func InitializeKubeClient(conf *rest.Config) {
	var err error
	ctlClient, err = client.New(conf, client.Options{Scheme: Scheme})
	if err != nil {
		panic(err)
	}

	dynamicClient, err = dynamic.NewForConfig(conf)
	if err != nil {
		panic(err)
	}

	discoveryClient, err = discovery.NewDiscoveryClientForConfig(conf)
	if err != nil {
		panic(err)
	}

	config = conf
}

// CtlClient returns the controller-runtime client
func CtlClient() client.Client {
	if ctlClient == nil {
		panic("please invoke kclient.InitializeKubeClient() to initializes client instance before call this function")
	}
	return ctlClient
}

// DynamicClient returns the Kubernetes dynamic Client
func DynamicClient() dynamic.Interface {
	if dynamicClient == nil {
		panic("please invoke kclient.InitializeKubeClient() to initializes dynamicClient instance before call this function")
	}
	return dynamicClient
}

// DiscoveryClient returns the Kubernetes discovery Client
func DiscoveryClient() *discovery.DiscoveryClient {
	if discoveryClient == nil {
		panic("please invoke kclient.InitializeKubeClient() to initializes discoveryClient instance before call this function")
	}
	return discoveryClient
}

// Config returns the *rest.Config
func Config() *rest.Config {
	if config == nil {
		panic("please invoke kclient.InitializeKubeClient() to initializes config instance before call this function")
	}
	return config
}
