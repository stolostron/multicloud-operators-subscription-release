package subscriptionreleasemgr

import (
	"github.com/golang/glog"
	helmrelease "github.com/operator-framework/operator-sdk/pkg/helm/release"
	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const chartDir = "/Users/dvernier/Downloads/ibm-razee-api"

func NewHelmManager(s appv1alpha1.SubscriptionRelease) (helmrelease.Manager, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(s.GroupVersionKind())
	o.SetNamespace(s.GetNamespace())
	o.SetName(s.GetName())
	o.SetUID(s.GetUID())

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
