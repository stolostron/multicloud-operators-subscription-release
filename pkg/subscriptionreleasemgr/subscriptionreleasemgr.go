package subscriptionreleasemgr

import (
	"errors"
	"net/http"
	"os"

	"github.com/ghodss/yaml"
	helmrelease "github.com/operator-framework/operator-sdk/pkg/helm/release"
	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const CHARTS_DIR = "CHARTS_DIR"

var log = logf.Log.WithName("subscriptionreleasemgr")

func NewManager(httpClient *http.Client, secret *corev1.Secret, s *appv1alpha1.SubscriptionRelease) (helmrelease.Manager, error) {
	srLogger := log.WithValues("SubscriptionRelease.Namespace", s.Namespace, "SubscrptionRelease.Name", s.Name)
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(s.GroupVersionKind())
	o.SetNamespace(s.GetNamespace())
	releaseName := s.Spec.ReleaseName[0:4]
	o.SetName(releaseName)
	o.SetUID(s.GetUID())

	mgr, err := manager.New(cfg, manager.Options{
		Namespace: s.GetNamespace(),
		//		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		srLogger.Error(err, "Failed to create a new manager.")
		return nil, err
	}

	chartsDir := os.Getenv(CHARTS_DIR)
	if chartsDir == "" {
		err = errors.New("Environment variable not set")
		srLogger.Error(err, "Failed to create a new manager.", "Variable", CHARTS_DIR)
		return nil, err
	}
	chartDir, err := utils.DownloadChart(httpClient, secret, chartsDir, s)
	srLogger.Info("ChartDir", "ChartDir", chartDir)
	if err != nil {
		srLogger.Error(err, "Failed to download the tgz")
		return nil, err
	}
	f := helmrelease.NewManagerFactory(mgr, chartDir)
	if s.Spec.Values != "" {
		var spec interface{}
		err = yaml.Unmarshal([]byte(s.Spec.Values), &spec)
		if err != nil {
			srLogger.Error(err, "Failed to Unmarshal the values", "values", s.Spec.Values)
			return nil, err
		}
		o.Object["spec"] = spec
	}
	helmManager, err := f.NewManager(o)
	return helmManager, err
}
