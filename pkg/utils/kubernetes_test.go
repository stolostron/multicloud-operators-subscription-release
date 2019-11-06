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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestLabelsChecker(t *testing.T) {
	req0 := metav1.LabelSelectorRequirement{
		Key:      "l1",
		Operator: metav1.LabelSelectorOpIn,
		Values:   []string{"v1"},
	}
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"l1": "v1",
			"l2": "v2",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			req0,
		},
	}
	ls := map[string]string{
		"l1": "v1",
		"l2": "v2",
	}

	b := LabelsChecker(labelSelector, ls)
	assert.Equal(t, true, b)

	labelSelector.MatchExpressions[0].Operator = "BadOperator"
	b = LabelsChecker(labelSelector, ls)
	assert.Equal(t, false, b)
}

func TestConvertLabels(t *testing.T) {
	req0 := metav1.LabelSelectorRequirement{
		Key:      "l1",
		Operator: metav1.LabelSelectorOpIn,
		Values:   []string{"v1"},
	}
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"l1": "v1",
			"l2": "v2",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			req0,
		},
	}
	s, err := ConvertLabels(labelSelector)
	assert.NoError(t, err)
	assert.Equal(t, "l1=v1,l1 in (v1),l2=v2", s.String())

	labelSelector.MatchExpressions[0].Operator = "BadOperator"
	s, err = ConvertLabels(labelSelector)
	assert.Error(t, err)
	assert.Equal(t, labels.Nothing(), s)

	labelSelector = nil
	s, err = ConvertLabels(labelSelector)
	assert.NoError(t, err)
	assert.Equal(t, labels.Everything(), s)
}

func TestGetAccessToken(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"password": []byte("password"),
		},
	}
	pw := GetAccessToken(secret)
	assert.Equal(t, "password", pw)

	secret = &corev1.Secret{
		Data: map[string][]byte{
			"accessToken": []byte("accessToken"),
		},
	}
	pw = GetAccessToken(secret)
	assert.Equal(t, "accessToken", pw)

	secret = &corev1.Secret{
		Data: map[string][]byte{
			"password":    []byte("password"),
			"accessToken": []byte("accessToken"),
		},
	}
	pw = GetAccessToken(secret)
	assert.Equal(t, "accessToken", pw)

	secret = &corev1.Secret{
		Data: map[string][]byte{},
	}
	pw = GetAccessToken(secret)
	assert.Equal(t, "", pw)
}
