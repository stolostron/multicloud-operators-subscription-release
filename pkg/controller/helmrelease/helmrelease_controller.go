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
	"os"
	"time"

	helmController "github.com/operator-framework/operator-sdk/pkg/helm/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
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
	if err := c.Watch(&source.Kind{Type: &appv1alpha1.HelmRelease{}}, &handler.EnqueueRequestForObject{},
		predicate.GenerationChangedPredicate{}); err != nil {
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

	helmReleaseManager, err := r.newHelmReleaseManager(instance)
	if err != nil {
		errUpdate := setErrorStatus(r.GetClient(), instance, err)
		if errUpdate != nil {
			return reconcile.Result{}, errUpdate
		}

		klog.V(1).Info("Requeue after two minutes.")

		return reconcile.Result{RequeueAfter: time.Minute * 2}, nil
	}

	hor := &helmController.HelmOperatorReconciler{
		Client:         r.GetClient(),
		GVK:            instance.GroupVersionKind(),
		ManagerFactory: helmReleaseManager,
	}

	result, err := hor.Reconcile(request)
	if err != nil {
		klog.Error(err, "- Failed during HelmOperator Reconcile.")

		errGet := r.GetClient().Get(context.TODO(), request.NamespacedName, instance)
		if errGet == nil {
			if !containsErrorConditions(instance) {
				errUpdate := setErrorStatus(r.GetClient(), instance, err)
				if errUpdate != nil {
					return reconcile.Result{}, errUpdate
				}
			}
		} else {
			klog.Error(errGet, "- Failed to get HelmRelease: ", request.NamespacedName)
			return reconcile.Result{}, errGet
		}

		klog.V(1).Info("Requeue after two minutes.")

		return reconcile.Result{RequeueAfter: time.Minute * 2}, nil
	}

	return result, nil
}

func containsErrorConditions(hr *appv1alpha1.HelmRelease) bool {
	if hr.Status.Conditions == nil {
		return false
	}

	for i := range hr.Status.Conditions {
		if hr.Status.Conditions[i].Type == appv1alpha1.ConditionIrreconcilable ||
			hr.Status.Conditions[i].Type == appv1alpha1.ConditionReleaseFailed {
			return true
		}
	}

	return false
}

func setErrorStatus(client client.StatusClient, hr *appv1alpha1.HelmRelease, err error) error {
	klog.V(1).Info(fmt.Sprintf("Attempting to set %s/%s error status for error: %v", hr.GetNamespace(), hr.GetName(), err))

	hr.Status.SetCondition(appv1alpha1.HelmAppCondition{
		Type:    appv1alpha1.ConditionIrreconcilable,
		Status:  appv1alpha1.StatusTrue,
		Reason:  appv1alpha1.ReasonReconcileError,
		Message: err.Error(),
	})

	errUpdate := client.Status().Update(context.TODO(), hr)

	if errUpdate == nil {
		return nil
	}

	klog.Error(errUpdate, "- Failed to update HelmRelease status: ", hr)

	return errUpdate
}
