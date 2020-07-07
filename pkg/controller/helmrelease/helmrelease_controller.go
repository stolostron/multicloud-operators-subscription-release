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
	"strconv"
	"time"

	"github.com/ghodss/yaml"
	helmController "github.com/operator-framework/operator-sdk/pkg/helm/controller"
	"github.com/operator-framework/operator-sdk/pkg/helm/release"
	helmrelease "github.com/operator-framework/operator-sdk/pkg/helm/release"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
)

const (
	//DefaultMaxConcurrent is the default value for the MaxConcurrentReconciles
	DefaultMaxConcurrent = 10

	// MaxConcurrentEnvVar is the constant for env variable HR_MAX_CONCURRENT
	// which is the maximum concurrent reconcile number
	MaxConcurrentEnvVar = "HR_MAX_CONCURRENT"

	// OperatorSDKUpgradeForceAnnotation to perform the equalivent of `helm upgrade --force`
	OperatorSDKUpgradeForceAnnotation = "helm.operator-sdk/upgrade-force"

	// HelmReleaseUpgradeForceAnnotation to force a Helm chart upgrade even if the old and new manifests are the same
	HelmReleaseUpgradeForceAnnotation = "apps.open-cluster-management.io/hr-upgrade-force"
)

//ControllerCMDOptions possible command line options
type ControllerCMDOptions struct {
	MaxConcurrent int
}

//Options the command line options
var Options = ControllerCMDOptions{}

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

	maxConcurrentReconciles := Options.MaxConcurrent
	if maxConcurrentReconciles < 1 {
		maxConcurrentReconciles = DefaultMaxConcurrent
	}

	envMaxConcurrent := getEnvMaxConcurrent()
	if envMaxConcurrent != 0 && envMaxConcurrent > 0 {
		maxConcurrentReconciles = envMaxConcurrent
	}

	klog.Info("The MaxConcurrentReconciles is set to: ", maxConcurrentReconciles)

	// Create a new controller
	c, err := controller.New("helmrelease-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: maxConcurrentReconciles})
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

// getEnvMaxConcurrent returns the maximum number of concurrent reconciles
func getEnvMaxConcurrent() int {
	maxStr, found := os.LookupEnv(MaxConcurrentEnvVar)
	if !found {
		return 0
	}

	max, err := strconv.Atoi(maxStr)
	if err != nil {
		klog.Error(err)
		return 0
	}

	return max
}

// blank assignment to verify that ReconcileHelmRelease implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileHelmRelease{}

// ReconcileHelmRelease reconciles a HelmRelease object
type ReconcileHelmRelease struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	manager.Manager
}

// HelmOperatorReconcileResult holds the result of the HelmOperatorReconcile
type HelmOperatorReconcileResult struct {
	Result reconcile.Result
	Error  error
}

// Reconcile reads that state of the cluster for a HelmRelease object and makes changes based on the state read
// and what is in the HelmRelease.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileHelmRelease) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	klog.V(1).Info("Reconciling HelmRelease:", request)

	// Fetch the HelmRelease instance
	instance := &appv1.HelmRelease{}

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

	helmReleaseManagerFactory, err := r.newHelmReleaseManagerFactory(instance)
	if err != nil {
		klog.Error(err, "- Failed to create new HelmReleaseManagerFactory: ", instance)

		if errUpdate := setErrorStatus(r.GetClient(), instance, err, appv1.ConditionIrreconcilable); errUpdate != nil {
			return reconcile.Result{}, errUpdate
		}

		klog.V(1).Info("Requeue after two minutes.")

		return reconcile.Result{RequeueAfter: time.Minute * 2}, nil
	}

	helmReleaseManager, err := r.newHelmReleaseManager(instance, request, helmReleaseManagerFactory)
	if err != nil {
		klog.Error(err, "- Failed to create new HelmReleaseManager: ", instance)

		if errUpdate := setErrorStatus(r.GetClient(), instance, err, appv1.ConditionIrreconcilable); errUpdate != nil {
			return reconcile.Result{}, errUpdate
		}

		return reconcile.Result{}, nil
	}

	if err := helmReleaseManager.Sync(context.TODO()); err != nil {
		klog.Error(err, "- Failed to sync HelmRelease: ", instance)

		if errUpdate := setErrorStatus(r.GetClient(), instance, err, appv1.ConditionIrreconcilable); errUpdate != nil {
			return reconcile.Result{}, errUpdate
		}

		return reconcile.Result{}, nil
	}

	deleted := instance.GetDeletionTimestamp() != nil

	if !deleted &&
		!containsErrorConditions(instance) &&
		!helmReleaseManager.IsUpdateRequired() &&
		helmReleaseManager.IsInstalled() &&
		instance.Status.DeployedRelease != nil {
		if hasBooleanAnnotation(instance, HelmReleaseUpgradeForceAnnotation) {
			return processUpgradeRelease(r.GetClient(), instance, helmReleaseManager)
		}

		klog.Info("Update is not required. Skipping Reconciling HelmRelease:", request)

		return reconcile.Result{Requeue: false}, nil
	}

	return processHorReconcile(r.GetClient(), request, instance, helmReleaseManagerFactory)
}

// processHorReconcile ensures the horReconcile() returns or it will time it out.
func processHorReconcile(
	client client.Client, request reconcile.Request, instance *appv1.HelmRelease, factory helmrelease.ManagerFactory) (
	reconcile.Result, error) {
	klog.V(2).Info("Processing reconciliation: ", request)

	hor := &helmController.HelmOperatorReconciler{
		Client:         client,
		GVK:            instance.GroupVersionKind(),
		ManagerFactory: factory,
	}

	c := make(chan HelmOperatorReconcileResult)

	go func() {
		res := horReconcile(request, *hor)
		c <- res
	}()

	// process the horReconcile() return or timeout.
	return processHelmOperatorReconcileResult(client, instance, c)
}

// horReconcile calls the Helm Operator reconcile and returns the result
func horReconcile(request reconcile.Request, hor helmController.HelmOperatorReconciler) HelmOperatorReconcileResult {
	result, err := hor.Reconcile(request)
	horResult := &HelmOperatorReconcileResult{result, err}

	return *horResult
}

func containsErrorConditions(hr *appv1.HelmRelease) bool {
	if hr.Status.Conditions == nil {
		return false
	}

	for i := range hr.Status.Conditions {
		if hr.Status.Conditions[i].Type == appv1.ConditionIrreconcilable ||
			hr.Status.Conditions[i].Type == appv1.ConditionReleaseFailed {
			return true
		}
	}

	return false
}

func setErrorStatus(client client.StatusClient, hr *appv1.HelmRelease, err error, conditionType appv1.HelmAppConditionType) error {
	klog.V(1).Info(fmt.Sprintf("Attempting to set %s/%s error status for error: %v", hr.GetNamespace(), hr.GetName(), err))

	hr.Status.SetCondition(appv1.HelmAppCondition{
		Type:    conditionType,
		Status:  appv1.StatusTrue,
		Reason:  appv1.ReasonReconcileError,
		Message: err.Error(),
	})

	return updateResourceStatus(client, hr)
}

func updateResourceStatus(client client.StatusClient, hr *appv1.HelmRelease) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return client.Status().Update(context.TODO(), hr)
	})
}

func hasBooleanAnnotation(hr *appv1.HelmRelease, annotation string) bool {
	force := hr.GetAnnotations()[annotation]
	if force == "" {
		return false
	}

	value := false

	if i, err := strconv.ParseBool(force); err != nil {
		klog.Info("Could not parse annotation as a boolean",
			"annotation", annotation, "value informed", force)
	} else {
		value = i
	}

	return value
}

// processUpgradeRelease ensures the upgradeRelease() returns or it will time it out.
func processUpgradeRelease(client client.StatusClient, hr *appv1.HelmRelease,
	manager helmrelease.Manager) (reconcile.Result, error) {
	klog.V(2).Info("Processing upgrade: ", hr)

	c := make(chan HelmOperatorReconcileResult)

	go func() {
		res := upgradeRelease(client, hr, manager)
		c <- res
	}()

	// process the upgradeRelease() return or timeout.
	return processHelmOperatorReconcileResult(client, hr, c)
}

// upgradeRelease upgrades the helm release. It should only be call when there is a HelmReleaseUpgradeForceAnnotation
func upgradeRelease(client client.StatusClient, hr *appv1.HelmRelease,
	manager helmrelease.Manager) HelmOperatorReconcileResult {
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

		_ = updateResourceStatus(client, hr)

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

	err = updateResourceStatus(client, hr)
	horResult := &HelmOperatorReconcileResult{reconcile.Result{}, err}

	return *horResult
}

// processHelmOperatorReconcileResult determines if timeout is necessary
func processHelmOperatorReconcileResult(client client.StatusClient, hr *appv1.HelmRelease,
	c chan HelmOperatorReconcileResult) (reconcile.Result, error) {
	select {
	case res := <-c:
		return res.Result, res.Error
	case <-time.After(30 * time.Minute):
		err := fmt.Errorf("timeout after 30 minutes while processing %v, check if there are any hung jobs or pods that are blocking the processing", hr)
		klog.Error(err)

		errUpdate := setErrorStatus(client, hr, err, appv1.ConditionTimedout)
		if errUpdate != nil {
			return reconcile.Result{}, errUpdate
		}

		return reconcile.Result{}, err
	}
}
