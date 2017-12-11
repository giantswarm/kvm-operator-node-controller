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
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/v1"
	nodeutilv1 "k8s.io/kubernetes/pkg/api/v1/node"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/cloudprovider"

	"github.com/giantswarm/certificatetpr"
	_ "github.com/giantswarm/kvm-operator-node-controller/provider"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
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

var errorCount uint64

type controller struct {
	kubeClient        clientset.Interface
	cloud             cloudprovider.Interface
	nodeMonitorPeriod time.Duration
	logger            micrologger.Logger
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

	var clusterAPI string
	var clusterID string
	var help bool

	flag.StringVar(&clusterAPI, "cluster-api", "", "kubernetes api endpoint of guest cluster")
	flag.StringVar(&clusterID, "cluster-id", "", "id of guest cluster (e.g. XXXXX)")
	flag.BoolVar(&help, "help", false, "print usage and exit")
	flag.Parse()

	// Print usage.
	if help {
		flag.Usage()
		return
	}

	// Check clusterAPI and clusterID are set.
	if clusterAPI == "" || clusterID == "" {
		panic(fmt.Sprint("guest cluster api and id must be set"))
	}

	// Get guest cluster client.
	kubeClient, err := newGuestClientFromSecret(clusterAPI, clusterID)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize kubernetes client: %v", err))
	}

	// Initialize kubernetes provider.
	cloud, err := cloudprovider.InitCloudProvider("kubernetes", "")
	if err != nil {
		panic(fmt.Sprintf("failed to initialize cloud provider: %v", err))
	}

	// Create controller instance.
	controller := newController(kubeClient, cloud, nodeMonitorPeriod)

	// Start the kvm-operator node controller.
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	// Start healthz endpoint.
	go func() {
		http.HandleFunc("/healthz", healthzHandler)

		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			panic(fmt.Sprintf("failed to setup healthz endpoint: %v", err))
		}
	}()

	// Wait forever.
	select {}
}

// Increments error count by 1.
func (c *controller) countErr() {
	atomic.AddUint64(&errorCount, 1)
}

// Return http 500 if error counter value is not 0.
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadUint64(&errorCount) != 0 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Creates new instance of controller.
func newController(
	kubeClient clientset.Interface,
	cloud cloudprovider.Interface,
	nodeMonitorPeriod time.Duration) *controller {

	logger, err := micrologger.New(micrologger.DefaultConfig())
	if err != nil {
		microerror.Mask(err)
	}

	return &controller{
		kubeClient:        kubeClient,
		cloud:             cloud,
		nodeMonitorPeriod: nodeMonitorPeriod,
		logger:            logger,
	}
}

// Run controller function in a loop.
func (c *controller) Run(stopCh chan struct{}) {
	defer utilruntime.HandleCrash()

	// Let the workers stop when we are done
	c.logger.Log("info", "starting kvm-operator node controller")

	// Start a loop to periodically check if any nodes have been deleted from cloudprovider
	go wait.Until(c.MonitorNode, c.nodeMonitorPeriod, stopCh)

	<-stopCh
	c.logger.Log("info", "stopping kvm-operator node controller")
}

// Monitor node queries the cloudprovider for non-ready nodes and deletes them
// if they cannot be found in the cloud provider.
func (c *controller) MonitorNode() {
	// Get nodes from cloud provider.
	instances, ok := c.cloud.Instances()
	if !ok {
		utilruntime.HandleError(fmt.Errorf("failed to get instances from cloud provider"))
		c.countErr()
		return
	}

	// Get nodes known by kubernetes cluster.
	nodes, err := c.kubeClient.CoreV1().Nodes().List(metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		c.logger.Log("error", "error monitoring node status", "trace", microerror.Mask(err))
		c.countErr()
		return
	}

	for i := range nodes.Items {
		var currentReadyCondition *v1.NodeCondition
		node := &nodes.Items[i]

		c.logger.Log("info", "checking node status", "node", node.Name)

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
				c.logger.Log(
					"info", "failed while getting a node to retry updating NodeStatus. Probably node was deleted.",
					"node", name,
				)
				break
			}
			time.Sleep(retrySleepTime)
		}
		if currentReadyCondition == nil {
			c.logger.Log("error", "update status of node from Controller exceeds retry count or the Node was deleted", "node", node.Name)
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
					c.logger.Log(
						"error", "error getting data for node from cloud provider",
						"node", node.Name,
						"trace", microerror.Mask(err),
					)
					c.countErr()
					continue
				}

				if exists {
					// Continue checking the remaining nodes since the current one is fine.
					continue
				}

				c.logger.Log(
					"info", "deleting node since it is no longer present in cloud provider",
					"node", node.Name,
				)

				go func(nodeName string) {
					defer utilruntime.HandleCrash()
					if err := c.kubeClient.CoreV1().Nodes().Delete(nodeName, nil); err != nil {
						c.logger.Log(
							"error", "unable to delete node",
							"node", node.Name,
							"trace", microerror.Mask(err),
						)
						c.countErr()
					}
				}(node.Name)

			}
		}
		c.logger.Log(
			"info", "node state",
			"node", node.Name,
			"state", currentReadyCondition.Status,
		)
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

func newGuestClientFromSecret(clusterAPI, clusterID string) (clientset.Interface, error) {
	// Get host cluster client to retrieve certificates.
	var secret *v1.Secret
	{
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, microerror.Mask(err)
		}
		// creates the clientset
		hostClient, err := clientset.NewForConfig(config)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		// Get API secret with certificates.
		secretName := fmt.Sprintf("%s-api", clusterID)

		manifest, err := hostClient.CoreV1().Secrets(metav1.NamespaceDefault).Get(secretName, metav1.GetOptions{})
		if err != nil {
			return nil, microerror.Mask(err)
		}
		secret = manifest
	}

	// XXX(r7vme): Not using operatorkit k8s client here, because it does not support
	// TLS certificates as raw data, only supports files.
	//
	// Initialize client config
	var restConfig *rest.Config

	restConfig = &rest.Config{
		Host: clusterAPI,
		TLSClientConfig: rest.TLSClientConfig{
			CertData: secret.Data[string(certificatetpr.Crt)],
			KeyData:  secret.Data[string(certificatetpr.Key)],
			CAData:   secret.Data[string(certificatetpr.CA)],
		},
		Timeout: 30 * time.Second,
	}

	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return client, nil
}
