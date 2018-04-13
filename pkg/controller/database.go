package controller

import (
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/api/core/v1"

	informers "github.com/adawolfs/database-controller/pkg/client/informers/externalversions"
	clientset "github.com/adawolfs/database-controller/pkg/client/clientset/versioned"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	localSchema "github.com/adawolfs/database-controller/pkg/client/clientset/versioned/scheme"
	listers "github.com/adawolfs/database-controller/pkg/client/listers/adawolfs.com/v1"
	"k8s.io/client-go/tools/record"
	"github.com/golang/glog"
	"fmt"
)

type DatabaseController struct {
	// kubeclientset is a standard kubernetes clientset
	kubeClientSet kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	customClientSet clientset.Interface

	DBConfig  *DBConfig

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced

	dbLister	listers.DatabaseLister
	dbSynced	cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder	record.EventRecorder
}

const controllerAgentName = "database-controller"

func NewDatabaseController(
	kubeClientSet kubernetes.Interface,
	customClientSet clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	customInformerFactory informers.SharedInformerFactory) *DatabaseController {

	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()

	//Obtain references to shared index informers for databases
	dbInformer := customInformerFactory.Adawolfs().V1().Databases()

	// Add sample-controller types to the default Kubernetes Scheme so Events can be logged for sample-controller types.
	localSchema.AddToScheme(scheme.Scheme)

	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &DatabaseController{
		kubeClientSet:		kubeClientSet,
		customClientSet:	customClientSet,
		deploymentsLister:	deploymentInformer.Lister(),
		deploymentsSynced:	deploymentInformer.Informer().HasSynced,
		dbLister:			dbInformer.Lister(),
		dbSynced:			dbInformer.Informer().HasSynced,
		workqueue:			workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Databases"),
		recorder: 			recorder,
	}

	glog.Info("Setting up event handlers")

	// Set up an event handler for when Database resources change
	dbInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueDb,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueDb(new)
		},
	})

	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Foo resource will enqueue that Foo resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}


// handleObject will take any resource implementing metav1.Object and attempt
// to find the Database resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Database resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *DatabaseController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Foo, we should not do anything more
		// with it.
		if ownerRef.Kind != "Foo" {
			return
		}

		db, err := c.dbLister.Databases(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of foo '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueDb(db)
		return
	}
}


// enqueueDb takes a Database resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Databases.
func (c *DatabaseController) enqueueDb(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}