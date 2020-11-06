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

// install/upgrade/uninstall code ported from:
// github.com/operator-framework/operator-sdk/internal/helm/controller/reconcile.go
// the uninstall code has been heavily modified to include cleanup of deployed release resources

//Package helmrelease controller manages the helmrelease CR
package helmrelease

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/ghodss/yaml"
	"github.com/prometheus/common/log"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	"github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/release"
)

const (
	finalizer = "uninstall-helm-release"

	defaultMaxConcurrent = 10
)

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
	chartsDir := os.Getenv(appv1.ChartsDir)
	if chartsDir == "" {
		chartsDir, err := ioutil.TempDir("/tmp", "charts")
		if err != nil {
			return err
		}

		err = os.Setenv(appv1.ChartsDir, chartsDir)
		if err != nil {
			return err
		}
	}

	klog.Info("The MaxConcurrentReconciles is set to: ", defaultMaxConcurrent)

	// Create a new controller
	c, err := controller.New("helmrelease-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: defaultMaxConcurrent})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource HelmRelease
	if err := c.Watch(&source.Kind{Type: &appv1.HelmRelease{}}, &handler.EnqueueRequestForObject{},
		predicate.GenerationChangedPredicate{}); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileHelmRelease implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileHelmRelease{}

// ReconcileHelmRelease reconciles a HelmRelease object
type ReconcileHelmRelease struct {
	manager.Manager
}

// Reconcile reads that state of the cluster for a HelmRelease object and makes changes based on the state read
// and what is in the HelmRelease.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileHelmRelease) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	klog.V(1).Info("Reconciling HelmRelease: ", request.Namespace, "/", request.Name)

	// Fetch the HelmRelease instance
	instance := &appv1.HelmRelease{}

	err := r.GetClient().Get(context.TODO(), request.NamespacedName, instance)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		klog.Error(err, "Failed to lookup resource ", request.Namespace, "/", request.Name)
		return reconcile.Result{}, err
	}

	// setting the nil spec to "":"" allows helmrelease to reconcile with default chart values.
	if instance.Spec == nil {
		spec := make(map[string]interface{})

		err := yaml.Unmarshal([]byte("{\"\":\"\"}"), &spec)
		if err != nil {
			return reconcile.Result{}, err
		}

		instance.Spec = spec

		err = r.GetClient().Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// handles the download of the chart as well
	helmOperatorManagerFactory, err := r.newHelmOperatorManagerFactory(instance)
	if err != nil {
		klog.Error(err, "- Failed to create new HelmOperatorManagerFactory: ", instance.GetNamespace(), "/", instance.GetName())

		instance.Status.SetCondition(appv1.HelmAppCondition{
			Type:    appv1.ConditionIrreconcilable,
			Status:  appv1.StatusTrue,
			Reason:  appv1.ReasonReconcileError,
			Message: err.Error(),
		})
		_ = r.updateResourceStatus(instance)

		klog.Info("Requeue HelmRelease after one minute ", instance.GetNamespace(), "/", instance.GetName())

		return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
	}

	manager, err := r.newHelmOperatorManager(instance, request, helmOperatorManagerFactory)
	if err != nil {
		klog.Error(err, "- Failed to create new HelmOperatorManager: ", instance.GetNamespace(), "/", instance.GetName())

		instance.Status.SetCondition(appv1.HelmAppCondition{
			Type:    appv1.ConditionIrreconcilable,
			Status:  appv1.StatusTrue,
			Reason:  appv1.ReasonReconcileError,
			Message: err.Error(),
		})
		_ = r.updateResourceStatus(instance)

		klog.Info("Requeue HelmRelease after one minute ", instance.GetNamespace(), "/", instance.GetName())

		return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
	}

	if err := manager.Sync(context.TODO()); err != nil {
		klog.Error(err, "Failed to sync HelmRelease ", instance.GetNamespace(), "/", instance.GetName())

		instance.Status.SetCondition(appv1.HelmAppCondition{
			Type:    appv1.ConditionIrreconcilable,
			Status:  appv1.StatusTrue,
			Reason:  appv1.ReasonReconcileError,
			Message: err.Error(),
		})
		_ = r.updateResourceStatus(instance)

		klog.Info("Requeue HelmRelease after one minute ", instance.GetNamespace(), "/", instance.GetName())

		return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
	}

	instance.Status.RemoveCondition(appv1.ConditionIrreconcilable)

	// helm uninstall
	if instance.GetDeletionTimestamp() != nil {
		if !contains(instance.GetFinalizers(), finalizer) {
			klog.Info("HelmRelease is terminated, skipping reconciliation ", instance.GetNamespace(), "/", instance.GetName())

			return reconcile.Result{}, nil
		}

		_, err := manager.UninstallRelease(context.TODO())
		if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
			klog.Error(err, "Failed to uninstall HelmRelease ", instance.GetNamespace(), "/", instance.GetName())
			instance.Status.SetCondition(appv1.HelmAppCondition{
				Type:    appv1.ConditionReleaseFailed,
				Status:  appv1.StatusTrue,
				Reason:  appv1.ReasonUninstallError,
				Message: err.Error(),
			})
			_ = r.updateResourceStatus(instance)
			return reconcile.Result{}, err
		}

		klog.Info("Uninstalled HelmRelease ", instance.GetNamespace(), ",", instance.GetName())

		// no need to check for remaining resources when there is no DeployedRelease
		// skip ahead to removing the finalizer and let the helmrelease terminate
		if instance.Status.DeployedRelease == nil || instance.Status.DeployedRelease.Manifest == "" {
			controllerutil.RemoveFinalizer(instance, finalizer)

			if err := r.updateResource(instance); err != nil {
				klog.Error(err, " - Failed to strip HelmRelease uninstall finalizer ", instance.GetNamespace(), "/", instance.GetName())

				return reconcile.Result{}, err
			}

			klog.Info("Removed finalizer from HelmRelease ", instance.GetNamespace(), ",", instance.GetName(), " requeue after 1 minute")

			return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
		}

		instance.Status.RemoveCondition(appv1.ConditionReleaseFailed)

		// find all the deployed resources and check to see if they still exists
		isFoundResource := false
		foundResource := &unstructured.Unstructured{}

		resources := releaseutil.SplitManifests(instance.Status.DeployedRelease.Manifest)
		for _, resource := range resources {
			var u unstructured.Unstructured
			if err := yaml.Unmarshal([]byte(resource), &u); err != nil {
				klog.Error(err, " - Failed to unmarshal resource ", resource)

				return reconcile.Result{}, err
			}

			gvk := u.GroupVersionKind()
			if gvk.Empty() {
				continue
			}

			o := &unstructured.Unstructured{}
			o.SetName(u.GetName())
			o.SetGroupVersionKind(u.GroupVersionKind())

			if u.GetNamespace() == "" {
				o.SetNamespace(instance.GetNamespace())
			}

			if r.isResourceDeleted(o, instance) {
				// resource is already delete, check the next one.
				continue
			}

			isFoundResource = true
			foundResource = o
		}

		if isFoundResource {
			message := "Failed to delete HelmRelease due to resource: " + foundResource.GroupVersionKind().String() + " " +
				foundResource.GetNamespace() + "/" + foundResource.GetName() + " is not deleted yet. Checking again after one minute."

			// at least one resource still exists, check again after one minute
			instance.Status.SetCondition(appv1.HelmAppCondition{
				Type:    appv1.ConditionReleaseFailed,
				Status:  appv1.StatusTrue,
				Reason:  appv1.ReasonUninstallError,
				Message: message,
			})
			_ = r.updateResourceStatus(instance)

			klog.Info("Requeue HelmRelease after one minute ", instance.GetNamespace(), "/", instance.GetName())

			return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
		}

		klog.Info("HelmRelease ", instance.GetNamespace(), "/", instance.GetName(),
			" all DeployedRelease resources are deleted/terminating")

		instance.Status.RemoveCondition(appv1.ConditionReleaseFailed)
		instance.Status.SetCondition(appv1.HelmAppCondition{
			Type:   appv1.ConditionDeployed,
			Status: appv1.StatusFalse,
			Reason: appv1.ReasonUninstallSuccessful,
		})
		_ = r.updateResourceStatus(instance)

		controllerutil.RemoveFinalizer(instance, finalizer)

		if err := r.updateResource(instance); err != nil {
			klog.Error(err, " - Failed to strip HelmRelease uninstall finalizer ", instance.GetNamespace(), "/", instance.GetName())

			return reconcile.Result{}, err
		}

		return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
	}

	instance.Status.SetCondition(appv1.HelmAppCondition{
		Type:   appv1.ConditionInitialized,
		Status: appv1.StatusTrue,
	})

	// helm install
	if !manager.IsInstalled() {
		installedRelease, err := manager.InstallRelease(context.TODO())
		if err != nil {
			klog.Error(err, "Failed to install HelmRelease ", instance.GetNamespace(), "/", instance.GetName())
			instance.Status.SetCondition(appv1.HelmAppCondition{
				Type:    appv1.ConditionReleaseFailed,
				Status:  appv1.StatusTrue,
				Reason:  appv1.ReasonInstallError,
				Message: err.Error(),
			})
			_ = r.updateResourceStatus(instance)
			return reconcile.Result{}, err
		}

		instance.Status.RemoveCondition(appv1.ConditionReleaseFailed)

		klog.V(1).Info("Adding finalizer (", finalizer, ") to ", instance.GetNamespace(), "/", instance.GetName())
		controllerutil.AddFinalizer(instance, finalizer)
		if err := r.updateResource(instance); err != nil {
			klog.Error("Failed to add uninstall finalizer to ", instance.GetNamespace(), "/", instance.GetName())
			return reconcile.Result{}, err
		}

		klog.Info("Installed HelmRelease ", instance.GetNamespace(), "/", instance.GetName())

		message := ""
		if installedRelease.Info != nil {
			message = installedRelease.Info.Notes
		}
		instance.Status.SetCondition(appv1.HelmAppCondition{
			Type:    appv1.ConditionDeployed,
			Status:  appv1.StatusTrue,
			Reason:  appv1.ReasonInstallSuccessful,
			Message: message,
		})
		instance.Status.DeployedRelease = &appv1.HelmAppRelease{
			Name:     installedRelease.Name,
			Manifest: installedRelease.Manifest,
		}
		err = r.updateResourceStatus(instance)
		return reconcile.Result{}, err
	}

	if !contains(instance.GetFinalizers(), finalizer) {
		klog.V(1).Info("Adding finalizer (", finalizer, ") to ", instance.GetNamespace(), "/", instance.GetName())
		controllerutil.AddFinalizer(instance, finalizer)
		if err := r.updateResource(instance); err != nil {
			klog.Error("Failed to add uninstall finalizer to ", instance.GetNamespace(), "/", instance.GetName())
			return reconcile.Result{}, err
		}
	}

	// helm upgrade
	if manager.IsUpgradeRequired() {
		force := hasHelmUpgradeForceAnnotation(instance)
		_, upgradedRelease, err := manager.UpgradeRelease(context.TODO(), release.ForceUpgrade(force))
		if err != nil {
			klog.Error(err, "Failed to upgrade HelmRelease ", instance.GetNamespace(), "/", instance.GetName())
			instance.Status.SetCondition(appv1.HelmAppCondition{
				Type:    appv1.ConditionReleaseFailed,
				Status:  appv1.StatusTrue,
				Reason:  appv1.ReasonUpgradeError,
				Message: err.Error(),
			})
			_ = r.updateResourceStatus(instance)
			return reconcile.Result{}, err
		}
		instance.Status.RemoveCondition(appv1.ConditionReleaseFailed)

		klog.Info("Upgraded HelmRelease ", "force=", force, " for ", instance.GetNamespace(), "/", instance.GetName())
		message := ""
		if upgradedRelease.Info != nil {
			message = upgradedRelease.Info.Notes
		}
		instance.Status.SetCondition(appv1.HelmAppCondition{
			Type:    appv1.ConditionDeployed,
			Status:  appv1.StatusTrue,
			Reason:  appv1.ReasonUpgradeSuccessful,
			Message: message,
		})
		instance.Status.DeployedRelease = &appv1.HelmAppRelease{
			Name:     upgradedRelease.Name,
			Manifest: upgradedRelease.Manifest,
		}
		err = r.updateResourceStatus(instance)

		return reconcile.Result{}, err
	}

	// If a change is made to the CR spec that causes a release failure, a
	// ConditionReleaseFailed is added to the status conditions. If that change
	// is then reverted to its previous state, the operator will stop
	// attempting the release and will resume reconciling. In this case, we
	// need to remove the ConditionReleaseFailed because the failing release is
	// no longer being attempted.
	instance.Status.RemoveCondition(appv1.ConditionReleaseFailed)

	// ensure the status is populated with install/upgrade reason
	expectedRelease, err := manager.GetDeployedRelease()
	if err != nil {
		log.Error(err, "Failed to get deployed release for HelmRelease ", instance.GetNamespace(), "/", instance.GetName())
		instance.Status.SetCondition(appv1.HelmAppCondition{
			Type:    appv1.ConditionIrreconcilable,
			Status:  appv1.StatusTrue,
			Reason:  appv1.ReasonReconcileError,
			Message: err.Error(),
		})
		_ = r.updateResourceStatus(instance)
		return reconcile.Result{}, err
	}
	instance.Status.RemoveCondition(appv1.ConditionIrreconcilable)

	reason := appv1.ReasonUpgradeSuccessful
	if expectedRelease.Version == 1 {
		reason = appv1.ReasonInstallSuccessful
	}
	message := ""
	if expectedRelease.Info != nil {
		message = expectedRelease.Info.Notes
	}
	instance.Status.SetCondition(appv1.HelmAppCondition{
		Type:    appv1.ConditionDeployed,
		Status:  appv1.StatusTrue,
		Reason:  reason,
		Message: message,
	})
	instance.Status.DeployedRelease = &appv1.HelmAppRelease{
		Name:     expectedRelease.Name,
		Manifest: expectedRelease.Manifest,
	}
	err = r.updateResourceStatus(instance)
	return reconcile.Result{}, err
}

func (r ReconcileHelmRelease) updateResourceStatus(hr *appv1.HelmRelease) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.GetClient().Status().Update(context.TODO(), hr)
	})
}

func (r ReconcileHelmRelease) updateResource(hr *appv1.HelmRelease) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.GetClient().Update(context.TODO(), hr)
	})
}

func contains(l []string, s string) bool {
	for _, elem := range l {
		if elem == s {
			return true
		}
	}
	return false
}

//isResourceDeleted finds the given resource, if it exists then delete it.
// return true if the resource is already deleted.
func (r *ReconcileHelmRelease) isResourceDeleted(resource *unstructured.Unstructured, hr *appv1.HelmRelease) bool {
	klog.V(2).Info("Getting resource: ", resource.GetNamespace(), "/", resource.GetName(),
		" ", resource.GroupVersionKind())

	nsn := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}

	// try to get the resource in the namespace
	err := r.GetClient().Get(context.TODO(), nsn, resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true // resource is already deleted
		}

		klog.V(2).Info("Ignorable error while attempting to fetch resource from namespace: ",
			resource.GetNamespace(), "/", resource.GetName(), " ", resource.GroupVersionKind(), " - ", err)

		// it's not in the namespace try looking for the resource in cluster scope
		resource.SetNamespace("")

		nsn = types.NamespacedName{Name: resource.GetName()}

		err := r.GetClient().Get(context.TODO(), nsn, resource)
		if err != nil {
			klog.V(2).Info("Ignorable error while attempting to fetch resource from cluster: ",
				resource.GetName(), " ", resource.GroupVersionKind(), " - ", err)

			return true // resource is already deleted
		}
	}

	// found the resource so it's not deleted yet

	klog.Info("Removal of HelmRelease ", hr.GetNamespace(), "/", hr.GetName(),
		" is blocked by resource: ", resource.GetNamespace(), "/", resource.GetName(),
		" ", resource.GroupVersionKind())

	if err = r.GetClient().Delete(context.TODO(), resource); err != nil {
		klog.Error(err, " - Failed to delete resource: ", resource.GetNamespace(), "/", resource.GetName(),
			" ", resource.GroupVersionKind())
	}

	return false
}

// returns the boolean representation of the annotation string
// will return false if annotation is not set
func hasHelmUpgradeForceAnnotation(hr *appv1.HelmRelease) bool {
	const helmUpgradeForceAnnotation = "helm.sdk.operatorframework.io/upgrade-force"
	force := hr.GetAnnotations()[helmUpgradeForceAnnotation]
	if force == "" {
		return false
	}
	value := false
	if i, err := strconv.ParseBool(force); err != nil {
		klog.Info("Could not parse annotation as a boolean ",
			"annotation=", helmUpgradeForceAnnotation, " value informed ", force,
			" for ", hr.GetNamespace(), "/", hr.GetName())
	} else {
		value = i
	}
	return value
}
