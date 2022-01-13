//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file was autogenerated by openapi-gen. Do not edit it manually!

package v1

import (
	spec "github.com/go-openapi/spec"
	common "k8s.io/kube-openapi/pkg/common"
)

func GetOpenAPIDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	return map[string]common.OpenAPIDefinition{
		"github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.HelmRelease":     schema_pkg_apis_apps_v1_HelmRelease(ref),
		"github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.HelmReleaseRepo": schema_pkg_apis_apps_v1_HelmReleaseRepo(ref),
	}
}

func schema_pkg_apis_apps_v1_HelmRelease(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "HelmRelease is the Schema for the subscriptionreleases API",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"kind": {
						SchemaProps: spec.SchemaProps{
							Description: "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"repo": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.HelmReleaseRepo"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.HelmAppSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.HelmAppStatus"),
						},
					},
				},
			},
		},
		Dependencies: []string{
			"github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.HelmAppSpec", "github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.HelmAppStatus", "github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.HelmReleaseRepo", "k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"},
	}
}

func schema_pkg_apis_apps_v1_HelmReleaseRepo(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "HelmReleaseRepo defines the repository of HelmRelease",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"source": {
						SchemaProps: spec.SchemaProps{
							Description: "INSERT ADDITIONAL SPEC FIELDS - desired state of cluster Important: Run \"operator-sdk generate k8s\" to regenerate code after modifying this file Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html Source holds the url toward the helm-chart",
							Ref:         ref("github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.Source"),
						},
					},
					"chartName": {
						SchemaProps: spec.SchemaProps{
							Description: "ChartName is the name of the chart within the repo",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"version": {
						SchemaProps: spec.SchemaProps{
							Description: "Version is the chart version",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"digest": {
						SchemaProps: spec.SchemaProps{
							Description: "Digest is the helm repo chart digest",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"secretRef": {
						SchemaProps: spec.SchemaProps{
							Description: "Secret to use to access the helm-repo defined in the CatalogSource.",
							Ref:         ref("k8s.io/api/core/v1.ObjectReference"),
						},
					},
					"configMapRef": {
						SchemaProps: spec.SchemaProps{
							Description: "Configuration parameters to access the helm-repo defined in the CatalogSource",
							Ref:         ref("k8s.io/api/core/v1.ObjectReference"),
						},
					},
					"insecureSkipVerify": {
						SchemaProps: spec.SchemaProps{
							Description: "InsecureSkipVerify is used to skip repo server's TLS certificate verification",
							Type:        []string{"boolean"},
							Format:      "",
						},
					},
				},
			},
		},
		Dependencies: []string{
			"github.com/stolostron/multicloud-operators-subscription-release/pkg/apis/apps/v1.Source", "k8s.io/api/core/v1.ObjectReference"},
	}
}
