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

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	podutilv1 "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

type instances struct {
	client    clientset.Interface
	namespace string
}

func newInstances(client clientset.Interface, namespace string) cloudprovider.Instances {
	return &instances{
		client:    client,
		namespace: namespace,
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
	glog.Infof("Checking pod %s in namespace %s.", string(nodeName), i.namespace)
	pod, err := i.client.CoreV1().Pods(i.namespace).Get(string(nodeName), metav1.GetOptions{})

	if apierr.IsNotFound(err) {
		// Handle not found error.
		glog.Infof("Pod %s not found in namespace %s.", string(nodeName), i.namespace)
		return "", cloudprovider.InstanceNotFound
	} else if err != nil {
		// Handle other errors.
		glog.Errorf("Error getting pod %s: %v.", string(nodeName), err)
		return "", err
	} else if !podutilv1.IsPodReady(pod) {
		// Check if pod is not ready. e.g. if pod stuck in Terminating state or CrashLoopBackoff.
		glog.Infof("Pod %s in namespace %s is not ready.\n", string(nodeName), i.namespace)
		return "", cloudprovider.InstanceNotFound
	}

	// Finally if none of conditions above are met return that pod is OK.
	glog.Infof("Pod %s in namespace %s is ready.\n", string(nodeName), i.namespace)
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
