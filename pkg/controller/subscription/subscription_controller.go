package subscription

import (
	"context"

	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/helmreposubscriber"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_subscription")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Subscription Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	subscriberMap := make(map[string]appv1alpha1.Subscriber)
	return &ReconcileSubscription{client: mgr.GetClient(), scheme: mgr.GetScheme(), subscriberMap: subscriberMap}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("subscription-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Subscription
	err = c.Watch(&source.Kind{Type: &appv1alpha1.Subscription{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Subscription
	err = c.Watch(&source.Kind{Type: &appv1alpha1.SubscriptionRelease{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.Subscription{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSubscription implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSubscription{}

// ReconcileSubscription reconciles a Subscription object
type ReconcileSubscription struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client        client.Client
	scheme        *runtime.Scheme
	subscriberMap map[string]appv1alpha1.Subscriber
}

// Reconcile reads that state of the cluster for a Subscription object and makes changes based on the state read
// and what is in the Subscription.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSubscription) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Subscription")

	// Fetch the Subscription instance
	instance := &appv1alpha1.Subscription{}
	subkey := request.NamespacedName.String()
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Subscription deleted but request already created")
			r.cleanSubscriber(subkey)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Error reading the object - requeue the request")
		return reconcile.Result{}, err
	}

	myFinalizerName := "monitor.subscription.app.ibm.com"
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(instance.ObjectMeta.Finalizers, myFinalizerName) {
			reqLogger.Info("Add Finalizer")
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, myFinalizerName)
			if err := r.client.Update(context.TODO(), instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(instance.ObjectMeta.Finalizers, myFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			reqLogger.Info("Subscription deleted, Finalizing!")
			r.cleanSubscriber(subkey)

			// remove our finalizer from the list and update it.
			instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, myFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{}, err

	}

	subscriber := r.subscriberMap[subkey]
	if subscriber == nil {
		reqLogger.Info("subscriber does not exist")
		subscriber = &helmreposubscriber.HelmRepoSubscriber{
			Client:       r.client,
			Scheme:       r.scheme,
			Subscription: instance,
		}
		reqLogger.Info("Subscription", "subscription.Name", instance.Name, "configMapRef", instance.Spec.ConfigMapRef)
		r.subscriberMap[subkey] = subscriber
		err = subscriber.Restart()
	} else {
		reqLogger.Info("subscriber does exist")
		err = subscriber.Update(instance)
	}
	//If the subscriber didn't start then clean
	if !subscriber.IsStarted() {
		reqLogger.Info("Subscription didn't start")
		r.cleanSubscriber(subkey)
	}
	if err != nil {
		reqLogger.Error(err, "Error processing subscription - requeue the request")
		return reconcile.Result{}, err
	}
	// Set Subscription instance as the owner and controller
	return reconcile.Result{}, nil
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func (r *ReconcileSubscription) cleanSubscriber(subkey string) {
	reqLogger := log.WithValues("subkey", subkey)
	subscriber := r.subscriberMap[subkey]
	if subscriber != nil {
		reqLogger.Info("Cleaning subscriber map and stopping subscriber")
		subscriber.Stop()
		delete(r.subscriberMap, subkey)
	}
}
