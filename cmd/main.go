package main

import (
	"flag"
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/adawolfs/database-controller/pkg/client/clientset/versioned"
	informers "github.com/adawolfs/database-controller/pkg/client/informers/externalversions"
	"k8s.io/sample-controller/pkg/signals"

	"github.com/adawolfs/database-controller/pkg/controller"
	"github.com/adawolfs/database-controller/pkg/config"
	"log"
	"os"
)

var (
	masterURL	string
	kubeconfig	string
	configFile	string
)

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)

	dbconfig, err := config.ParseConfig(configFile)
	if err != nil {
		log.Println("failed to read configuration:", err)
		os.Exit(1)
	}

	controller := controller.NewDatabaseController(kubeClient, exampleClient, kubeInformerFactory, exampleInformerFactory, dbconfig)

	go kubeInformerFactory.Start(stopCh)
	go exampleInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "/home/adawolfs/.kube/config", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "https://cluster.digitalgeko.com", "The address of the Kubernetes API controller. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&configFile, "config", "/home/adawolfs/go/src/github.com/adawolfs/database-controller/cmd/config.yml", "Path to YAML configuration file.")
}
