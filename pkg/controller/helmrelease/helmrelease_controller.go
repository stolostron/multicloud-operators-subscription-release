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
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
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
	return &ReconcileHelmRelease{mgr}
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
			if subRelNew.DeletionTimestamp != nil {
				return true
			}
			if len(subRelOld.GetFinalizers()) != len(subRelNew.GetFinalizers()) {
				// finalizer changes, process it
				return true
			}
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
	manager.Manager
}

// Reconcile reads that state of the cluster for a HelmRelease object and makes changes based on the state read
// and what is in the HelmRelease.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileHelmRelease) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	klog.V(1).Info("Reconciling HelmRelease:", request)
	klog.Error("Reconciling HelmRelease:", request)

	// Fetch the HelmRelease instance
	instance := &appv1alpha1.HelmRelease{}

	err := r.GetClient().Get(context.TODO(), request.NamespacedName, instance)
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

	if instance.DeletionTimestamp == nil && r.prepareHelmRelease(instance) {
		// add finalizer and secret annotation and come again
		return reconcile.Result{}, nil
	}

	// Define a new Pod object
	err = r.manageHelmRelease(instance)

	//if the instance was set for deletion and the finalizer already remove then
	//no need to update the status
	if instance.DeletionTimestamp != nil && !utils.HasFinalizer(instance) {
		return reconcile.Result{}, r.GetClient().Update(context.TODO(), instance)
	}

	return r.SetStatus(instance, err)
}

func (r *ReconcileHelmRelease) manageHelmRelease(sr *appv1alpha1.HelmRelease) error {
	klog.V(3).Info(fmt.Sprintf("chart: %s-%s", sr.Spec.ChartName, sr.Spec.Version))

	klog.V(5).Info("Create Manager")

	helmReleaseManager, err := r.newHelmReleaseManager(sr)

	if err != nil {
		klog.Error(err, "- Failed to create NewManager ", sr.Spec.ChartName)
		return err
	}

	klog.V(5).Info("Sync repo")

	err = helmReleaseManager.Sync(context.TODO())
	if err != nil {
		klog.Error(err, "- Failed to while sync :", sr.Spec.ChartName)
		return err
	}

	if sr.DeletionTimestamp == nil {
		if helmReleaseManager.IsInstalled() {
			klog.Error("Update chart ", sr.Spec.ChartName)

			_, _, err = helmReleaseManager.UpdateRelease(context.TODO())
			if err != nil {
				klog.Error(err, " - Failed to while update chart: ", sr.Spec.ChartName)
				return err
			}
		} else {
			klog.Error("Install chart: ", sr.Spec.ChartName)

			_, err = helmReleaseManager.InstallRelease(context.TODO())
			if err != nil {
				klog.Error(err, " - Failed to while install chart: ", sr.Spec.ChartName)
				return err
			}
		}
	} else {
		klog.Error("Delete chart: ", sr.Spec.ChartName)
		if helmReleaseManager.IsInstalled() {
			_, err = helmReleaseManager.UninstallRelease(context.TODO())
			if err != nil {
				klog.Error(err, " - Failed to while un-install chart: ", sr.Spec.ChartName)
			}
		}
		klog.Info("Remove finalizer from helmrelease : ", sr.Namespace, "/", sr.Name)
		utils.RemoveFinalizer(sr)
	}

	return nil
}

func (r *ReconcileHelmRelease) prepareHelmRelease(sr *appv1alpha1.HelmRelease) bool {
	needUpdate := false

	if !utils.HasFinalizer(sr) {
		klog.Info("Add finalizer: ", sr.Name)
		utils.AddFinalizer(sr)

		needUpdate = true
	}

	annotations := sr.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	exsec := annotations[appv1alpha1.ReleaseSecretAnnotationKey]
	relsec := sr.Namespace + "/" + sr.Spec.ReleaseName

	if exsec == "" {
		exsec = relsec
		annotations[appv1alpha1.ReleaseSecretAnnotationKey] = relsec
		sr.SetAnnotations(annotations)

		needUpdate = true
	}

	if exsec != relsec {
		err := fmt.Errorf("release name can not be changed: new %s, old %s", relsec, exsec)
		_, _ = r.SetStatus(sr, err)

		return true
	}

	if needUpdate {
		err := r.GetClient().Update(context.TODO(), sr)
		if err != nil {
			klog.Error(err, " - Unable to prepare helmrelease:", sr.Namespace, "/", sr.Name)
		}
	}

	return needUpdate
}

//SetStatus set the subscription release status
func (r *ReconcileHelmRelease) SetStatus(instance *appv1alpha1.HelmRelease, issue error) (reconcile.Result, error) {
	//Success
	if issue == nil {
		instance.Status.Message = ""
		instance.Status.Status = appv1alpha1.HelmReleaseSuccess
		instance.Status.Reason = ""
		instance.Status.LastUpdateTime = metav1.Now()

		err := r.GetClient().Status().Update(context.TODO(), instance)
		if err != nil {
			klog.Error(err, " - unable to update status")

			return reconcile.Result{
				RequeueAfter: time.Second,
			}, nil
		}

		return reconcile.Result{}, nil
	}

	klog.Error(issue, " - retrying later")

	var retryInterval time.Duration

	lastUpdate := instance.Status.LastUpdateTime.Time
	lastStatus := instance.Status.Status
	instance.Status.Message = "Error, retrying later"
	instance.Status.Reason = issue.Error()
	instance.Status.Status = appv1alpha1.HelmReleaseFailed
	instance.Status.LastUpdateTime = metav1.Now()

	err := r.GetClient().Status().Update(context.TODO(), instance)
	if err != nil {
		klog.Error(err, " - unable to update status")

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
