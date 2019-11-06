/*
Copyright 2019 The Kubernetes Authors.

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

//Package helmrelease controller manages the helmreleas CR
package helmrelease

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
	"github.com/IBM/multicloud-operators-subscription-release/pkg/helmreleasemgr"
	"github.com/IBM/multicloud-operators-subscription-release/pkg/utils"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new HelmRelease Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileHelmRelease{config: mgr.GetConfig(), client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	chartsDir := os.Getenv(appv1alpha1.ChartsDir)
	if chartsDir == "" {
		chartsDir, err := ioutil.TempDir("/tmp", "charts")
		if err != nil {
			return err
		}

		err = os.Setenv(appv1alpha1.ChartsDir, chartsDir)
		if err != nil {
			return err
		}
	}

	// Create a new controller
	c, err := controller.New("helmrelease-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource HelmRelease
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			subRelOld := e.ObjectOld.(*appv1alpha1.HelmRelease)
			subRelNew := e.ObjectNew.(*appv1alpha1.HelmRelease)
			return !reflect.DeepEqual(subRelOld.Spec, subRelNew.Spec)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}

	err = c.Watch(&source.Kind{Type: &appv1alpha1.HelmRelease{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileHelmRelease implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileHelmRelease{}

// ReconcileHelmRelease reconciles a HelmRelease object
type ReconcileHelmRelease struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	config *rest.Config
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a HelmRelease object and makes changes based on the state read
// and what is in the HelmRelease.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileHelmRelease) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	klog.Info("Reconciling HelmRelease")

	// Fetch the HelmRelease instance
	instance := &appv1alpha1.HelmRelease{}

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
	err = r.manageHelmRelease(instance)

	return r.SetStatus(instance, err)
}

func (r *ReconcileHelmRelease) manageHelmRelease(sr *appv1alpha1.HelmRelease) error {
	klog.V(3).Info(fmt.Sprintf("chart: %s-%s", sr.Spec.ChartName, sr.Spec.Version))

	configMap, err := utils.GetConfigMap(r.client, sr.Namespace, sr.Spec.ConfigMapRef)
	if err != nil {
		return err
	}

	secret, err := utils.GetSecret(r.client, sr.Namespace, sr.Spec.SecretRef)
	if err != nil {
		klog.Error(err, "Failed to retrieve secret ", sr.Spec.SecretRef.Name)
		return err
	}

	klog.V(5).Info("Create Manager")

	helmReleaseManager, err := helmreleasemgr.NewHelmReleaseManager(r.config, configMap, secret, sr)
	if err != nil {
		klog.Error(err, "Failed to create NewManager ", sr.Spec.ChartName)
		return err
	}

	klog.V(5).Info("Sync repo")

	err = helmReleaseManager.Sync(context.TODO())
	if err != nil {
		klog.Error(err, "Failed to while sync :", sr.Spec.ChartName)
		return err
	}

	if helmReleaseManager.IsInstalled() {
		klog.Info("Update chart ", sr.Spec.ChartName)

		_, _, err = helmReleaseManager.UpdateRelease(context.TODO())
		if err != nil {
			klog.Error(err, "Failed to while update chart: ", sr.Spec.ChartName)
			return err
		}
	} else {
		klog.Info("Install chart: ", sr.Spec.ChartName)

		_, err = helmReleaseManager.InstallRelease(context.TODO())
		if err != nil {
			klog.Error(err, "Failed to while install chart: ", sr.Spec.ChartName)
			return err
		}
	}

	return nil
}

//SetStatus set the subscription release status
func (r *ReconcileHelmRelease) SetStatus(s *appv1alpha1.HelmRelease, issue error) (reconcile.Result, error) {
	//Success
	if issue == nil {
		s.Status.Message = ""
		s.Status.Status = appv1alpha1.HelmReleaseSuccess
		s.Status.Reason = ""
		s.Status.LastUpdateTime = metav1.Now()

		err := r.client.Status().Update(context.Background(), s)
		if err != nil {
			klog.Error(err, "unable to update status")

			return reconcile.Result{
				RequeueAfter: time.Second,
			}, nil
		}

		return reconcile.Result{}, nil
	}

	var retryInterval time.Duration

	lastUpdate := s.Status.LastUpdateTime.Time
	lastStatus := s.Status.Status
	s.Status.Message = "Error, retrying later"
	s.Status.Reason = issue.Error()
	s.Status.Status = appv1alpha1.HelmReleaseFailed
	s.Status.LastUpdateTime = metav1.Now()

	err := r.client.Status().Update(context.Background(), s)
	if err != nil {
		klog.Error(err, "unable to update status")

		return reconcile.Result{
			RequeueAfter: time.Second,
		}, nil
	}

	if lastUpdate.IsZero() || lastStatus != appv1alpha1.HelmReleaseFailed {
		retryInterval = time.Second
	} else {
		retryInterval = time.Duration(math.Max(float64(time.Second.Nanoseconds()*2), float64(metav1.Now().Sub(lastUpdate).Round(time.Second).Nanoseconds())))
	}

	requeueAfter := time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Minute.Nanoseconds()*2)))
	klog.V(5).Info("requeueAfter: ", requeueAfter)

	return reconcile.Result{
		RequeueAfter: requeueAfter,
	}, nil
}
