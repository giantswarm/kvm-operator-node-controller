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

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	nodeutilv1 "k8s.io/kubernetes/pkg/api/v1/node"
	"k8s.io/kubernetes/pkg/cloudprovider"

	_ "github.com/giantswarm/kvm-operator-node-controller/provider"
)

const (
	// nodeStatusUpdateRetry controls the number of retries of writing NodeStatus update.
	nodeStatusUpdateRetry = 5

	// The amount of time the nodecontroller should sleep between retrying NodeStatus updates
	retrySleepTime = 20 * time.Millisecond

	// The amount of time  between node status checks
	nodeMonitorPeriod = 30 * time.Second
)

// Common variables.
var (
	description = "Calico node controller for Kubernetes."
	gitCommit   = "n/a"
	name        = "calico-node-controller"
	source      = "https://github.com/giantswarm/calico-node-controller"
)

type controller struct {
	kubeClient        clientset.Interface
	cloud             cloudprovider.Interface
	nodeMonitorPeriod time.Duration
}

func main() {
	// Print version.
	if (len(os.Args) > 1) && (os.Args[1] == "version") {
		fmt.Printf("Description:    %s\n", description)
		fmt.Printf("Git Commit:     %s\n", gitCommit)
		fmt.Printf("Go Version:     %s\n", runtime.Version())
		fmt.Printf("Name:           %s\n", name)
		fmt.Printf("OS / Arch:      %s / %s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Source:         %s\n", source)
		return
	}

	var kubeconfig string
	var master string
	var help bool

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.BoolVar(&help, "help", false, "Print usage and exit")
	flag.Parse()

	// Print usage.
	if help {
		flag.Usage()
		return
	}

	// Create kubeclient config.
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		glog.Fatal(err)
	}

	// Create the client.
	kubeClient, err := clientset.NewForConfig(config)
	if err != nil {
		glog.Fatal(err)
	}

	// Initialize kubernetes provider.
	cloud, err := cloudprovider.InitCloudProvider("kubernetes", "")
	if err != nil {
		glog.Fatalf("failed to initialize cloud provider: %v", err)
	}

	// Create controller instance.
	controller := newController(kubeClient, cloud, nodeMonitorPeriod)

	// Start the kvm-operator node controller.
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	// Wait forever.
	select {}
}

// Creates new instance of controller.
func newController(
	kubeClient clientset.Interface,
	cloud cloudprovider.Interface,
	nodeMonitorPeriod time.Duration) *controller {
	return &controller{
		kubeClient:        kubeClient,
		cloud:             cloud,
		nodeMonitorPeriod: nodeMonitorPeriod,
	}
}

// Run controller function in a loop.
func (c *controller) Run(stopCh chan struct{}) {
	defer utilruntime.HandleCrash()

	// Let the workers stop when we are done
	glog.Info("Starting kvm-operator node controller")

	// Start a loop to periodically check if any nodes have been deleted from cloudprovider
	go wait.Until(c.MonitorNode, c.nodeMonitorPeriod, stopCh)

	<-stopCh
	glog.Info("Stopping kvm-operator node controller")
}

// Monitor node queries the cloudprovider for non-ready nodes and deletes them
// if they cannot be found in the cloud provider.
func (c *controller) MonitorNode() {
	// Get nodes from cloud provider.
	instances, ok := c.cloud.Instances()
	if !ok {
		utilruntime.HandleError(fmt.Errorf("failed to get instances from cloud provider"))
		return
	}

	// Get nodes known by kubernetes cluster.
	nodes, err := c.kubeClient.CoreV1().Nodes().List(metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		glog.Errorf("Error monitoring node status: %v", err)
		return
	}

	for i := range nodes.Items {
		var currentReadyCondition *v1.NodeCondition
		node := &nodes.Items[i]

		glog.Infof("Checking node %s", node.Name)

		// Try to get the current node status
		// If node status is empty, then kubelet has not posted ready status yet. In this case,
		// try nodeStatusUpdateRetry times. If still nothing than give up this node and process next node.
		for rep := 0; rep < nodeStatusUpdateRetry; rep++ {
			_, currentReadyCondition = nodeutilv1.GetNodeCondition(&node.Status, v1.NodeReady)
			if currentReadyCondition != nil {
				break
			}
			name := node.Name
			node, err = c.kubeClient.CoreV1().Nodes().Get(name, metav1.GetOptions{})
			if err != nil {
				glog.Errorf("Failed while getting a Node to retry updating NodeStatus. Probably Node %s was deleted.", name)
				break
			}
			time.Sleep(retrySleepTime)
		}
		if currentReadyCondition == nil {
			glog.Errorf("Update status of Node %v from Controller exceeds retry count or the Node was deleted.", node.Name)
			continue
		}
		// If the known node status says that Node is NotReady, then check if the node has been removed
		// from the cloud provider. If node cannot be found in cloudprovider, then delete the node immediately.
		if currentReadyCondition != nil {
			if currentReadyCondition.Status != v1.ConditionTrue {
				// Check with the cloud provider to see if the node still exists. If it
				// doesn't, delete the node immediately.
				exists, err := ensureNodeExists(instances, node)
				if err != nil {
					glog.Errorf("Error getting data for node %s from cloud provider: %v", node.Name, err)
					continue
				}

				if exists {
					// Continue checking the remaining nodes since the current one is fine.
					continue
				}

				glog.Infof("Deleting node since it is no longer present in cloud provider: %s", node.Name)

				go func(nodeName string) {
					defer utilruntime.HandleCrash()
					if err := c.kubeClient.CoreV1().Nodes().Delete(nodeName, nil); err != nil {
						glog.Errorf("unable to delete node %q: %v", nodeName, err)
					}
				}(node.Name)

			}
		}
		glog.Infof("Node %s Ready state is %s.", node.Name, currentReadyCondition.Status)
	}
}

// Checks if the instance exists in host cluster
func ensureNodeExists(instances cloudprovider.Instances, node *v1.Node) (bool, error) {
	_, err := instances.ExternalID(types.NodeName(node.Name))
	if err != nil {
		if err == cloudprovider.InstanceNotFound {
			return false, nil
		}
		return false, fmt.Errorf("ensureNodeExists: Error fetching by NodeName: %v", err)
	}

	return true, nil
}
