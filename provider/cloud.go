/*
Copyright 2017 Giant Swarm.

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

package provider

import (
	"fmt"
	"io"
	"os"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
)

const providerName string = "kubernetes"
const providerKubeconfigEnv string = "PROVIDER_HOST_CLUSTER_KUBECONFIG"
const providerNamespaceEnv string = "PROVIDER_HOST_CLUSTER_NAMESPACE"

type cloud struct {
	client    clientset.Interface
	instances cloudprovider.Instances
	logger    micrologger.Logger
}

func newCloud(config io.Reader) (cloudprovider.Interface, error) {
	// Initialize logger.
	logger, err := micrologger.New(micrologger.DefaultConfig())
	if err != nil {
		microerror.Mask(err)
	}

	// Check if namespace is set.
	namespace := os.Getenv(providerNamespaceEnv)
	if namespace == "" {
		err := fmt.Errorf("please specify host cluster namespace via %s", providerNamespaceEnv)
		return nil, err
	}

	// Get kubeconfig for host cluster.
	// This is not required as we are running in host cluster
	// so by default in-cluster configuration will be used.
	kubeconfig := os.Getenv(providerKubeconfigEnv)
	if kubeconfig == "" {
		logger.Log("info", "trying to use in-cluster config")
	}

	// Create kubeclient config.
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		microerror.Mask(err)
	}

	// Create the client.
	hostClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		microerror.Mask(err)
	}

	return &cloud{
		client:    hostClient,
		instances: newInstances(hostClient, namespace, logger),
		logger:    logger,
	}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(config io.Reader) (cloudprovider.Interface, error) {
		return newCloud(config)
	})
}

func (c *cloud) Initialize(clientBuilder controller.ControllerClientBuilder) {
}

func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (c *cloud) ProviderName() string {
	return providerName
}

func (c *cloud) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return nil, nil
}

func (c *cloud) HasClusterID() bool {
	return false
}
