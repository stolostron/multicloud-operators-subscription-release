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
	"reflect"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/onsi/gomega"
	appv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	helmoperator "github.com/operator-framework/operator-sdk/pkg/helm/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestHelmOperator_uninstallRelease(t *testing.T) {
	testscheme := scheme.Scheme

	testscheme.AddKnownTypes(appv1.SchemeGroupVersion, &appv1.HelmRelease{})

	testHelmRelease := &appv1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "apps.open-cluster-management.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-helmrelease",
			Namespace: helmReleaseNS,
		},
	}

	testDeletingHelmReleaseNilFinalizers := &appv1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "apps.open-cluster-management.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-helmrelease",
			Namespace:         helmReleaseNS,
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
		},
	}
	testDeletingHelmRelease := &appv1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "apps.open-cluster-management.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-helmrelease",
			Namespace:         helmReleaseNS,
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
			Finalizers:        []string{finalizer},
		},
		Repo: appv1.HelmReleaseRepo{
			Source: &appv1.Source{
				SourceType: appv1.HelmRepoSourceType,
				HelmRepo: &appv1.HelmRepo{
					Urls: []string{
						"https://raw.github.com/open-cluster-management/multicloud-operators-subscription-release/master/test/helmrepo/subscription-release-test-3-0.1.0.tgz"},
				},
			},
			ChartName: "subscription-release-test-1",
		},
	}

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
		LeaderElection:     false,
	})
	if err != nil {
		t.Fatal("Failed to create manager", err)
	}

	rec := &ReconcileHelmRelease{
		mgr,
	}

	stopMgr, mgrStopped := StartTestManager(mgr, gomega.NewGomegaWithT(t))

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	err = mgr.GetClient().Create(context.TODO(), testDeletingHelmRelease)
	if err != nil {
		t.Fatal("Failed to create helmrelease", err)
	}

	time.Sleep(4 * time.Second)

	helmReleaseKey := types.NamespacedName{
		Name:      testDeletingHelmRelease.GetName(),
		Namespace: helmReleaseNS,
	}

	mgr.GetClient().Get(context.TODO(), helmReleaseKey, testDeletingHelmRelease)

	spec := make(map[string]interface{})
	yaml.Unmarshal([]byte("{\"\":\"\"}"), &spec)
	testDeletingHelmRelease.Spec = spec

	mgr.GetClient().Update(context.TODO(), testDeletingHelmRelease)

	time.Sleep(4 * time.Second)

	err = mgr.GetClient().Get(context.TODO(), helmReleaseKey, testDeletingHelmRelease)
	if err != nil {
		t.Fatal("Failed to get helmrelease", err)
	}

	factory, err := rec.newHelmOperatorManagerFactory(testDeletingHelmRelease)
	if err != nil {
		t.Fatal("Failed to create factory", err)
	}

	manager, err := rec.newHelmOperatorManager(testDeletingHelmRelease, reconcile.Request{NamespacedName: helmReleaseKey}, factory)
	if err != nil {
		t.Fatal("Failed to create manager", err)
	}

	type args struct {
		helmRelease *appv1.HelmRelease
		manager     helmoperator.Manager
	}

	tests := []struct {
		name    string
		args    args
		want    helmOperatorReconcileResult
		wantErr bool
	}{
		{
			name: "DeletionTimestamp is nil",
			args: args{
				helmRelease: testHelmRelease,
			},
			want:    helmOperatorReconcileResult{reconcile.Result{}, nil},
			wantErr: false,
		},
		{
			name: "finalizers is nil",
			args: args{
				helmRelease: testDeletingHelmReleaseNilFinalizers,
			},
			want:    helmOperatorReconcileResult{reconcile.Result{}, nil},
			wantErr: false,
		},
		{
			name: "uninstall",
			args: args{
				helmRelease: testDeletingHelmRelease,
				manager:     manager,
			},
			want:    helmOperatorReconcileResult{reconcile.Result{}, nil},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReconcileHelmRelease{}

			got := r.uninstallRelease(tt.args.helmRelease, tt.args.manager)

			if (got.Error != nil) != tt.wantErr {
				t.Errorf("ReconcileHelmRelease.uninstallRelease() error = %v, wantErr %v", got.Error, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileHelmRelease.uninstallRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHelmOperator_forceUpgradeRelease(t *testing.T) {
	testscheme := scheme.Scheme

	testscheme.AddKnownTypes(appv1.SchemeGroupVersion, &appv1.HelmRelease{})

	testHelmReleaseNoAnnotations := &appv1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "apps.open-cluster-management.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-helmrelease",
			Namespace: helmReleaseNS,
		},
	}

	testHelmReleaseForceFalse := &appv1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "apps.open-cluster-management.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-helmrelease",
			Namespace:   helmReleaseNS,
			Annotations: map[string]string{HelmReleaseUpgradeForceAnnotation: "false"},
		},
	}

	type args struct {
		helmRelease *appv1.HelmRelease
		manager     helmoperator.Manager
	}

	tests := []struct {
		name    string
		args    args
		want    helmOperatorReconcileResult
		wantErr bool
	}{
		{
			name: "no force annotation",
			args: args{
				helmRelease: testHelmReleaseNoAnnotations,
			},
			want:    helmOperatorReconcileResult{reconcile.Result{}, nil},
			wantErr: false,
		},
		{
			name: "force annotation set to false",
			args: args{
				helmRelease: testHelmReleaseForceFalse,
			},
			want:    helmOperatorReconcileResult{reconcile.Result{}, nil},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReconcileHelmRelease{}

			got := r.uninstallRelease(tt.args.helmRelease, tt.args.manager)

			if (got.Error != nil) != tt.wantErr {
				t.Errorf("ReconcileHelmRelease.forceUpgradeRelease() error = %v, wantErr %v", got.Error, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileHelmRelease.forceUpgradeRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}
