package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

const controllerAgentName = "node-label-controller"

func main() {
	kubeconfigFile := os.Getenv("HOME") + "/.kube/config"
	kubeconfig := flag.String("kubeconfig", kubeconfigFile, "Kubeconfig File location")
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		klog.Infof("erorr %s building config from flags.. trying from inside the cluster", err.Error())
		config, err = rest.InClusterConfig()
		if err != nil {
			klog.Fatalf("Error building kubeconfig: %s", err.Error())
		}
	}
	kubeclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	klog.Info("List all nodes ")
	nodes, err := kubeclient.CoreV1().Nodes().List(context.Background(), v1.ListOptions{})
	if err != nil {
		klog.Fatalf("error %s, listing nodes\n", err.Error())
	}

	for _, node := range nodes.Items {
		klog.Info(node.Name)
	}

	//If we want to create informer for specific resources
	labelOptions := informers.WithTweakListOptions(func(opts *v1.ListOptions) {
		//opts.LabelSelector = "app=nats-box"
	})

	// By default NewSharedInformerFactory creates informerfactory for all Namespaces
	// Use NewSharedInformerFactoryWithOptions for creating informer instance in specific namespace
	kubeinformerfactory := informers.NewSharedInformerFactoryWithOptions(kubeclient, 30*time.Second, labelOptions)

	controller := newController(kubeclient, kubeinformerfactory.Core().V1().Nodes())

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := SetupSignalHandler()

	kubeinformerfactory.Start(stopCh)

	if err = controller.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func SetupSignalHandler() (stopCh <-chan struct{}) {
	close(make(chan struct{})) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}
