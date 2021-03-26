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

package e2e

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

const (
	defaultAddr      = "http://localhost:8765"
	runEndpoint      = "/run"
	stageRunEndpoint = "/run/stage"
	clusterEndpoint  = "/clusters"
	Success          = "succeed"
)

type TResponse struct {
	TestID  string      `json:"test_id"`
	Name    string      `json:"name"`
	Status  string      `json:"run_status"`
	Error   string      `json:"error"`
	Details interface{} `json:"details"`
}

func runner(runID string, runStage bool) error {
	URL := fmt.Sprintf("%s%s?id=%s", defaultAddr, runEndpoint, runID)
	if runStage {
		URL = fmt.Sprintf("%s%s?id=%s", defaultAddr, stageRunEndpoint, runID)
	}

	resp, err := http.Get(URL)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		res := &TResponse{}

		if err := json.Unmarshal(bodyBytes, res); err != nil {
			return err
		}

		if res.Status != Success {
			return fmt.Errorf("failed test on %s, with status %s err: %s", res.TestID, res.Status, res.Status)
		}

		return nil
	}

	return fmt.Errorf("incorrect response code %v", resp.StatusCode)
}

func isSeverUp() error {
	URL := fmt.Sprintf("%s%s", defaultAddr, clusterEndpoint)
	resp, err := http.Get(URL)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("e2e server is not up")
	}

	return nil
}

func TestE2ESuite(t *testing.T) {
	if err := isSeverUp(); err != nil {
		t.Fatal(err)
	}

	testIDs := []string{"RHACM4K-2346", "RHACM4K-1680", "RHACM4K-1701", "RHACM4K-2352", "RHACM4K-2347", "RHACM4K-2570", "RHACM4K-2569"}
	stageTestIDs := []string{"RHACM4K-2348", "RHACM4K-1732", "RHACM4K-2566", "RHACM4K-2568"}

	for _, tID := range testIDs {
		if err := runner(tID, false); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("HelmRelease e2e sub tests (1/2) %v passed", testIDs)

	for _, tID := range stageTestIDs {
		if err := runner(tID, true); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("HelmRelease e2e sub tests (2/2) %v passed", stageTestIDs)
}
