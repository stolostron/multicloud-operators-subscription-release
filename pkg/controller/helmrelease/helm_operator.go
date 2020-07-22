/*
Copyright 2020 Red Hat

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

/*
This file consist of functionalities that are originally from the operator-sdk helm operator
https://github.com/operator-framework/operator-sdk/tree/master/pkg/helm

The goal is to always use the operator-sdk api unless it's absolutely necessary to make changes
to meet some of helmrelease's requirements.
*/

package helmrelease

import (
	"context"
	"errors"
	"time"

	"github.com/ghodss/yaml"
	"github.com/operator-framework/operator-sdk/pkg/helm/release"
	helmrelease "github.com/operator-framework/operator-sdk/pkg/helm/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
)

const (
	finalizer = "uninstall-helm-release"

	// OperatorSDKUpgradeForceAnnotation to perform the equalivent of `helm upgrade --force`
	OperatorSDKUpgradeForceAnnotation = "helm.operator-sdk/upgrade-force"
)

/*
 uninstallRelease uninstalls the helm release. It should only be call when there is a DeletionTimestamp.

 Origin: https://github.com/operator-framework/operator-sdk/blob/master/pkg/helm/controller/reconcile.go
 The uninstall path inside Reconcile()
 Modification: Significant. Added resources check to make sure all resources are deleted before removing the finalizer.
 Justification: The operator-sdk helm operator strips the finalizer even in cases where resources still exist.
*/
func (r *ReconcileHelmRelease) uninstallRelease(hr *appv1.HelmRelease,
	manager helmrelease.Manager) HelmOperatorReconcileResult {
	// sanity check
	if hr.GetDeletionTimestamp() == nil {
		klog.Error("uninstallRelease() should only be called when the DeletionTimestamp is populated ",
			hr.GetNamespace(), "/", hr.GetName())

		horResult := &HelmOperatorReconcileResult{reconcile.Result{}, nil}

		return *horResult
	}

	if !contains(hr.GetFinalizers(), finalizer) {
		klog.Info("HelmRelease is terminated, skipping reconciliation ", hr.GetNamespace(), "/", hr.GetName())

		horResult := &HelmOperatorReconcileResult{reconcile.Result{}, nil}

		return *horResult
	}

	hr.Status.RemoveCondition(appv1.ConditionTimedout)
	hr.Status.RemoveCondition(appv1.ConditionIrreconcilable)

	_, err := manager.UninstallRelease(context.TODO())
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		klog.Error(err, "Failed to uninstall HelmRelease ", hr.GetNamespace(), "/", hr.GetName())
		hr.Status.SetCondition(appv1.HelmAppCondition{
			Type:    appv1.ConditionReleaseFailed,
			Status:  appv1.StatusTrue,
			Reason:  appv1.ReasonUninstallError,
			Message: err.Error(),
		})

		_ = r.updateResourceStatus(hr)
		horResult := &HelmOperatorReconcileResult{reconcile.Result{}, err}

		return *horResult
	}

	klog.Info("Uninstalled HelmRelease ", hr.GetNamespace(), ",", hr.GetName())

	hr.Status.RemoveCondition(appv1.ConditionReleaseFailed)
	hr.Status.SetCondition(appv1.HelmAppCondition{
		Type:   appv1.ConditionDeployed,
		Status: appv1.StatusFalse,
		Reason: appv1.ReasonUninstallSuccessful +
			" but not all DeployedRelease resources are deleted",
	})

	if err := r.updateResourceStatus(hr); err != nil {
		klog.Info("Failed to update HelmRelease status ", hr.GetNamespace(), "/", hr.GetName())

		horResult := &HelmOperatorReconcileResult{reconcile.Result{}, err}

		return *horResult
	}

	// find all the deployed resources and check to see if they still exists
	foundResource := false
	resources := releaseutil.SplitManifests(hr.Status.DeployedRelease.Manifest)
	for _, resource := range resources {
		var u unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(resource), &u); err != nil {
			klog.Error(err, " - Failed to unmarshal resource ", resource)
			horResult := &HelmOperatorReconcileResult{reconcile.Result{}, err}

			return *horResult
		}

		gvk := u.GroupVersionKind()
		if gvk.Empty() {
			continue
		}

		o := &unstructured.Unstructured{}
		o.SetName(u.GetName())
		o.SetGroupVersionKind(u.GroupVersionKind())

		if u.GetNamespace() == "" {
			o.SetNamespace(hr.GetNamespace())
		}

		if r.isResourceDeleted(o, hr) {
			// resource is already delete, check the next one.
			continue
		}

		foundResource = true
	}

	if foundResource {
		// at least one resource still exists, check again after 10 seconds
		horResult := &HelmOperatorReconcileResult{reconcile.Result{RequeueAfter: time.Second * 10}, nil}

		return *horResult
	}

	klog.Info("HelmRelease ", hr.GetNamespace(), "/", hr.GetName(), " all DeployedRelease resources are deleted")
	controllerutil.RemoveFinalizer(hr, finalizer)

	if err := r.updateResource(hr); err != nil {
		klog.Error(err, " - Failed to strip HelmRelease uninstall finalizer ", hr.GetNamespace(), "/", hr.GetName())

		horResult := &HelmOperatorReconcileResult{reconcile.Result{}, err}

		return *horResult
	}

	horResult := &HelmOperatorReconcileResult{reconcile.Result{RequeueAfter: time.Minute * 2}, nil}

	return *horResult
}

//isResourceDeleted finds the given resource, if it exists then delete it. return true if the resource is already deleted.
func (r *ReconcileHelmRelease) isResourceDeleted(resource *unstructured.Unstructured, hr *appv1.HelmRelease) bool {
	// find the resource in the namespace
	found, err := r.isResourceExists(resource, hr)
	if err != nil {
		klog.Error(err, " - Failed to lookup resource ", resource)

		return false
	}

	if found {
		if err = r.GetClient().Delete(context.TODO(), resource); err != nil {
			klog.Error(err, " - Failed to delete resource: ", resource.GetNamespace(), "/", resource.GetName(),
				" GVK: ", resource.GroupVersionKind())
		}

		return false
	}

	// resource is not in the namespace. find the resource in the cluster.
	resource.SetNamespace("")
	found, _ = r.isResourceExists(resource, hr)
	if found {
		if err = r.GetClient().Delete(context.TODO(), resource); err != nil {
			klog.Error(err, " - Failed to delete resource: ", resource.GetName(),
				" GVK: ", resource.GroupVersionKind())
		}

		return false
	}

	// resource is not in the cluster either. return true to confirm that the resource is already delete.

	return true
}

func (r *ReconcileHelmRelease) isResourceExists(resource *unstructured.Unstructured,
	hr *appv1.HelmRelease) (bool, error) {
	klog.V(2).Info("Getting resource: ", resource.GetNamespace(), "/", resource.GetName(),
		" GVK: ", resource.GroupVersionKind())

	nsn := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
	if resource.GetNamespace() == "" {
		nsn = types.NamespacedName{Name: resource.GetName()}

		resource.SetName("")
	}

	err := r.GetClient().Get(context.TODO(), nsn, resource)
	if err == nil {
		klog.Info("Removal of HelmRelease ", hr.GetNamespace(), "/", hr.GetName(),
			" is blocked by resource: ", resource.GetNamespace(), "/", resource.GetName(),
			" GVK: ", resource.GroupVersionKind())
		return true, nil
	}

	if apierrors.IsNotFound(err) {
		return false, nil // resource is deleted
	}

	return false, err
}

/*
 forceUpgradeRelease forces the upgrade on the helm release.
 It should only be call when there is a HelmReleaseUpgradeForceAnnotation set to true.

 Origin: https://github.com/operator-framework/operator-sdk/blob/master/pkg/helm/controller/reconcile.go
 The upgrade path inside Reconcile()
 Modification: Minimal. Removed the IsUpgradeRequired() check.
 Justification: Operator-sdk always skips unnecessary upgrades. Sometimes, helmrelease might need to
 force the release upgrade. For example, to increment the helm revision number.
*/
func (r *ReconcileHelmRelease) forceUpgradeRelease(hr *appv1.HelmRelease,
	manager helmrelease.Manager) HelmOperatorReconcileResult {
	hrforce := hasBooleanAnnotation(hr, HelmReleaseUpgradeForceAnnotation)
	if !hrforce {
		klog.Error("forceUpgradeRelease() should only be called when the annotation is set to true ",
			HelmReleaseUpgradeForceAnnotation)

		horResult := &HelmOperatorReconcileResult{reconcile.Result{}, nil}

		return *horResult
	}

	hr.Status.RemoveCondition(appv1.ConditionTimedout)
	hr.Status.RemoveCondition(appv1.ConditionIrreconcilable)

	force := hasBooleanAnnotation(hr, OperatorSDKUpgradeForceAnnotation)
	_, upgradedRelease, err := manager.UpdateRelease(context.TODO(), release.ForceUpdate(force))

	if err != nil {
		klog.Error(err, "Release failed")
		hr.Status.SetCondition(appv1.HelmAppCondition{
			Type:    appv1.ConditionReleaseFailed,
			Status:  appv1.StatusTrue,
			Reason:  appv1.ReasonUpdateError,
			Message: err.Error(),
		})

		_ = r.updateResourceStatus(hr)

		horResult := &HelmOperatorReconcileResult{reconcile.Result{}, err}

		return *horResult
	}

	hr.Status.RemoveCondition(appv1.ConditionReleaseFailed)

	klog.Info("Upgraded release", "force", force)
	klog.V(1).Info("Config values", "values", upgradedRelease.Config)

	message := ""

	if upgradedRelease.Info != nil {
		message = upgradedRelease.Info.Notes
	}

	hr.Status.SetCondition(appv1.HelmAppCondition{
		Type:    appv1.ConditionDeployed,
		Status:  appv1.StatusTrue,
		Reason:  appv1.ReasonUpdateSuccessful,
		Message: message,
	})

	hr.Status.DeployedRelease = &appv1.HelmAppRelease{
		Name:     upgradedRelease.Name,
		Manifest: upgradedRelease.Manifest,
	}

	err = r.updateResourceStatus(hr)
	horResult := &HelmOperatorReconcileResult{reconcile.Result{}, err}

	return *horResult
}

func contains(l []string, s string) bool {
	for _, elem := range l {
		if elem == s {
			return true
		}
	}

	return false
}
