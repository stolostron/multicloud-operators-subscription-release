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

package utils

import (
	"context"
	stdlog "log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/stolostron/multicloud-operators-subscription-release/pkg/apis"
)

var cfg *rest.Config

func TestMain(m *testing.M) {
	t := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "deploy", "crds"),
		},
	}

	err := apis.AddToScheme(scheme.Scheme)
	if err != nil {
		stdlog.Fatal(err)
	}

	if cfg, err = t.Start(); err != nil {
		stdlog.Fatal(err)
	}

	code := m.Run()

	err = t.Stop()
	if err != nil {
		stdlog.Fatal(err)
	}

	os.Exit(code)
}

// StartTestManager adds recFn
func StartTestManager(ctx context.Context, mgr manager.Manager, g *gomega.GomegaWithT) *sync.WaitGroup {
	wg := &sync.WaitGroup{}

	wg.Add(1)

	go func() {
		wg.Done()
		mgr.Start(ctx)
	}()

	time.Sleep(2 * time.Second)

	return wg
}
