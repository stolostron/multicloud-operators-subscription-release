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

//Package helmrelease controller manages the helmrelease CR
package helmrelease

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/ghodss/yaml"
	helmOperatorController "github.com/operator-framework/operator-sdk/pkg/helm/controller"
	helmoperator "github.com/operator-framework/operator-sdk/pkg/helm/release"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
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
		klog.Error(err, "- Failed to create new HelmOperatorManagerFactory: ", instance)

		if errUpdate := r.setErrorStatus(instance, err, appv1.ConditionIrreconcilable); errUpdate != nil {
			return reconcile.Result{}, errUpdate
		}

		klog.V(1).Info("Requeue after two minutes.")

		return reconcile.Result{RequeueAfter: time.Minute * 2}, nil
	}

	helmOperatorManager, err := r.newHelmOperatorManager(instance, request, helmOperatorManagerFactory)
	if err != nil {
		klog.Error(err, "- Failed to create new HelmOperatorManager: ", instance)

		if errUpdate := r.setErrorStatus(instance, err, appv1.ConditionIrreconcilable); errUpdate != nil {
			return reconcile.Result{}, errUpdate
		}

		return reconcile.Result{}, nil
	}

	if err := helmOperatorManager.Sync(context.TODO()); err != nil {
		klog.Error(err, "- Failed to sync HelmRelease: ", instance)

		if errUpdate := r.setErrorStatus(instance, err, appv1.ConditionIrreconcilable); errUpdate != nil {
			return reconcile.Result{}, errUpdate
		}

		return reconcile.Result{}, nil
	}

	// delete path, uninstall the helmrelease
	if instance.GetDeletionTimestamp() != nil {
		return r.processUninstallRelease(instance, helmOperatorManager)
	}

	// check to see if it's necessary to upgrade
	if !containsErrorConditions(instance) &&
		!helmOperatorManager.IsUpdateRequired() &&
		helmOperatorManager.IsInstalled() &&
		instance.Status.DeployedRelease != nil {
		// force the upgrade if HelmReleaseUpgradeForceAnnotation is set to true
		if hasBooleanAnnotation(instance, HelmReleaseUpgradeForceAnnotation) {
			return r.processForceUpgradeRelease(instance, helmOperatorManager)
		}

		klog.Info("Upgrade is not required. Skipping Reconciling HelmRelease:", request)

		return reconcile.Result{Requeue: false}, nil
	}

	return r.processHelmOperatorReconcile(request, instance, helmOperatorManagerFactory)
}

// processHelmOperatorReconcile ensures the helmOperatorReconcile() returns or it will timeout.
func (r *ReconcileHelmRelease) processHelmOperatorReconcile(request reconcile.Request, instance *appv1.HelmRelease, factory helmoperator.ManagerFactory) (
	reconcile.Result, error) {
	klog.V(2).Info("Processing reconciliation: ", request)

	instance.Status.RemoveCondition(appv1.ConditionTimedout)

	hor := &helmOperatorController.HelmOperatorReconciler{
		Client:         r.GetClient(),
		GVK:            instance.GroupVersionKind(),
		ManagerFactory: factory,
	}

	c := make(chan HelmOperatorReconcileResult)

	go func() {
		res := helmOperatorReconcile(request, *hor)
		c <- res
	}()

	// process the helmOperatorReconcile() return or timeout.
	return r.processHelmOperatorReconcileResult(instance, c)
}

// helmOperatorReconcile calls the Helm Operator reconcile and returns the result
func helmOperatorReconcile(request reconcile.Request, hor helmOperatorController.HelmOperatorReconciler) HelmOperatorReconcileResult {
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

func (r *ReconcileHelmRelease) setErrorStatus(hr *appv1.HelmRelease, err error, conditionType appv1.HelmAppConditionType) error {
	klog.V(1).Info(fmt.Sprintf("Attempting to set %s/%s error status for error: %v", hr.GetNamespace(), hr.GetName(), err))

	hr.Status.SetCondition(appv1.HelmAppCondition{
		Type:    conditionType,
		Status:  appv1.StatusTrue,
		Reason:  appv1.ReasonReconcileError,
		Message: err.Error(),
	})

	return r.updateResourceStatus(hr)
}

func (r *ReconcileHelmRelease) updateResourceStatus(hr *appv1.HelmRelease) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.GetClient().Status().Update(context.TODO(), hr)
	})
}

func (r *ReconcileHelmRelease) updateResource(hr *appv1.HelmRelease) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.GetClient().Update(context.TODO(), hr)
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

// processUninstallRelease ensures the uninstallRelease() returns or it will timeout.
func (r *ReconcileHelmRelease) processUninstallRelease(hr *appv1.HelmRelease,
	manager helmoperator.Manager) (reconcile.Result, error) {
	klog.V(2).Info("Processing uninstall release: ", hr.GetNamespace(), "/", hr.GetName())

	c := make(chan HelmOperatorReconcileResult)

	go func() {
		res := r.uninstallRelease(hr, manager)
		c <- res
	}()

	// process the uninstallRelease() return or timeout.
	return r.processHelmOperatorReconcileResult(hr, c)
}

// processForceUpgradeRelease ensures the forceUpgradeRelease() returns or it will timeout.
func (r *ReconcileHelmRelease) processForceUpgradeRelease(hr *appv1.HelmRelease,
	manager helmoperator.Manager) (reconcile.Result, error) {
	klog.V(2).Info("Processing force upgrade: ", hr.GetNamespace(), "/", hr.GetName())

	c := make(chan HelmOperatorReconcileResult)

	go func() {
		res := r.forceUpgradeRelease(hr, manager)
		c <- res
	}()

	// process the forceUpgradeRelease() return or timeout.
	return r.processHelmOperatorReconcileResult(hr, c)
}

// processHelmOperatorReconcileResult determines if timeout is necessary
func (r *ReconcileHelmRelease) processHelmOperatorReconcileResult(hr *appv1.HelmRelease,
	c chan HelmOperatorReconcileResult) (reconcile.Result, error) {
	select {
	case res := <-c:
		return res.Result, res.Error
	case <-time.After(30 * time.Minute):
		err := fmt.Errorf("timeout after 30 minutes while processing %v, check if there are any hung jobs or pods that are blocking the processing", hr)
		klog.Error(err)

		errUpdate := r.setErrorStatus(hr, err, appv1.ConditionTimedout)
		if errUpdate != nil {
			return reconcile.Result{}, errUpdate
		}

		return reconcile.Result{}, err
	}
}
