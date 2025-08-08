/*
Copyright 2025 The CoHDI Authors.

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
	"testing"
	"time"

	"github.com/CoHDI/dynamic-device-scaler/internal/types"
	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSortByTime(t *testing.T) {
	now := time.Now().UTC()
	hourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)
	tests := []struct {
		name  string
		input []types.ResourceClaimInfo
		want  []types.ResourceClaimInfo
		order string
	}{
		{
			name:  "normal descending order",
			order: "Descending",
			input: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(twoHoursAgo)},
				{CreationTimestamp: metav1.NewTime(now)},
				{CreationTimestamp: metav1.NewTime(hourAgo)},
			},
			want: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(now)},
				{CreationTimestamp: metav1.NewTime(hourAgo)},
				{CreationTimestamp: metav1.NewTime(twoHoursAgo)},
			},
		},
		{
			name:  "normal ascending order",
			order: "Ascending",
			input: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(twoHoursAgo)},
				{CreationTimestamp: metav1.NewTime(now)},
				{CreationTimestamp: metav1.NewTime(hourAgo)},
			},
			want: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(twoHoursAgo)},
				{CreationTimestamp: metav1.NewTime(hourAgo)},
				{CreationTimestamp: metav1.NewTime(now)},
			},
		},
		{
			name:  "empty slice",
			order: "Descending",
			input: []types.ResourceClaimInfo{},
			want:  []types.ResourceClaimInfo{},
		},
		{
			name:  "single element",
			order: "Descending",
			input: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(now)},
			},
			want: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(now)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]types.ResourceClaimInfo, len(tt.input))
			copy(got, tt.input)

			sortByTime(got, tt.order)

			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.want))
			}
			for i := 0; i < len(got); i++ {
				if !got[i].CreationTimestamp.Time.Equal(tt.want[i].CreationTimestamp.Time) {
					t.Errorf("index %d time mismatch:\ngot:  %v\nwant: %v",
						i, got[i].CreationTimestamp.Time, tt.want[i].CreationTimestamp.Time)
				}

				if got[i].Name != tt.want[i].Name {
					t.Errorf("index %d name mismatch:\ngot:  %q\nwant: %q",
						i, got[i].Name, tt.want[i].Name)
				}
			}
		})
	}
}

func TestNotIn(t *testing.T) {
	tests := []struct {
		name     string
		target   int
		slice    []int
		expected bool
	}{
		{"target not in slice", 5, []int{1, 2, 3, 4}, true},
		{"target in slice", 3, []int{1, 2, 3, 4}, false},
		{"empty slice", 1, []int{}, true},
		{"single element slice, target not in", 2, []int{1}, true},
		{"single element slice, target in", 1, []int{1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := notIn(tt.target, tt.slice)
			if result != tt.expected {
				t.Errorf("notIn(%v, %v) = %v; expected %v", tt.target, tt.slice, result, tt.expected)
			}
		})
	}
}

func TestIsDeviceCoexistence(t *testing.T) {
	tests := []struct {
		name                string
		model1              string
		model2              string
		composableDRASpec   types.ComposableDRASpec
		expectedCoexistence bool
	}{
		{
			name:   "Models can coexist",
			model1: "ModelA",
			model2: "ModelB",
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "ModelA",
						CannotCoexistWith: []int{},
					},
					{
						Index:             2,
						CDIModelName:      "ModelB",
						CannotCoexistWith: []int{},
					},
				},
			},
			expectedCoexistence: true,
		},
		{
			name:   "Models cannot coexist",
			model1: "ModelA",
			model2: "ModelB",
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "ModelA",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "ModelB",
						CannotCoexistWith: []int{1},
					},
				},
			},
			expectedCoexistence: false,
		},
		{
			name:   "Model not found",
			model1: "ModelX",
			model2: "ModelY",
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "ModelA",
						CannotCoexistWith: []int{},
					},
					{
						Index:             2,
						CDIModelName:      "ModelB",
						CannotCoexistWith: []int{},
					},
				},
			},
			expectedCoexistence: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDeviceCoexistence(tt.model1, tt.model2, tt.composableDRASpec)
			if result != tt.expectedCoexistence {
				t.Errorf("expected %v, got %v", tt.expectedCoexistence, result)
			}
		})
	}
}

func TestIsLastUsedOverThreshold(t *testing.T) {
	tests := []struct {
		name                       string
		annotations                map[string]string
		creationTimes              metav1.Time
		threshold                  time.Duration
		useCreateTimeWhenNotExists bool
		expectedResult             bool
		expectedErr                bool
		expectedErrMsg             string
	}{
		{
			name:                       "No annotations",
			annotations:                nil,
			threshold:                  time.Minute,
			useCreateTimeWhenNotExists: false,
			creationTimes:              metav1.Now(),
			expectedResult:             true,
		},
		{
			name:                       "Annotation not found",
			annotations:                map[string]string{},
			creationTimes:              metav1.Now(),
			threshold:                  time.Minute,
			useCreateTimeWhenNotExists: true,
			expectedResult:             false,
		},
		{
			name:                       "use creation time when not exists and creation time is zero",
			annotations:                map[string]string{},
			threshold:                  time.Minute,
			useCreateTimeWhenNotExists: true,
			creationTimes:              metav1.Time{},
			expectedErr:                true,
			expectedErrMsg:             "creation timestamp is missing",
		},
		{
			name: "Invalid time format",
			annotations: map[string]string{
				"composable.test/last-used-time": "invalid-time-format",
			},
			threshold:      time.Minute,
			creationTimes:  metav1.Now(),
			expectedResult: false,
			expectedErr:    true,
			expectedErrMsg: "failed to parse time:",
		},
		{
			name: "Time less than threshold",
			annotations: map[string]string{
				"composable.test/last-used-time": time.Now().Add(-30 * time.Second).Format(time.RFC3339),
			},
			threshold:      time.Minute,
			creationTimes:  metav1.Now(),
			expectedResult: false,
			expectedErr:    false,
		},
		{
			name: "Time more than threshold",
			annotations: map[string]string{
				"composable.test/last-used-time": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
			},
			threshold:      time.Minute,
			creationTimes:  metav1.Now(),
			expectedResult: true,
			expectedErr:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resource := cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations:       tc.annotations,
					CreationTimestamp: tc.creationTimes,
				},
				Spec: cdioperator.ComposableResourceSpec{
					Type:       "gpu",
					Model:      "A100",
					TargetNode: "node1",
				},
			}

			result, err := isLastUsedOverThreshold(resource, "composable.test", tc.threshold, tc.useCreateTimeWhenNotExists)

			if tc.expectedErr {
				if err == nil {
					t.Errorf("Expected error, got none")
				}
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if result != tc.expectedResult {
				t.Errorf("Expected result: %v, got %v", tc.expectedResult, result)
			}
		})
	}
}

func TestRescheduleFailedNotification(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name                       string
		existingResourceClaimList  *resourceapi.ResourceClaimList
		existingResourceSliceList  *resourceapi.ResourceSliceList
		existingRequestList        *cdioperator.ComposabilityRequestList
		existingComposableResource *cdioperator.ComposableResourceList
		nodeInfo                   types.NodeInfo
		composableDRASpec          types.ComposableDRASpec
		resourceClaims             []types.ResourceClaimInfo
		resourceSlices             []types.ResourceSliceInfo
		expectedResourceClaims     []types.ResourceClaimInfo
		wantErr                    bool
		expectedErrMsg             string
	}{
		{
			name: "failed to list composabilityRequest",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "claim1",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 80G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "gpu0",
								},
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to list composabilityRequestList:",
		},
		{
			name: "failed to get resourceSlice",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "claim1",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to get resourceSlice:",
		},
		{
			name: "setDevicesState failed",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "claim1",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 80G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "gpu0",
								},
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to set devices state:",
		},
		{
			name: "Device not coexistence",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 80G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "gpu0",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 5,
					},
				},
			},
			wantErr: false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 80G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
		},
		{
			name: "Device not coexistence and setDevicesState failed",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim1",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 80G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "gpu0",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 5,
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to set devices state:",
		},
		{
			name: "ComposabilityRequestList exceed the maximum",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:        "A100 40G",
						MaxDevice:    1,
						MaxDeviceSet: true,
					},
				},
			},
			wantErr: false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
		},
		{
			name: "ComposabilityRequestList exceed the maximum and setDevicesState failed",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim1",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:        "A100 40G",
						MaxDevice:    1,
						MaxDeviceSet: true,
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to set devices state:",
		},
		{
			name: "ResourceClaim devices do not coexist with ComposableResource",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 80G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State: "Online",
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 1,
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{},
			wantErr:             false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
		},
		{
			name: "ResourceClaim devices do not coexist with ComposableResource and setDevicesState failed",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 80G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State: "Online",
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim1",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 1,
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{},
			wantErr:             true,
			expectedErrMsg:      "failed to set devices state:",
		},
		{
			name: "ComposabilityRequestList have different model",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:        "A100 40G",
						MaxDevice:    1,
						MaxDeviceSet: true,
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			wantErr: false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
		},
		{
			name: "ResourceClaim devices do not coexist with ComposabilityRequest",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 4,
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model:      "A100 80G",
								Size:       10,
								TargetNode: "node1",
							},
						},
					},
				},
			},
			wantErr: false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
		},
		{
			name: "ResourceClaim devices do not coexist with ComposabilityRequest and setDevicesState failed",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim1",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 4,
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model:      "A100 80G",
								Size:       10,
								TargetNode: "node1",
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to set devices state:",
		},
		{
			name: "ResourceClaimInfo devices do not coexist",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
				{
					Name:              "test-claim2",
					Namespace:         "test-ns",
					NodeName:          "node2",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 80G",
							State: "Preparing",
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 1,
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to set devices state:",
		},
		{
			name: "ResourceClaimInfo devices do not coexist",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
				{
					Name:              "test-claim1",
					Namespace:         "test-ns",
					NodeName:          "node2",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 80G",
							State: "Preparing",
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim1",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 1,
					},
				},
			},
			wantErr: false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
					},
				},
				{
					Name:              "test-claim1",
					Namespace:         "test-ns",
					NodeName:          "node2",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 80G",
							State: "Failed",
						},
					},
				},
			},
		},
		{
			name: "ResourceClaimInfo device with Failed state and setDevicesState failed",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
					},
				},
				{
					Name:              "test-claim2",
					Namespace:         "test-ns",
					NodeName:          "node2",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 80G",
							State: "Failed",
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 1,
					},
				},
			},
			wantErr: false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
					},
				},
				{
					Name:              "test-claim2",
					Namespace:         "test-ns",
					NodeName:          "node2",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 80G",
							State: "Failed",
						},
					},
				},
			},
		},
		{
			name: "ResourceClaim not exit in resourceSlice",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-3",
								},
							},
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 1,
					},
				},
			},
			wantErr: false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Failed",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
		},
		{
			name: "ResourceClaim not exit in resourceSlice and set device state failed",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "composable1",
							Namespace: "test-ns",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "claim1",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-3",
								},
							},
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 1,
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to set devices state:",
		},
		{
			name: "failed to list composableResource",
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Model: "A100 80G",
								Size:  10,
							},
						},
					},
				},
			},
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "device-1",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
						{
							Name:              "device-2",
							Model:             "A100 40G",
							State:             "Preparing",
							ResourceSliceName: "test-rs",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Spec: resourceapi.ResourceClaimSpec{
							Devices: resourceapi.DeviceClaim{
								Requests: []resourceapi.DeviceRequest{
									{
										Name: "gpu0",
										FirstAvailable: []resourceapi.DeviceSubRequest{
											{
												Name:            "gpu",
												DeviceClassName: "gpu.nvidia.com",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-rs",
						},
						Spec: resourceapi.ResourceSliceSpec{
							Devices: []resourceapi.Device{
								{
									Name: "device-1",
								},
								{
									Name: "device-2",
								},
							},
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             1,
						CDIModelName:      "A100 40G",
						CannotCoexistWith: []int{2},
					},
					{
						Index:             2,
						CDIModelName:      "A100 80G",
						CannotCoexistWith: []int{1},
					},
				},
			},
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:     "A100 40G",
						MaxDevice: 1,
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to list composableResource:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			s := runtime.NewScheme()
			if tc.existingResourceClaimList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &resourceapi.ResourceClaim{}, &resourceapi.ResourceClaimList{})
				for i := range tc.existingResourceClaimList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceClaimList.Items[i])
				}
			}
			if tc.existingResourceSliceList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &resourceapi.ResourceSlice{}, &resourceapi.ResourceSliceList{})
				for i := range tc.existingResourceSliceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceSliceList.Items[i])
				}
			}
			if tc.existingRequestList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposabilityRequest{}, &cdioperator.ComposabilityRequestList{})
				for i := range tc.existingRequestList.Items {
					clientObjects = append(clientObjects, &tc.existingRequestList.Items[i])
				}
			}
			if tc.existingComposableResource != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})
				for i := range tc.existingComposableResource.Items {
					clientObjects = append(clientObjects, &tc.existingComposableResource.Items[i])
				}
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).WithStatusSubresource(&resourceapi.ResourceClaim{}).Build()

			result, err := RescheduleFailedNotification(context.Background(), fakeClient, tc.nodeInfo, tc.resourceClaims, tc.resourceSlices, tc.composableDRASpec)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assert.ElementsMatch(t, result, tc.expectedResourceClaims)
		})
	}
}

func TestRescheduleNotification(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name                      string
		existingResourceClaimList *resourceapi.ResourceClaimList
		existingResourceList      *cdioperator.ComposableResourceList
		resourceClaimInfos        []types.ResourceClaimInfo
		resourceSliceInfos        []types.ResourceSliceInfo
		labelPrefix               string
		deviceNoAllocation        time.Duration
		expectedResourceClaims    []types.ResourceClaimInfo
		wantErr                   bool
		expectedErrMsg            string
	}{
		{
			name: "failed to list composableResource",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Device: "device-1",
								},
								{
									Device: "device-2",
								},
							},
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device-1",
							UUID: "123",
						},
						{
							Name: "device-2",
							UUID: "456",
						},
					},
				},
			},
			deviceNoAllocation: time.Minute,
			labelPrefix:        "composable.test",
			wantErr:            true,
			expectedErrMsg:     "failed to list composableResource: ",
		},
		{
			name: "composableResource not exist",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Device: "device-1",
								},
								{
									Device: "device-2",
								},
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device-1",
							UUID: "123",
						},
						{
							Name: "device-2",
							UUID: "456",
						},
					},
				},
			},
			deviceNoAllocation: time.Minute,
			labelPrefix:        "composable.test",
			wantErr:            false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
		},
		{
			name: "resourceClaim with failed devices",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Device: "device-1",
								},
								{
									Device: "device-2",
								},
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource1",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "123",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource2",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "456",
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Failed",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Failed",
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device-1",
							UUID: "123",
						},
						{
							Name: "device-2",
							UUID: "456",
						},
					},
				},
			},
			deviceNoAllocation: time.Minute,
			labelPrefix:        "composable.test",
			wantErr:            false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Failed",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Failed",
						},
					},
				},
			},
		},
		{
			name: "failed to check if last used time is over threshold",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Device: "device-1",
								},
								{
									Device: "device-2",
								},
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource1",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "123",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource2",
							Annotations: map[string]string{
								"composable.test/last-used-time": "test",
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "456",
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device-1",
							UUID: "123",
						},
						{
							Name: "device-2",
							UUID: "456",
						},
					},
				},
			},
			deviceNoAllocation: time.Minute,
			labelPrefix:        "composable.test",
			wantErr:            false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
		},
		{
			name: "resourceClaim device used by pod",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Device: "device-1",
								},
								{
									Device: "device-2",
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Driver: "nvidia",
											Pool:   "test-pool",
											Device: "device-1",
										},
										{
											Driver: "nvidia",
											Pool:   "test-pool",
											Device: "device-2",
										},
									},
								},
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource1",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "123",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource2",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "456",
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Driver: "nvidia",
					Pool:   "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device-1",
							UUID: "123",
						},
						{
							Name: "device-2",
							UUID: "456",
						},
					},
				},
			},
			deviceNoAllocation: time.Minute,
			labelPrefix:        "composable.test",
			wantErr:            false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
		},
		{
			name: "composableResource not over time",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Device: "device-1",
								},
								{
									Device: "device-2",
								},
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource1",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-10 * time.Second).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "123",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource2",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-10 * time.Second).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "456",
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device-1",
							UUID: "123",
						},
						{
							Name: "device-2",
							UUID: "456",
						},
					},
				},
			},
			deviceNoAllocation: time.Minute,
			labelPrefix:        "composable.test",
			wantErr:            false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
		},
		{
			name: "normal case",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "test-ns",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Device: "device-1",
								},
								{
									Device: "device-2",
								},
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource1",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "123",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource2",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type:       "gpu",
							Model:      "A100 40G",
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:       "Online",
							CDIDeviceID: "456",
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device-1",
							UUID: "123",
						},
						{
							Name: "device-2",
							UUID: "456",
						},
					},
				},
			},
			deviceNoAllocation: time.Minute,
			labelPrefix:        "composable.test",
			wantErr:            false,
			expectedResourceClaims: []types.ResourceClaimInfo{
				{
					Name:              "test-claim",
					Namespace:         "test-ns",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now},
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "device-1",
							Model: "A100 40G",
							State: "Reschedule",
						},
						{
							Name:  "device-2",
							Model: "A100 40G",
							State: "Reschedule",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := runtime.NewScheme()
			clientObjects := []runtime.Object{}

			if tc.existingResourceList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})
				for i := range tc.existingResourceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceList.Items[i])
				}
			}

			if tc.existingResourceClaimList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &resourceapi.ResourceClaim{}, &resourceapi.ResourceClaimList{})
				for i := range tc.existingResourceClaimList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceClaimList.Items[i])
				}
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).WithStatusSubresource(&resourceapi.ResourceClaim{}).Build()

			result, err := RescheduleNotification(context.Background(), fakeClient, tc.resourceClaimInfos, tc.resourceSliceInfos, tc.labelPrefix, tc.deviceNoAllocation)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assert.ElementsMatch(t, result, tc.expectedResourceClaims)
		})
	}
}
