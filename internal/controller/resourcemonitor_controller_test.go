/*
Copyright 2025.

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

package controller

import (
	"context"
	"testing"
	"time"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"
)

func TestUpdateComposableResourceLastUsedTime(t *testing.T) {
	testCases := []struct {
		name                  string
		existingResourceList  *cdioperator.ComposableResourceList
		resourceSliceInfoList []types.ResourceSliceInfo
		labelPrefix           string
		wantErr               bool
		expectedErrMsg        string
		expectedUpdate        bool
	}{
		{
			name:        "empty resource",
			labelPrefix: "test",
		},
		{
			name:        "none Online resource",
			labelPrefix: "test",
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rs0",
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:  "gpu",
							Model: "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State: "Running",
						},
					},
				},
			},
		},
		{
			name:        "resource update failed",
			labelPrefix: "test",
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						Spec: cdioperator.ComposableResourceSpec{
							Type:  "gpu",
							Model: "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:    "Online",
							DeviceID: "123",
						},
					},
				},
			},
			resourceSliceInfoList: []types.ResourceSliceInfo{
				{
					Name: "rs0",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "123",
						},
					},
				},
			},
			expectedUpdate: false,
			wantErr:        true,
			expectedErrMsg: "failed to update ComposableResource:  \"\" is invalid: metadata.name: Required value: name is required",
		},
		{
			name:        "resource do not match ResourceSliceInfo",
			labelPrefix: "test",
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rs0",
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:  "gpu",
							Model: "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:    "Online",
							DeviceID: "123",
						},
					},
				},
			},
			resourceSliceInfoList: []types.ResourceSliceInfo{
				{
					Name: "rs0",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "456",
						},
					},
				},
			},
			expectedUpdate: false,
		},
		{
			name:        "normal case",
			labelPrefix: "test",
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rs0",
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:  "gpu",
							Model: "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:    "Online",
							DeviceID: "123",
						},
					},
				},
			},
			resourceSliceInfoList: []types.ResourceSliceInfo{
				{
					Name: "rs0",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "123",
						},
					},
				},
			},
			expectedUpdate: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			if tc.existingResourceList != nil {
				for i := range tc.existingResourceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceList.Items[i])
				}
			}

			s := scheme.Scheme
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			resourceController := &ResourceMonitorReconciler{
				Client: fakeClient,
			}

			err := resourceController.updateComposableResourceLastUsedTime(context.Background(), tc.resourceSliceInfoList, tc.labelPrefix)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				if err.Error() != tc.expectedErrMsg {
					t.Errorf("Error message is incorrect. Got: %q, Want: %q", err.Error(), tc.expectedErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resourceList := &cdioperator.ComposableResourceList{}
			err = fakeClient.List(context.Background(), resourceList)
			if err != nil {
				t.Errorf("failed to get resourceList: %v", err)
			}

			for _, rs := range resourceList.Items {
				if tc.expectedUpdate {
					assert.Contains(t, rs.Annotations, tc.labelPrefix+"/last-used-time")
					_, err := time.Parse(time.RFC3339, rs.Annotations[tc.labelPrefix+"/last-used-time"])
					assert.NoError(t, err)
				} else {
					assert.Nil(t, rs.Annotations)
				}
			}
		})
	}
}
