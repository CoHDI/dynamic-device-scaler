package utils

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
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

func TestIsLastUsedOverMinute(t *testing.T) {
	tests := []struct {
		name               string
		annotations        map[string]string
		deviceNoAllocation time.Duration
		expectedResult     bool
		expectedErr        bool
		errMsg             string
	}{
		{
			name:               "No annotations",
			annotations:        nil,
			deviceNoAllocation: time.Minute,
			expectedResult:     false,
			expectedErr:        true,
			errMsg:             "annotations not found",
		},
		{
			name:               "Annotation not found",
			annotations:        map[string]string{},
			deviceNoAllocation: time.Minute,
			expectedResult:     false,
		},
		{
			name: "Invalid time format",
			annotations: map[string]string{
				"composable.test/last-used-time": "invalid-time-format",
			},
			deviceNoAllocation: time.Minute,
			expectedResult:     false,
			expectedErr:        true,
			errMsg:             fmt.Errorf("failed to parse time: %v", fmt.Errorf("parsing time \"invalid-time-format\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"invalid-time-format\" as \"2006\"")).Error(),
		},
		{
			name: "Time less than deviceNoAllocation",
			annotations: map[string]string{
				"composable.test/last-used-time": time.Now().Add(-30 * time.Second).Format(time.RFC3339),
			},
			deviceNoAllocation: time.Minute,
			expectedResult:     false,
			expectedErr:        false,
		},
		{
			name: "Time more than deviceNoAllocation",
			annotations: map[string]string{
				"composable.test/last-used-time": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
			},
			deviceNoAllocation: time.Minute,
			expectedResult:     true,
			expectedErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations:       tt.annotations,
					CreationTimestamp: metav1.Now(),
				},
				Spec: cdioperator.ComposableResourceSpec{
					Type:       "gpu",
					Model:      "A100",
					TargetNode: "node1",
				},
			}

			result, err := isLastUsedOverTime(resource, "composable.test", tt.deviceNoAllocation)

			if tt.expectedErr {
				if err == nil {
					t.Errorf("Expected error, got none")
				} else if err.Error() != tt.errMsg {
					t.Errorf("Expected error message %q, got %q", tt.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if result != tt.expectedResult {
				t.Errorf("Expected result: %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestRescheduleFailedNotification(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name                      string
		existingResourceClaimList *resourceapi.ResourceClaimList
		existingRequestList       *cdioperator.ComposabilityRequestList
		nodeInfo                  types.NodeInfo
		composableDRASpec         types.ComposableDRASpec
		resourceClaims            []types.ResourceClaimInfo
		resourceSlices            []types.ResourceSliceInfo
		expectedResourceClaims    []types.ResourceClaimInfo
		wantErr                   bool
		expectedErrMsg            string
	}{
		{
			name: "setDevicesState failed",

			resourceClaims: []types.ResourceClaimInfo{
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
							Model: "A100 80G",
							State: "Preparing",
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
			expectedErrMsg: "failed to get ResourceClaim: resourceclaims.resource.k8s.io \"test-claim\" not found",
		},
		{
			name: "Device not coexistence",
			resourceClaims: []types.ResourceClaimInfo{
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
							Model: "A100 80G",
							State: "Preparing",
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
										Name:            "gpu",
										DeviceClassName: "gpu.nvidia.com",
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
							Name:  "device-1",
							Model: "A100 40G",
							State: "Failed",
						},
						{
							Name:  "device-2",
							Model: "A100 80G",
							State: "Failed",
						},
					},
				},
			},
		},
		{
			name: "ComposabilityRequestList exceed the maximum",
			resourceClaims: []types.ResourceClaimInfo{
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
										Name:            "gpu",
										DeviceClassName: "gpu.nvidia.com",
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
			name: "ComposabilityRequestList have different model",
			resourceClaims: []types.ResourceClaimInfo{
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
										Name:            "gpu",
										DeviceClassName: "gpu.nvidia.com",
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
			name: "ResourceClaimInfo devices do not coexist",
			resourceClaims: []types.ResourceClaimInfo{
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
										Name:            "gpu",
										DeviceClassName: "gpu.nvidia.com",
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
			expectedErrMsg: "failed to get ResourceClaim: resourceclaims.resource.k8s.io \"test-claim2\" not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			if tc.existingResourceClaimList != nil {
				for i := range tc.existingResourceClaimList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceClaimList.Items[i])
				}
			}
			if tc.existingRequestList != nil {
				for i := range tc.existingRequestList.Items {
					clientObjects = append(clientObjects, &tc.existingRequestList.Items[i])
				}
			}

			s := scheme.Scheme
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposabilityRequest{}, &cdioperator.ComposabilityRequestList{})
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := RescheduleFailedNotification(context.Background(), fakeClient, tc.nodeInfo, tc.resourceClaims, tc.resourceSlices, tc.composableDRASpec)

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
			if !reflect.DeepEqual(result, tc.expectedResourceClaims) {
				t.Errorf("resourceclaim infos are incorrect. Got: %v, Want: %v", result, tc.expectedResourceClaims)
			}
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
			clientObjects := []runtime.Object{}
			if tc.existingResourceClaimList != nil {
				for i := range tc.existingResourceClaimList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceClaimList.Items[i])
				}
			}
			if tc.existingResourceList != nil {
				for i := range tc.existingResourceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceList.Items[i])
				}
			}

			s := scheme.Scheme
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := RescheduleNotification(context.Background(), fakeClient, tc.resourceClaimInfos, tc.resourceSliceInfos, tc.labelPrefix, tc.deviceNoAllocation)

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
			if !reflect.DeepEqual(result, tc.expectedResourceClaims) {
				t.Errorf("resourceclaim infos are incorrect. Got: %v, Want: %v", result, tc.expectedResourceClaims)
			}
		})
	}
}
