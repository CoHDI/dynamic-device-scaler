package utils

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetResourceClaimInfo(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name                      string
		existingResourceClaimList *resourceapi.ResourceClaimList
		expectedResourceClaimInfo []types.ResourceClaimInfo
		wantErr                   bool
		expectedErrMsg            string
	}{
		{
			name: "normal case",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-claim-0",
							Namespace:         "default",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Status: resourceapi.ResourceClaimStatus{
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{},
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu-0",
									Device: "gpu.nvidia.com",
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-claim-1",
							Namespace:         "default",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Status: resourceapi.ResourceClaimStatus{
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Resource: "pods",
									Name:     "test-pod-1",
									UID:      "1234",
								},
							},
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu-1",
									Device: "gpu.nvidia.com",
								},
								{
									Driver: "gpu-2",
									Device: "gpu.nvidia.com",
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-claim-2",
							Namespace:         "default",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Status: resourceapi.ResourceClaimStatus{
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Resource: "pods",
									Name:     "test-pod-1",
									UID:      "1234",
								},
							},
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu-1",
									Device: "gpu.nvidia.com",
									Conditions: []metav1.Condition{
										{
											Type:   "FabricDeviceReschedule",
											Status: metav1.ConditionTrue,
										},
									},
								},
								{
									Driver: "gpu-2",
									Device: "gpu.nvidia.com",
									Conditions: []metav1.Condition{
										{
											Type:   "FabricDeviceFailed",
											Status: metav1.ConditionTrue,
										},
									},
								},
								{
									Driver: "gpu-3",
									Device: "gpu.nvidia.com",
									Conditions: []metav1.Condition{
										{
											Type:   "test-condition",
											Status: metav1.ConditionTrue,
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:              "test-claim-1",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: now.Truncate(time.Second)},
					Devices: []types.ResourceClaimDevice{
						{
							Name:      "gpu-1",
							UsedByPod: true,
						},
						{
							Name:      "gpu-2",
							UsedByPod: true,
						},
					},
				},
				{
					Name:              "test-claim-2",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: now.Truncate(time.Second)},
					Devices: []types.ResourceClaimDevice{
						{
							Name:      "gpu-1",
							UsedByPod: true,
							State:     "Reschedule",
						},
						{
							Name:      "gpu-2",
							UsedByPod: true,
							State:     "Failed",
						},
						{
							Name:      "gpu-3",
							UsedByPod: true,
							State:     "Preparing",
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

			s := scheme.Scheme

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := GetResourceClaimInfo(context.Background(), fakeClient)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				if err.Error() != tc.expectedErrMsg {
					t.Errorf("Error message is incorrect. Got: %q, Want: %q", err.Error(), tc.expectedErrMsg)
				}
				return
			}
			if !reflect.DeepEqual(result, tc.expectedResourceClaimInfo) {
				t.Errorf("Expected ResourceClaim info. Got: %v, Want: %v", result, tc.expectedResourceClaimInfo)
			}
		})
	}
}

func TestGetResourceSliceInfo(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name                      string
		existingResourceSliceList *resourceapi.ResourceSliceList
		expectedResourceSliceInfo []types.ResourceSliceInfo
		wantErr                   bool
		expectedErrMsg            string
	}{
		{
			name: "normal case",
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-resourceslice-1",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Spec: resourceapi.ResourceSliceSpec{
							Driver:   "gpu.nvidia.com",
							NodeName: "node1",
							Devices: []resourceapi.Device{
								{
									Name: "gpu-0",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid": {StringValue: ptr.To("1234")},
										},
									},
								},
								{
									Name: "gpu-1",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid": {StringValue: ptr.To("5678")},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:              "test-resourceslice-1",
					CreationTimestamp: metav1.Time{Time: now.Truncate(time.Second)},
					Driver:            "gpu.nvidia.com",
					NodeName:          "node1",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu-0",
							UUID: "1234",
						},
						{
							Name: "gpu-1",
							UUID: "5678",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			if tc.existingResourceSliceList != nil {
				for i := range tc.existingResourceSliceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceSliceList.Items[i])
				}
			}

			s := scheme.Scheme

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := GetResourceSliceInfo(context.Background(), fakeClient)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				if err.Error() != tc.expectedErrMsg {
					t.Errorf("Error message is incorrect. Got: %q, Want: %q", err.Error(), tc.expectedErrMsg)
				}
				return
			}
			if !reflect.DeepEqual(result, tc.expectedResourceSliceInfo) {
				t.Errorf("Expected ResourceSlice info. Got: %v, Want: %v", result, tc.expectedResourceSliceInfo)
			}
		})
	}
}

func TestGetNodeInfo(t *testing.T) {
	testCases := []struct {
		name              string
		existingNode      *corev1.NodeList
		composableDRASpec types.ComposableDRASpec
		expectedNodeInfos []types.NodeInfo
		wantErr           bool
		expectedErrMsg    string
	}{
		{
			name: "normal case",
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 80G",
						K8sDeviceName: "nvidia-a100-80g",
					},
				},
			},
			existingNode: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Labels: map[string]string{
								"composable.fsastech.com/nvidia-a100-80g":          "true",
								"composable.fsastech.com/fabric":                   "123",
								"composable.fsastech.com/nvidia-a100-80g-size-min": "2",
								"composable.fsastech.com/nvidia-a100-80g-size-max": "6",
							},
						},
					},
				},
			},
			expectedNodeInfos: []types.NodeInfo{
				{
					Name:     "node1",
					FabricID: "123",
					Models: []types.ModelConstraints{
						{
							Model:      "A100 80G",
							DeviceName: "nvidia-a100-80g",
							MinDevice:  2,
							MaxDevice:  6,
						},
					},
				},
			},
		},
		{
			name: "error get model name",
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40g",
					},
				},
			},
			existingNode: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Labels: map[string]string{
								"composable.fsastech.com/nvidia-a100-80g":          "true",
								"composable.fsastech.com/fabric":                   "123",
								"composable.fsastech.com/nvidia-a100-80g-size-min": "2",
								"composable.fsastech.com/nvidia-a100-80g-size-max": "6",
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "unknown device name: nvidia-a100-80g",
		},
		{
			name: "invalid integer",
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 80G",
						K8sDeviceName: "nvidia-a100-80g",
					},
				},
			},
			existingNode: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Labels: map[string]string{
								"composable.fsastech.com/nvidia-a100-80g":          "true",
								"composable.fsastech.com/fabric":                   "123",
								"composable.fsastech.com/nvidia-a100-80g-size-min": "ss",
								"composable.fsastech.com/nvidia-a100-80g-size-max": "6",
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "invalid integer in ss: strconv.Atoi: parsing \"ss\": invalid syntax",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeObjects := []runtime.Object{}
			if tc.existingNode != nil {
				kubeObjects = append(kubeObjects, tc.existingNode)
			}
			kubeClient := k8sfake.NewClientset(kubeObjects...)

			result, err := GetNodeInfo(context.Background(), kubeClient, tc.composableDRASpec)

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

			if !reflect.DeepEqual(result, tc.expectedNodeInfos) {
				t.Errorf("NodeInfos are incorrect. Got: %v, Want: %v", result, tc.expectedNodeInfos)
			}
		})
	}
}

func TestGetModelName(t *testing.T) {
	tests := []struct {
		name              string
		deviceName        string
		composableDRASpec types.ComposableDRASpec
		wantErr           bool
		expectedErrMsg    string
		expectedResult    string
	}{
		{
			name:       "unknown device name",
			deviceName: "nvidia-a33",
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40",
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "unknown device name: nvidia-a33",
		},
		{
			name:       "normal device name",
			deviceName: "nvidia-a100-40",
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40",
					},
				},
			},
			expectedResult: "A100 40G",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := getModelName(tc.composableDRASpec, tc.deviceName)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
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
				t.Errorf("Unexpected model name. Got: %s, Want: %s", result, tc.expectedErrMsg)
			}
		})
	}
}

func TestGetConfigMapInfo(t *testing.T) {
	tests := []struct {
		name            string
		configMapData   map[string]string
		createConfigMap bool
		wantSpec        types.ComposableDRASpec
		wantErr         bool
		expectedErrMsg  string
	}{
		{
			name: "Success info",
			configMapData: map[string]string{
				"device-info": `
- index: 1
  cdi-model-name: "A100 40G"
  dra-attributes:
    - productName: "NVIDIA A100 40GB PCIe"
  label-key-model: "composable-a100-40G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-40"
  cannot-coexist-with: [2, 3, 4]
            `,
				"label-prefix":    "composable.fsastech.com",
				"fabric-id-range": `[1, 2, 3]`,
			},
			createConfigMap: true,
			wantSpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:        1,
						CDIModelName: "A100 40G",
						DRAttributes: []types.DRAttribute{
							{
								ProductName: "NVIDIA A100 40GB PCIe",
							},
						},
						LabelKeyModel:     "composable-a100-40G",
						DriverName:        "gpu.nvidia.com",
						K8sDeviceName:     "nvidia-a100-40",
						CannotCoexistWith: []int{2, 3, 4},
					},
				},
				LabelPrefix:   "composable.fsastech.com",
				FabricIDRange: []int{1, 2, 3},
			},
			wantErr: false,
		},
		{
			name:            "Configmap not found",
			createConfigMap: false,
			wantErr:         true,
			expectedErrMsg:  "failed to get ConfigMap",
		},
		{
			name: "Invalid device info",
			configMapData: map[string]string{
				"device-info":  "invalid yaml",
				"label-prefix": "test-",
			},
			createConfigMap: true,
			wantErr:         true,
			expectedErrMsg:  "failed to parse device-info",
		},
		{
			name: "Invalid fabric-id-range",
			configMapData: map[string]string{
				"device-info": `
- index: 1
  cdi-model-name: "A100 40G"
  dra-attributes:
    - productName: "NVIDIA A100 40GB PCIe"
  label-key-model: "composable-a100-40G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-40"
  cannot-coexist-with: [2, 3, 4]
            `,
				"label-prefix":    "test-",
				"fabric-id-range": "invalid info",
			},
			createConfigMap: true,
			wantErr:         true,
			expectedErrMsg:  "failed to parse fabric-id-range",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var clientSet *k8sfake.Clientset
			if tc.createConfigMap {
				clientSet = k8sfake.NewSimpleClientset(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "composable-dra-dds",
						Namespace: "composable-dra",
					},
					Data: tc.configMapData,
				})
			} else {
				clientSet = k8sfake.NewSimpleClientset()
			}

			result, err := GetConfigMapInfo(context.Background(), clientSet)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !strings.Contains(err.Error(), tc.expectedErrMsg) {
					t.Errorf("error message %q does not contain %q", err.Error(), tc.expectedErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(result, tc.wantSpec) {
				t.Errorf("got %+v, want %+v", result, tc.wantSpec)
			}
		})
	}
}
