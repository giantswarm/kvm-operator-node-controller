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
	"errors"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

type instances struct {
	client    clientset.Interface
	namespace string
	logger    micrologger.Logger
}

func newInstances(
	client clientset.Interface,
	namespace string,
	logger micrologger.Logger) cloudprovider.Instances {

	return &instances{
		client:    client,
		namespace: namespace,
		logger:    logger,
	}
}

func (i *instances) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	return errors.New("not implemented")
}

// ExternalID returns the pod name of the VM in host cluster.
// If the pod does not exist or not ready, the
// returned error will be cloudprovider.InstanceNotFound.
func (i *instances) ExternalID(nodeName types.NodeName) (string, error) {
	// Try to get requested pod in namespace.
	i.logger.Log("info", "checking pod", "pod", string(nodeName), "namespace", i.namespace)
	pod, err := i.client.CoreV1().Pods(i.namespace).Get(string(nodeName), metav1.GetOptions{})

	if apierr.IsNotFound(err) {
		// Handle not found error.
		i.logger.Log("info", "pod not found", "pod", string(nodeName), "namespace", i.namespace)
		return "", cloudprovider.InstanceNotFound
	} else if err != nil {
		// Handle other errors.
		i.logger.Log("error", "can not get pod", "pod", string(nodeName), "trace", microerror.Mask(err))
		return "", err
	} else if pod.Status.Phase != v1.PodRunning {
		// Check if pod phase is not running. e.g. if pod stuck in Terminating state or CrashLoopBackoff.
		i.logger.Log("info", "pod not running", "pod", string(nodeName), "namespace", i.namespace)
		return "", cloudprovider.InstanceNotFound
	}

	// Finally if none of conditions above are met return that pod is OK.
	i.logger.Log("info", "pod is running", "pod", string(nodeName), "namespace", i.namespace)
	return string(nodeName), nil
}

func (i *instances) CurrentNodeName(hostname string) (types.NodeName, error) {
	return types.NodeName(""), errors.New("not implemented")
}

func (i *instances) InstanceID(nodeName types.NodeName) (string, error) {
	return "", errors.New("not implemented")
}

func (i *instances) InstanceType(name types.NodeName) (string, error) {
	return "", errors.New("not implemented")
}

func (i *instances) InstanceTypeByProviderID(providerID string) (string, error) {
	return "", errors.New("not implemented")
}

func (i *instances) InstanceExistsByProviderID(providerID string) (bool, error) {
	return false, errors.New("not implemented")
}

func (i *instances) NodeAddresses(name types.NodeName) ([]v1.NodeAddress, error) {
	return nil, errors.New("not implemented")
}

func (i *instances) NodeAddressesByProviderID(providerID string) ([]v1.NodeAddress, error) {
	return nil, errors.New("not implemented")
}
