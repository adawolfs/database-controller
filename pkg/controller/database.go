package controller

import (
	"k8s.io/client-go/kubernetes"
	//appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	informers "github.com/adawolfs/database-controller/pkg/client/informers/externalversions"
	clientset "github.com/adawolfs/database-controller/pkg/client/clientset/versioned"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	localSchema "github.com/adawolfs/database-controller/pkg/client/clientset/versioned/scheme"
	listers "github.com/adawolfs/database-controller/pkg/client/listers/adawolfs.com/v1"
	

	adawolfscomv1 "github.com/adawolfs/database-controller/pkg/apis/adawolfs.com/v1"
	
	"k8s.io/client-go/tools/record"
	"github.com/golang/glog"
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"
)

type DatabaseController struct {
	// kubeclientset is a standard kubernetes clientset
	kubeClientSet kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	customClientSet clientset.Interface

	//deploymentsLister appslisters.DeploymentLister
	//deploymentsSynced cache.InformerSynced

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

const (

	controllerAgentName = "database-controller"
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "Foo synced successfully"
)

func NewDatabaseController(
	kubeClientSet kubernetes.Interface,
	customClientSet clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	customInformerFactory informers.SharedInformerFactory) *DatabaseController {

	//deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()

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
		//deploymentsLister:	deploymentInformer.Lister(),
		//deploymentsSynced:	deploymentInformer.Informer().HasSynced,
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
	// owned by a Database resource will enqueue that Database resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	//deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: controller.handleObject,
	//	UpdateFunc: func(old, new interface{}) {
	//		newDepl := new.(*appsv1.Deployment)
	//		oldDepl := old.(*appsv1.Deployment)
	//		if newDepl.ResourceVersion == oldDepl.ResourceVersion {
	//			// Periodic resync will send update events for all known Deployments.
	//			// Two different versions of the same Deployment will always have different RVs.
	//			return
	//		}
	//		controller.handleObject(new)
	//	},
	//	DeleteFunc: controller.handleObject,
	//})

	return controller
}



// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *DatabaseController) Run(threadiness int, stopCh <-chan struct{}) error {

	//Defers the execution until the funcion RUN returns
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Database controller")
	fmt.Println("Starting Database controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.dbSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch a given workers to process Database resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *DatabaseController) runWorker() {
	for c.processNextWorkItem(){}
}


// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *DatabaseController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Database resource
// with the current status of the resource.
func (c *DatabaseController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the DB resource with this namespace/name
	db, err := c.dbLister.Databases(namespace).Get(name)
	if err != nil {
		// The DB resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("database '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	glog.Info("Handling: '%s", db.ObjectMeta.Name)

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	err = c.updateDBStatus(db)
	if err != nil {
		return err
	}

	c.recorder.Event(db, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}


func (c *DatabaseController) updateDBStatus(db *adawolfscomv1.Database) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	dbCopy := db.DeepCopy()
	dbCopy.Annotations["handled"] = "true"

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Foo resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.customClientSet.AdawolfsV1().Databases(db.Namespace).Update(dbCopy)
	return err
}

/*------------------------------------Handle Objects and add them to the queue---------------------------------------*/

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
		// If this object is not owned by a Database, we should not do anything more with it.
		if ownerRef.Kind != "Database" {
			return
		}
		// Get object reference
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