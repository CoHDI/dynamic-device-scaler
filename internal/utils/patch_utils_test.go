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
	"reflect"
	"testing"

	cdioperator "github.com/CoHDI/composable-resource-operator/api/v1alpha1"
	"github.com/CoHDI/dynamic-device-scaler/internal/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateNodeLabel(t *testing.T) {
	testCases := []struct {
		name                 string
		existingRequestList  *cdioperator.ComposabilityRequestList
		existingResourceList *cdioperator.ComposableResourceList
		existingNode         *corev1.Node
		nodeName             string
		composableDRASpec    types.ComposableDRASpec
		expectedNodeLabels   map[string]string
		wantErr              bool
		expectedErrMsg       string
	}{
		{
			name:           "failed to list composabilityRequestList",
			nodeName:       "test",
			wantErr:        true,
			expectedErrMsg: "failed to list composabilityRequestList:",
		},
		{
			name: "failed to list ComposableResourceList",
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
			nodeName:       "test",
			wantErr:        true,
			expectedErrMsg: "failed to list ComposableResourceList",
		},
		{
			name: "node not exist",
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
			nodeName:       "test",
			wantErr:        true,
			expectedErrMsg: "patch failed:",
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource3",
						},
						Spec: cdioperator.ComposableResourceSpec{
							Model:      "A100 60G",
							TargetNode: "test",
						},
						Status: cdioperator.ComposableResourceStatus{
							State: "Online",
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
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:        1,
						CDIModelName: "A100 40G",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB PCIe",
						},
						LabelKeyModel:     "composable-a100-40G",
						DriverName:        "gpu.nvidia.com",
						K8sDeviceName:     "nvidia-a100-40g",
						CannotCoexistWith: []int{2, 3},
					},
					{
						Index:        2,
						CDIModelName: "A100 80G",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 80GB PCIe",
						},
						LabelKeyModel:     "composable-a100-80G",
						DriverName:        "gpu.nvidia.com",
						K8sDeviceName:     "nvidia-a100-80g",
						CannotCoexistWith: []int{1, 3},
					},
					{
						Index:        3,
						CDIModelName: "H100",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA H100 PCIe",
						},
						LabelKeyModel:     "composable-h100",
						DriverName:        "gpu.nvidia.com",
						K8sDeviceName:     "nvidia-h100",
						CannotCoexistWith: []int{2, 3},
					},
					{
						Index:        4,
						CDIModelName: "CXL-mem",
						DRAAttributes: map[string]string{
							"productName": "CXL mem",
						},
						DriverName:        "cxl-mem",
						LabelKeyModel:     "cxl-mem",
						K8sDeviceName:     "cxl-mem",
						CannotCoexistWith: []int{2, 3},
					},
				},
				LabelPrefix:   "composable.fsastech.com",
				FabricIDRange: []int{1, 2, 3},
			},
			nodeName: "test",
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeObjects := []runtime.Object{}
			if tc.existingNode != nil {
				kubeObjects = append(kubeObjects, tc.existingNode)
			}

			kubeClient := k8sfake.NewClientset(kubeObjects...)
			s := runtime.NewScheme()

			clientObjects := []runtime.Object{}
			if tc.existingRequestList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposabilityRequest{}, &cdioperator.ComposabilityRequestList{})

				for i := range tc.existingRequestList.Items {
					clientObjects = append(clientObjects, &tc.existingRequestList.Items[i])
				}
			}
			if tc.existingResourceList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})

				for i := range tc.existingResourceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceList.Items[i])
				}
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			err := UpdateNodeLabel(context.Background(), fakeClient, kubeClient, tc.nodeName, tc.composableDRASpec)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			updatedNode, err := kubeClient.CoreV1().Nodes().Get(context.Background(), tc.nodeName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get updated node: %v", err)
			}

			if !reflect.DeepEqual(updatedNode.Labels, tc.expectedNodeLabels) {
				t.Errorf("Node labels are incorrect. Got: %v, Want: %v", updatedNode.Labels, tc.expectedNodeLabels)
			}
		})
	}
}

func TestPatchComposabilityRequestSize(t *testing.T) {
	testCases := []struct {
		name                string
		existingRequestList *cdioperator.ComposabilityRequestList
		requestName         string
		count               int64
		wantErr             bool
		expectedSize        int64
		expectedErrMsg      string
	}{
		{
			name: "normal case",
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "request1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:  "gpu",
								Size:  1,
								Model: "A100 40G",
							},
						},
					},
				},
			},
			requestName:  "request1",
			count:        3,
			expectedSize: 3,
		},
		{
			name:           "failed to get ComposabilityRequest",
			requestName:    "request1",
			wantErr:        true,
			expectedErrMsg: "failed to get ComposabilityRequest:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			s := runtime.NewScheme()
			if tc.existingRequestList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposabilityRequest{}, &cdioperator.ComposabilityRequestList{})

				for i := range tc.existingRequestList.Items {
					clientObjects = append(clientObjects, &tc.existingRequestList.Items[i])
				}
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			err := PatchComposabilityRequestSize(context.Background(), fakeClient, tc.requestName, tc.count)

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

			updatedRequest := &cdioperator.ComposabilityRequest{}
			err = fakeClient.Get(
				context.Background(),
				k8stypes.NamespacedName{Name: tc.requestName},
				updatedRequest,
			)
			if err != nil {
				t.Fatalf("Failed to get updated node: %v", err)
			}

			if updatedRequest.Spec.Resource.Size != tc.expectedSize {
				t.Errorf("Unexpected ComposabilityRequest size. Got: %v, Want: %v", updatedRequest.Spec.Resource.Size, tc.expectedSize)
			}
		})
	}
}

func TestPatchResourceClaimDeviceConditions(t *testing.T) {
	testCases := []struct {
		name                      string
		existingResourceClaimList *resourceapi.ResourceClaimList
		resourceClaimName         string
		namespace                 string
		conditionType             string
		wantErr                   bool
		expectedErrMsg            string
	}{
		{
			name: "normal case",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "resource1",
							Namespace: "default",
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
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Device: "gpu-0",
									Driver: "gpu.nvidia.com",
									Pool:   "k8s-dra-driver",
								},
							},
						},
					},
				},
			},
			resourceClaimName: "resource1",
			namespace:         "default",
			conditionType:     "FabricDeviceReschedule",
		},
		{
			name: "device not exist",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "resource1",
							Namespace: "default",
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
						Status: resourceapi.ResourceClaimStatus{
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "gpu-0",
											Driver: "gpu.nvidia.com",
											Pool:   "k8s-dra-driver",
										},
									},
								},
							},
						},
					},
				},
			},
			resourceClaimName: "resource1",
			namespace:         "default",
			conditionType:     "FabricDeviceReschedule",
		},
		{
			name: "device condition already exists",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "resource1",
							Namespace: "default",
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
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Device: "gpu-0",
									Driver: "gpu.nvidia.com",
									Pool:   "k8s-dra-driver",
									Conditions: []metav1.Condition{
										{
											Type:               "FabricDeviceReschedule",
											Status:             metav1.ConditionFalse,
											Reason:             "DeviceConditionUpdated",
											Message:            "Device gpu-0 condition FabricDeviceReschedule updated",
											LastTransitionTime: metav1.Now(),
										},
									},
								},
							},
						},
					},
				},
			},
			resourceClaimName: "resource1",
			namespace:         "default",
			conditionType:     "FabricDeviceReschedule",
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

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).WithStatusSubresource(&resourceapi.ResourceClaim{}).Build()

			err := PatchResourceClaimDeviceConditions(context.Background(), fakeClient, tc.resourceClaimName, tc.namespace, tc.conditionType)

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

			updatedRequest := &resourceapi.ResourceClaim{}
			err = fakeClient.Get(
				context.Background(),
				k8stypes.NamespacedName{Name: tc.resourceClaimName, Namespace: tc.namespace},
				updatedRequest,
			)
			if err != nil {
				t.Fatalf("Failed to get updated node: %v", err)
			}

			found := false
			for _, cond := range updatedRequest.Status.Devices[0].Conditions {
				if cond.Type == tc.conditionType && cond.Status == metav1.ConditionTrue {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected condition %s not found in Device Conditions", tc.conditionType)
			}
		})
	}
}

func TestPatchComposableResourceAnnotation(t *testing.T) {
	testCases := []struct {
		name                           string
		existingComposableResourceList *cdioperator.ComposableResourceList
		resourceName                   string
		key                            string
		value                          string
		wantErr                        bool
		expectedErrMsg                 string
	}{
		{
			name:           "failed to get ComposableResource",
			resourceName:   "test",
			wantErr:        true,
			expectedErrMsg: "failed to get latest ComposableResource:",
		},
		{
			name: "normal case",
			existingComposableResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type: "GPU",
						},
					},
				},
			},
			resourceName: "test",
			key:          "test-key",
			value:        "test-value",
		},
		{
			name: "annotation exist",
			existingComposableResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								"test-key": "test-value",
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							Type: "GPU",
						},
					},
				},
			},
			resourceName: "test",
			key:          "test-key",
			value:        "test-value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := runtime.NewScheme()
			clientObjects := []runtime.Object{}
			if tc.existingComposableResourceList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})
				for i := range tc.existingComposableResourceList.Items {
					clientObjects = append(clientObjects, &tc.existingComposableResourceList.Items[i])
				}
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			err := PatchComposableResourceAnnotation(context.Background(), fakeClient, tc.resourceName, tc.key, tc.value)

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

			updatedComposableResource := &cdioperator.ComposableResource{}
			err = fakeClient.Get(
				context.Background(),
				k8stypes.NamespacedName{Name: tc.resourceName},
				updatedComposableResource,
			)
			if err != nil {
				t.Fatalf("Failed to get updated ComposableResource: %v", err)
			}

			if updatedComposableResource.Annotations[tc.key] != tc.value {
				t.Errorf("Expected annotation %q to be %q, but got %q",
					tc.key, tc.value, updatedComposableResource.Annotations[tc.key])
			}
		})
	}
}
