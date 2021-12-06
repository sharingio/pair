/*
Copyright 2018 The Kubernetes Authors.
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
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog"
	clusterAPIv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"

	"github.com/sharingio/pair/apps/cluster-api-manager/common"
	camk8s "github.com/sharingio/pair/apps/cluster-api-manager/kubernetes"
)

// overwritable variables
var (
	AppBuildVersion            = "0.0.0"
	AppBuildHash               = "???"
	AppBuildDate               = "???"
	AppBuildMode               = "development"
	endpointsForReconciliation = []string{
		"certmanage",
		"dnsmanage",
		"syncProviderID",
	}
)

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func getClusterAPIManagerEndpoint(url string) (response string, err error) {
	if url[:1] == "/" {
		url = url[1:]
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	return string(body), err
}

func main() {
	klog.InitFlags(nil)

	var kubeconfig string
	var leaseLockName string
	var leaseLockNamespace string
	var id string

	flag.StringVar(&id, "id", uuid.New().String(), "the holder identity name")
	flag.StringVar(&leaseLockName, "lease-lock-name", "", "the lease lock resource name")
	flag.StringVar(&leaseLockNamespace, "lease-lock-namespace", "", "the lease lock resource namespace")
	flag.Parse()

	if leaseLockName == "" {
		klog.Fatal("unable to get lease lock resource name (missing lease-lock-name flag).")
	}
	if leaseLockNamespace == "" {
		klog.Fatal("unable to get lease lock resource namespace (missing lease-lock-namespace flag).")
	}
	targetNamespace := common.GetTargetNamespace()
	clusterAPIManagerHost := common.GetEnvOrDefault("APP_CLUSTER_API_MANAGER_HOST", "http://sharingio-pair-clusterapimanager:8080")

	// leader election uses the Kubernetes API by writing to a
	// lock object, which can be a LeaseLock object (preferred),
	// a ConfigMap, or an Endpoints (deprecated) object.
	// Conflicting writes are detected and each client handles those actions
	// independently.
	config, err := buildConfig(kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}
	client := clientset.NewForConfigOrDie(config)
	err, dynamicClientset := camk8s.DynamicClient()
	if err != nil {
		log.Panicln(err)
		return
	}

	run := func(ctx context.Context) {
		// complete your controller loop here
		klog.Info("Controller loop...")

		var clusters []clusterAPIv1alpha3.Cluster
		groupVersion := clusterAPIv1alpha3.GroupVersion
		groupVersionResource := schema.GroupVersionResource{
			Version:  groupVersion.Version,
			Group:    groupVersion.Group,
			Resource: "clusters",
		}
		items, err := dynamicClientset.
			Resource(groupVersionResource).
			Namespace(targetNamespace).
			List(context.TODO(), metav1.ListOptions{})
		if err != nil && apierrors.IsNotFound(err) != true {
			log.Printf("Error: failed to list Clusters: %#v\n", err)
			return
		}
		for _, item := range items.Items {
			var c clusterAPIv1alpha3.Cluster
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &c)
			if err != nil {
				log.Println("Error: failed to restructure %T: error: %v", c, err)
				return
			}
			if c.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
				log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, c, c.ObjectMeta.Name)
				continue
			}
			clusters = append(clusters, c)
		}

		for _, c := range clusters {
			log.Printf("Reconciling Pair instance '%v'\n", c.ObjectMeta.Name)
			for _, e := range endpointsForReconciliation {
				log.Printf("Trying cluster-api-manager endpoint '%s'\n", e)
				go func() {
					resp, err := getClusterAPIManagerEndpoint(fmt.Sprintf("%s/api/instance/kubernetes/%s/%s", clusterAPIManagerHost, c.ObjectMeta.Name, e))
					if err != nil {
						log.Printf("Error from cluster-api-manager endpoint '%s'\n", e, err)
					}
					log.Printf("Response from cluster-api-manager endpoint '%v'\n", e, resp)
				}()
			}
		}
	}

	// use a Go context so we can tell the leaderelection code when we
	// want to step down
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// listen for interrupts or the Linux SIGTERM signal and cancel
	// our context, which the leader election code will observe and
	// step down
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		klog.Info("Received termination, signaling shutdown")
		cancel()
	}()

	// we use the Lease lock type since edits to Leases are less common
	// and fewer objects in the cluster watch "all Leases".
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: leaseLockNamespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	// start the leader election code loop
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock: lock,
		// IMPORTANT: you MUST ensure that any code you have that
		// is protected by the lease must terminate **before**
		// you call cancel. Otherwise, you could have a background
		// loop still running and another process could
		// get elected before your background loop finished, violating
		// the stated goal of the lease.
		ReleaseOnCancel: true,
		LeaseDuration:   60 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// we're notified when we start - this is where you would
				// usually put your code
				run(ctx)
			},
			OnStoppedLeading: func() {
				// we can do cleanup here
				klog.Infof("leader lost: %s", id)
				os.Exit(0)
			},
			OnNewLeader: func(identity string) {
				// we're notified when new leader elected
				if identity == id {
					// I just got the lock
					return
				}
				klog.Infof("new leader elected: %s", identity)
			},
		},
	})
}
