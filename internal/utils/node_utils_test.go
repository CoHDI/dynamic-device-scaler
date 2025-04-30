package utils

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
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
	}{
		{
			name: "normal descending order",
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
			name:  "empty slice",
			input: []types.ResourceClaimInfo{},
			want:  []types.ResourceClaimInfo{},
		},
		{
			name: "single element",
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

			sortByTime(got)

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
		name           string
		annotations    map[string]string
		expectedResult bool
		expectedErr    bool
		errMsg         string
	}{
		{
			name:           "No annotations",
			annotations:    nil,
			expectedResult: false,
			expectedErr:    true,
			errMsg:         "annotations not found",
		},
		{
			name:           "Annotation not found",
			annotations:    map[string]string{},
			expectedResult: false,
			expectedErr:    true,
			errMsg:         "annotation composable.test/last-used-time not found",
		},
		{
			name: "Invalid time format",
			annotations: map[string]string{
				"composable.test/last-used-time": "invalid-time-format",
			},
			expectedResult: false,
			expectedErr:    true,
			errMsg:         fmt.Errorf("failed to parse time: %v", fmt.Errorf("parsing time \"invalid-time-format\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"invalid-time-format\" as \"2006\"")).Error(),
		},
		{
			name: "Time less than a minute ago",
			annotations: map[string]string{
				"composable.test/last-used-time": time.Now().Add(-30 * time.Second).Format(time.RFC3339),
			},
			expectedResult: false,
			expectedErr:    false,
		},
		{
			name: "Time more than a minute ago",
			annotations: map[string]string{
				"composable.test/last-used-time": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
			},
			expectedResult: true,
			expectedErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
				Spec: cdioperator.ComposableResourceSpec{
					Type:       "gpu",
					Model:      "A100",
					TargetNode: "node1",
				},
			}

			result, err := isLastUsedOverMinute(resource)

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

func TestSetDevicesState(t *testing.T) {
	testCases := []struct {
		name              string
		resourceClaimInfo types.ResourceClaimInfo
		targetState       string
		conditionType     string
		existingRC        *resourceapi.ResourceClaim
		expectedRC        *resourceapi.ResourceClaim
		expectedRCInfo    types.ResourceClaimInfo
		wantErr           bool
	}{
		{
			name: "ResourceClaim with empty condition",
			resourceClaimInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Preparing",
					},
				},
			},
			targetState:   "Reschedule",
			conditionType: "FabricDeviceReschedule",
			existingRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device:     "device-1",
							Conditions: []metav1.Condition{},
						},
					},
				},
			},
			expectedRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expectedRCInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Reschedule",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ResourceClaim with non-empty condition",
			resourceClaimInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Preparing",
					},
				},
			},
			targetState:   "Reschedule",
			conditionType: "FabricDeviceReschedule",
			existingRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionFalse,
								},
							},
						},
					},
				},
			},
			expectedRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expectedRCInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Reschedule",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ResourceClaim with matched condition",
			resourceClaimInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Preparing",
					},
				},
			},
			targetState:   "Reschedule",
			conditionType: "FabricDeviceReschedule",
			existingRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expectedRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expectedRCInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Reschedule",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ResourceClaim not found",
			resourceClaimInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Preparing",
					},
				},
			},
			targetState:   "Ready",
			conditionType: "Ready",
			existingRC:    nil,
			expectedRC:    nil,
			wantErr:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			objs := []runtime.Object{}
			if tc.existingRC != nil {
				objs = append(objs, tc.existingRC)
			}

			s := scheme.Scheme
			s.AddKnownTypes(resourceapi.SchemeGroupVersion, &resourceapi.ResourceClaim{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).WithStatusSubresource(&resourceapi.ResourceClaim{}).Build()

			result, err := setDevicesState(ctx, fakeClient, tc.resourceClaimInfo, tc.targetState, tc.conditionType)

			if (err != nil) != tc.wantErr {
				t.Fatalf("setDevicesState() error = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr {
				updatedRC := &resourceapi.ResourceClaim{}
				namespacedName := k8stypes.NamespacedName{
					Name:      tc.resourceClaimInfo.Name,
					Namespace: tc.resourceClaimInfo.Namespace,
				}
				err := fakeClient.Get(ctx, namespacedName, updatedRC)
				if err != nil {
					t.Fatalf("failed to get updated ResourceClaim: %v", err)
				}

				for i := range updatedRC.Status.Devices {
					for j := range updatedRC.Status.Devices[i].Conditions {
						updatedRC.Status.Devices[i].Conditions[j].LastTransitionTime = metav1.Time{}
						if tc.expectedRC != nil && tc.expectedRC.Status.Devices != nil && len(tc.expectedRC.Status.Devices) > 0 {
							tc.expectedRC.Status.Devices[i].Conditions[j].LastTransitionTime = metav1.Time{}
						}
					}
				}

				if !reflect.DeepEqual(updatedRC.Status.Devices, tc.expectedRC.Status.Devices) {
					t.Errorf("ResourceClaim was not updated as expected.\nGot:  %#v\nWant: %#v", updatedRC.Status.Devices, tc.expectedRC.Status.Devices)
				}
				if !reflect.DeepEqual(result, tc.expectedRCInfo) {
					t.Errorf("ResourceClaimInfo was not updated as expected.\nGot:  %#v\nWant: %#v", result, tc.expectedRCInfo)
				}
			}
		})
	}
}

func TestUpdateNodeLabel(t *testing.T) {
	configMapData := map[string]string{
		"device-info": `
- index: 1
  cdi-model-name: "A100 40G"
  dra-attributes:
    - productName: "NVIDIA A100 40GB PCIe"
  label-key-model: "composable-a100-40G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-40g"
  cannot-coexist-with: [2, 3]
- index: 2
  cdi-model-name: "A100 80G"
  dra-attributes:
    - productName: "NVIDIA A100 80GB PCIe"
  label-key-model: "composable-a100-80G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-80g"
  cannot-coexist-with: [1, 3]
- index: 3
  cdi-model-name: "H100"
  dra-attributes:
    - productName: "NVIDIA H100 PCIe"
  label-key-model: "composable-h100"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-h100"
  cannot-coexist-with: [1, 2]
- index: 4
  cdi-model-name: "CXL-mem"
  dra-attributes:
    - productName: "CXL mem"
  label-key-model: "cxl-mem"
  driver-name: "cxl-mem"
  k8s-device-name: "cxl-mem"
  cannot-coexist-with: []
`,
		"label-prefix":    "composable.fsastech.com",
		"fabric-id-range": "[1, 2, 3]",
	}

	testCases := []struct {
		name                 string
		existingRequestList  *cdioperator.ComposabilityRequestList
		existingResourceList *cdioperator.ComposableResourceList
		existingNode         *corev1.Node
		nodeInfo             types.NodeInfo
		expectedNodeLabels   map[string]string
		wantErr              bool
		expectedErrMsg       string
	}{
		{
			name: "node not exist",
			nodeInfo: types.NodeInfo{
				Name: "test",
			},
			wantErr:        true,
			expectedErrMsg: "failed to get node: nodes \"test\" not found",
		},
		{
			name: "update node label successfully",
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Size:       2,
								Model:      "A100 40G",
								TargetNode: "test",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request2",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Size:       0,
								Model:      "A100 80G",
								TargetNode: "test",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request3",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Size:       2,
								Model:      "H100",
								TargetNode: "test2",
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
						},
						Spec: cdioperator.ComposableResourceSpec{
							Model:      "A100 40G",
							TargetNode: "test",
						},
						Status: cdioperator.ComposableResourceStatus{
							State: "Online",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource2",
						},
						Spec: cdioperator.ComposableResourceSpec{
							Model:      "A100 80G",
							TargetNode: "test",
						},
						Status: cdioperator.ComposableResourceStatus{
							State: "Deleting",
						},
					},
				},
			},
			existingNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						"composable.fsastech.com/nvidia-a100-80g": "true",
					},
				},
			},
			expectedNodeLabels: map[string]string{
				"composable.fsastech.com/nvidia-a100-40g": "true",
				"composable.fsastech.com/cxl-mem":         "true",
			},
			nodeInfo: types.NodeInfo{
				Name: "test",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeObjects := []runtime.Object{}
			if tc.existingNode != nil {
				kubeObjects = append(kubeObjects, tc.existingNode)
			}
			kubeObjects = append(kubeObjects, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "composable-dra-dds",
					Namespace: "composable-dra",
				},
				Data: configMapData,
			})

			kubeClient := k8sfake.NewClientset(kubeObjects...)

			clientObjects := []runtime.Object{}
			if tc.existingRequestList != nil {
				for i := range tc.existingRequestList.Items {
					clientObjects = append(clientObjects, &tc.existingRequestList.Items[i])
				}
			}
			if tc.existingResourceList != nil {
				for i := range tc.existingResourceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceList.Items[i])
				}
			}

			s := scheme.Scheme
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposabilityRequest{}, &cdioperator.ComposabilityRequestList{})
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			err := UpdateNodeLabel(context.Background(), fakeClient, kubeClient, tc.nodeInfo)

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
			updatedNode, err := kubeClient.CoreV1().Nodes().Get(context.Background(), tc.nodeInfo.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get updated node: %v", err)
			}

			if !reflect.DeepEqual(updatedNode.Labels, tc.expectedNodeLabels) {
				t.Errorf("Node labels are incorrect. Got: %v, Want: %v", updatedNode.Labels, tc.expectedNodeLabels)
			}
		})
	}
}

func TestIsResourceSliceRed(t *testing.T) {
	testCases := []struct {
		name                 string
		existingResourceList *cdioperator.ComposableResourceList
		claimDeviceName      string
		expectedResult       bool
		wantErr              bool
		expectedErrMsg       string
	}{
		{
			name:           "empty resource list",
			expectedResult: false,
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

			result, err := isResourceSliceRed(context.Background(), fakeClient, tc.claimDeviceName)

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
			if result != tc.expectedResult {
				t.Errorf("Expected result %v, got %v", tc.expectedResult, result)
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
				Name:     "node1",
				FabricID: "1",
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
				Name:     "node1",
				FabricID: "1",
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
				Name:     "node1",
				FabricID: "1",
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
				Name:     "node1",
				FabricID: "1",
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
		{
			name: "Device exceed the maximum",
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
				Name:     "node1",
				FabricID: "1",
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
								Model: "A100 40G",
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

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := RescheduleFailedNotification(context.Background(), fakeClient, tc.nodeInfo, tc.resourceClaims, tc.composableDRASpec)

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
		nodeInfo                  types.NodeInfo
		resourceClaims            []types.ResourceClaimInfo
		expectedResourceClaims    []types.ResourceClaimInfo
		wantErr                   bool
		expectedErrMsg            string
	}{
		{
			name: "ResourceSlice not red",

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

			result, err := RescheduleNotification(context.Background(), fakeClient, tc.nodeInfo, tc.resourceClaims)

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
