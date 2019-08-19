package subscriptionrelease

import (
	"context"
	"math"
	"reflect"
	"time"

	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/subscriptionreleasemgr"
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_subscriptionrelease")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new SubscriptionRelease Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSubscriptionRelease{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("subscriptionrelease-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SubscriptionRelease
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			subRelOld := e.ObjectOld.(*appv1alpha1.SubscriptionRelease)
			subRelNew := e.ObjectNew.(*appv1alpha1.SubscriptionRelease)
			if !reflect.DeepEqual(subRelOld.Spec, subRelNew.Spec) {
				return true
			}
			if subRelNew.Status.Status == subRelOld.Status.Status {
				return false
			}
			return true
		},
	}
	err = c.Watch(&source.Kind{Type: &appv1alpha1.SubscriptionRelease{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner SubscriptionRelease
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.SubscriptionRelease{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSubscriptionRelease implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSubscriptionRelease{}

// ReconcileSubscriptionRelease reconciles a SubscriptionRelease object
type ReconcileSubscriptionRelease struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a SubscriptionRelease object and makes changes based on the state read
// and what is in the SubscriptionRelease.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSubscriptionRelease) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SubscriptionRelease")

	// Fetch the SubscriptionRelease instance
	instance := &appv1alpha1.SubscriptionRelease{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Define a new Pod object
	err = r.manageSubcriptionRelease(instance)
	return r.SetStatus(instance, err)
}

func (r *ReconcileSubscriptionRelease) manageSubcriptionRelease(sr *appv1alpha1.SubscriptionRelease) error {
	srLogger := log.WithValues("SubscriptionRelease.Namespace", sr.Namespace, "SubscrptionRelease.Name", sr.Name)
	srLogger.Info("chart: ", "sr.Spec.ChartName", sr.Spec.ChartName, "sr.Spec.Version", sr.Spec.Version)
	configMap, err := utils.GetConfigMap(r.client, sr.Namespace, sr.Spec.ConfigMapRef)
	if err != nil {
		return err
	}
	httpClient, err := utils.GetHelmRepoClient(r.client, sr.Namespace, configMap)
	if err != nil {
		srLogger.Error(err, "Failed to create httpClient ", "sr.Spec.SecretRef.Name", sr.Spec.SecretRef.Name)
		return err
	}
	secret, err := utils.GetSecret(r.client, sr.Namespace, sr.Spec.SecretRef)
	if err != nil {
		srLogger.Error(err, "Failed to retrieve secret ", "sr.Spec.SecretRef.Name", sr.Spec.SecretRef.Name)
		return err
	}
	srLogger.Info("Create Manager")
	mgr, err := subscriptionreleasemgr.NewManager(httpClient, secret, sr)
	if err != nil {
		srLogger.Error(err, "Failed to create NewManager ", "sr.Spec.ChartName", sr.Spec.ChartName)
		return err
	}
	srLogger.Info("Sync repo")
	err = mgr.Sync(context.TODO())
	if err != nil {
		srLogger.Error(err, "Failed to while sync ", "sr.Spec.ChartName", sr.Spec.ChartName)
		return err
	}
	if mgr.IsInstalled() {
		srLogger.Info("Update chart", "sr.Spec.ChartName", sr.Spec.ChartName)
		_, _, err = mgr.UpdateRelease(context.TODO())
		if err != nil {
			srLogger.Error(err, "Failed to while sync ", "sr.Spec.ChartName", sr.Spec.ChartName)
			return err
		}
	} else {
		srLogger.Info("Install chart", "sr.Spec.ChartName", sr.Spec.ChartName)
		_, err = mgr.InstallRelease(context.TODO())
		if err != nil {
			srLogger.Error(err, "Failed to while sync ", "sr.Spec.ChartName", sr.Spec.ChartName)
			return err
		}
	}
	return nil
}

func (r *ReconcileSubscriptionRelease) SetStatus(s *appv1alpha1.SubscriptionRelease, issue error) (reconcile.Result, error) {
	srLogger := log.WithValues("SubscriptionRelease.Namespace", s.GetNamespace(), "SubscriptionRelease.Name", s.GetName())
	//Success
	if issue == nil {
		s.Status.Message = ""
		s.Status.Status = appv1alpha1.SubscriptionReleaseSuccess
		s.Status.Reason = ""
		s.Status.LastUpdateTime = metav1.Now()
		err := r.client.Status().Update(context.Background(), s)
		if err != nil {
			srLogger.Error(err, "unable to update status")
			return reconcile.Result{
				RequeueAfter: time.Second,
			}, nil
		}
		return reconcile.Result{}, nil
	}
	var retryInterval time.Duration
	//r.Recorder.Event(s, "Warning", "ProcessingError", issue.Error())
	lastUpdate := s.Status.LastUpdateTime.Time
	lastStatus := s.Status.Status
	s.Status.Message = "Error, retrying later"
	s.Status.Reason = issue.Error()
	s.Status.Status = appv1alpha1.SubscriptionReleaseFailed
	s.Status.LastUpdateTime = metav1.Now()

	err := r.client.Status().Update(context.Background(), s)
	if err != nil {
		srLogger.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: time.Second,
		}, nil
	}
	if lastUpdate.IsZero() || lastStatus != appv1alpha1.SubscriptionReleaseFailed {
		retryInterval = time.Second
	} else {
		//retryInterval = time.Duration(math.Max(float64(time.Second.Nanoseconds()*2), float64(metav1.Now().Sub(lastUpdate).Round(time.Second).Nanoseconds())))
		retryInterval = s.Status.LastUpdateTime.Sub(lastUpdate).Round(time.Second)
	}
	requeueAfter := time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Hour.Nanoseconds()*6)))
	srLogger.Info("requeueAfter", "->requeueAfter", requeueAfter)
	return reconcile.Result{
		RequeueAfter: requeueAfter,
	}, nil
}
