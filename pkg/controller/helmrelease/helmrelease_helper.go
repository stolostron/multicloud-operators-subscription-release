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

package helmrelease

import (
	"context"
	"fmt"
	"strings"

	appv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"

	"regexp"

	rspb "helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

// nameFilter filters a set of Helm storage releases by name.
func nameFilter(name string) releaseutil.FilterFunc {
	return releaseutil.FilterFunc(func(rls *rspb.Release) bool {
		if rls == nil {
			return true
		}
		return rls.Name == name
	})
}

// determines if this HelmRelease is owned by Subscription which is owned by MultiClusterHub
func (r *ReconcileHelmRelease) isMultiClusterHubOwnedResource(hr *appv1.HelmRelease) (bool, error) {
	klog.V(3).Info("Running isMultiClusterHubOwnedResource on ", hr.GetNamespace(), "/", hr.GetName())

	if hr.OwnerReferences == nil {
		return false, nil
	}

	for _, hrOwner := range hr.OwnerReferences {
		if hrOwner.Kind == "Subscription" {
			appsubGVK := schema.FromAPIVersionAndKind(hrOwner.APIVersion, hrOwner.Kind)
			appsubNsn := types.NamespacedName{Namespace: hr.GetNamespace(), Name: hrOwner.Name}

			appsub := &unstructured.Unstructured{}
			appsub.SetGroupVersionKind(appsubGVK)
			appsub.SetNamespace(appsubNsn.Namespace)
			appsub.SetName(appsubNsn.Name)

			err := r.GetClient().Get(context.TODO(), appsubNsn, appsub)
			if err != nil {
				if errors.IsNotFound(err) {
					klog.Info("Failed to find the parent (already deleted?), won't be able to determine if it's an ACM's HelmRelease: ",
						appsubNsn, " ", err)

					return false, nil
				}

				klog.Error("Failed to lookup HelmRelease's parent Subscription: ", appsubNsn, " ", err)

				return false, err
			}

			if appsub.GetOwnerReferences() != nil {
				for _, appsubOwner := range appsub.GetOwnerReferences() {
					if appsubOwner.Kind == "MultiClusterHub" &&
						strings.Contains(appsubOwner.APIVersion, "open-cluster-management") {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

// Remove CRD references from Helm storage and HelmRelease's Status.DeployedRelease.Manifest
// TODO add an annotation to trigger this feature instead of triggering on MultiClusterHub owned resource
func (r *ReconcileHelmRelease) hackMultiClusterHubRemoveCRDReferences(hr *appv1.HelmRelease) error {
	klog.V(3).Info("Running hackMultiClusterHubRemoveCRDReferences on ", hr.GetNamespace(), "/", hr.GetName())

	isOwnedByMCH, err := r.isMultiClusterHubOwnedResource(hr)
	if err != nil {
		klog.Error("Failed to determine if HelmRelease is owned a MultiClusterHub resource: ",
			hr.GetNamespace(), "/", hr.GetName())

		return err
	}

	if !isOwnedByMCH {
		klog.Info("HelmRelease is not owned by a MultiClusterHub resource: ",
			hr.GetNamespace(), "/", hr.GetName())

		return nil
	}

	klog.Info("HelmRelease is owned by a MultiClusterHub resource proceed with the removal of all CRD references: ",
		hr.GetNamespace(), "/", hr.GetName())

	clientv1, err := v1.NewForConfig(r.GetConfig())
	if err != nil {
		klog.Error("Failed create client for HelmRelease: ", hr.GetNamespace(), "/", hr.GetName())

		return err
	}

	storageBackend := storage.Init(driver.NewSecrets(clientv1.Secrets(hr.GetNamespace())))

	storageReleases, err := storageBackend.List(
		func(rls *rspb.Release) bool {
			return nameFilter(hr.GetName()).Check(rls)
		})
	if err != nil {
		klog.Error("Failed list all storage releases for HelmRelease: ", hr.GetNamespace(), "/", hr.GetName())

		return err
	}

	if storageReleases == nil {
		klog.Info("HelmRelease does not have any matching Helm storage releases: ",
			hr.GetNamespace(), "/", hr.GetName())
	} else {
		klog.Info("HelmRelease contains storage releases, attempting to strip CRDs from them: ",
			hr.GetNamespace(), "/", hr.GetName())
	}

	for _, storageRelease := range storageReleases {
		klog.Info("Release: ", storageRelease.Name)

		if storageRelease.Info != nil {
			klog.Info("Release: ", storageRelease.Name, " Status: ", storageRelease.Info.Status.String())
		}

		newManifest, changed := stripCRDs(storageRelease.Manifest)
		if changed {
			klog.Info("Release: ", storageRelease.Name, " needs updating")

			storageRelease.Manifest = newManifest

			err = storageBackend.Update(storageRelease)
			if err != nil {
				klog.Error("Failed update storage release for HelmRelease: ", hr.GetNamespace(), "/", hr.GetName())

				return err
			}

		} else {
			klog.Info("Release: ", storageRelease.Name, " is unchanged")
		}
	}

	if hr.Status.DeployedRelease == nil {
		klog.Info("HelmRelease does not have any Status.DeployedRelease: ",
			hr.GetNamespace(), "/", hr.GetName())

		return nil
	}

	klog.Info("HelmRelease contains Status.DeployedRelease, attempting to strip CRDs from it: ",
		hr.GetNamespace(), "/", hr.GetName())

	newManifest, changed := stripCRDs(hr.Status.DeployedRelease.Manifest)
	if changed {
		klog.Info("Status release: ", hr.GetName(), " needs updating")

		hr.Status.DeployedRelease.Manifest = newManifest

		err = r.updateResourceStatus(hr)
		if err != nil {
			klog.Error("Failed to update Status.DeployedRelease.Manifest for HelmRelease: ",
				hr.GetNamespace(), "/", hr.GetName())

			return err
		}

	} else {
		klog.Info("Status release: ", hr.GetName(), " is unchanged")
	}

	return nil
}

var sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")

func stripCRDs(bigFile string) (string, bool) {
	// Making sure that any extra whitespace in YAML stream doesn't interfere in splitting documents correctly.
	bigFileTmp := strings.TrimSpace(bigFile)
	docs := sep.Split(bigFileTmp, -1)
	// changed := false
	crdsRemoved := []string{}

	for _, yamlString := range docs {
		obj := &unstructured.Unstructured{}

		// decode YAML into unstructured.Unstructured
		dec := syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		_, _, err := dec.Decode([]byte(yamlString), nil, obj)
		if err != nil {
			klog.Warningf("Warning ignoring deserializing error: %s", yamlString)

			continue
		}

		if obj.GetKind() != "CustomResourceDefinition" {
			crdsRemoved = append(crdsRemoved, yamlString)
		} else {
			klog.Info("CRD detected: ", obj.GetName())
		}
	}

	if len(crdsRemoved) == len(docs) {
		return bigFile, false
	}

	newBigFile := fmt.Sprintf("---\n%s", strings.Join(crdsRemoved, "\n---\n"))
	return newBigFile, true
}
