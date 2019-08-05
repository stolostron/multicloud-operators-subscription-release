package helmreleasemanager

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	helmrelease "github.com/operator-framework/operator-sdk/pkg/helm/release"
	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/helm/pkg/repo"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const chartDir = "/Users/dvernier/Downloads/ibm-razee-api"

func NewHelmManager(s appv1alpha1.Subscription, chartVersion *repo.ChartVersion) (helmrelease.Manager, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	var channelName string
	if s.Spec.Channel != "" {
		strs := strings.Split(s.Spec.Channel, "/")
		if len(strs) != 2 {
			errmsg := "Illegal channel settings, want namespace/name, but get " + s.Spec.Channel
			err := errors.New(errmsg)
			glog.Error(err, "")
			return nil, err
		}
		channelName = strs[1]
	}

	releaseName := chartVersion.GetName()
	if channelName != "" {
		releaseName = releaseName + "-" + channelName
		fmt.Printf("%s-$s", releaseName+"-"+channelName)
	}

	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(s.GroupVersionKind())
	o.SetNamespace(s.GetNamespace())
	o.SetName(s.GetName())
	o.SetUID(s.GetUID())
	// m := getOverrides(s, "ibm-razee-api")
	// rn := o.GetName()
	// glog.Info(rn)
	// uuid, err := uuid.NewV4()
	// if err != nil {
	// 	glog.Error(err, "Failed to generate a UUID.")
	// 	return nil, err
	// }
	// o.SetUID(types.UID(uuid.String()))
	mgr, err := manager.New(cfg, manager.Options{
		Namespace: s.GetNamespace(),
		//		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		glog.Error(err, "Failed to create a new manager.")
		return nil, err
	}

	f := helmrelease.NewManagerFactory(mgr, chartDir)
	helmManager, err := f.NewManager(o)
	return helmManager, err
}

func loadIndex(data []byte) (*repo.IndexFile, error) {
	i := &repo.IndexFile{}
	if err := yaml.Unmarshal(data, i); err != nil {
		return i, err
	}
	i.SortEntries()
	if i.APIVersion == "" {
		return i, repo.ErrNoAPIVersion
	}
	return i, nil
}

func getOverrides(s appv1alpha1.Subscription, packageName string) (m map[string]interface{}) {
	dploverrides := make([]appv1alpha1.Overrides, 1)
	for _, overrides := range s.Spec.PackageOverrides {
		if overrides.PackageName == packageName {
			glog.Infof("Overrides for package %s found", packageName)
			dploverrides[0].PackageName = packageName
			dploverrides[0].PackageOverrides = make([]appv1alpha1.PackageOverride, 0)
			for _, override := range overrides.PackageOverrides {
				packageOverride := appv1alpha1.PackageOverride{
					RawExtension: runtime.RawExtension{
						Raw: override.RawExtension.Raw,
					},
				}
				dploverrides[0].PackageOverrides = append(dploverrides[0].PackageOverrides, packageOverride)
			}
			// data, err := yaml.Marshal(dploverrides[0].PackageOverrides[0].Raw)
			// if err != nil {
			// 	glog.Info(err)
			// 	return nil
			// }
			var o map[string]interface{}
			err := yaml.Unmarshal(dploverrides[0].PackageOverrides[0].Raw, &o)
			if err != nil {
				fmt.Print(err)
				return nil
			}
			fmt.Print(o["value"])
			err = yaml.Unmarshal([]byte(o["value"].(string)), &m)
			if err != nil {
				fmt.Print(err)
				return nil
			}
			return m
		}
	}
	return nil
}
