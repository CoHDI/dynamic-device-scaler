package utils

import (
	"context"
	"reflect"
	"testing"
	"time"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	"github.com/stretchr/testify/assert"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetConfiguredDeviceCount(t *testing.T) {
	testCases := []struct {
		name                           string
		existingComposableResourceList *cdioperator.ComposableResourceList
		existingResourceClaim          *resourceapi.ResourceClaimList
		resourceClaimInfos             []types.ResourceClaimInfo
		resourceSliceInfos             []types.ResourceSliceInfo
		model                          string
		nodeName                       string
		expectedResult                 int64
		wantErr                        bool
		expectedErrMsg                 string
	}{
		{
			name: "failed to list composableResourceList",
			existingResourceClaim: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "claim1",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu.nvidia.com",
									Pool:   "test",
									Device: "device1",
								},
								{
									Driver: "gpu.nvidia.com",
									Pool:   "test",
									Device: "device2",
								},
							},
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Name:     "pod1",
									Resource: "pods",
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "device1",
											Pool:   "test",
											Driver: "gpu.nvidia.com",
										},
									},
								},
							},
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Name:   "rs1",
					Driver: "gpu.nvidia.com",
					Pool:   "test",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device1",
							UUID: "123",
						},
						{
							Name: "device2",
							UUID: "456",
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:     "rs1",
					NodeName: "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "GPU2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
				{
					Name:     "rs2",
					NodeName: "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU4",
							Model: "A100 80G",
							State: "Preparing",
						},
					},
				},
				{
					Name:     "rs3",
					NodeName: "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU5",
							Model: "A100 40G",
							State: "Reschedule",
						},
					},
				},
			},
			model:          "A100 40G",
			nodeName:       "node1",
			wantErr:        true,
			expectedErrMsg: "failed to list composableResourceList:",
		},
		{
			name: "resourceClaim and nodeName do not match",
			existingComposableResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource1",
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node1",
							Model:      "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:    "Online",
							DeviceID: "123",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource2",
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node2",
							Model:      "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:    "Online",
							DeviceID: "123",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource3",
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node1",
							Model:      "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:    "Failed",
							DeviceID: "123",
						},
					},
				},
			},
			existingResourceClaim: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "claim1",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu.nvidia.com",
									Pool:   "test",
									Device: "device1",
								},
								{
									Driver: "gpu.nvidia.com",
									Pool:   "test",
									Device: "device2",
								},
							},
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Name:     "pod1",
									Resource: "pods",
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "device1",
											Pool:   "test",
											Driver: "gpu.nvidia.com",
										},
									},
								},
							},
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Name:   "rs1",
					Driver: "gpu.nvidia.com",
					Pool:   "test",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device1",
							UUID: "123",
						},
						{
							Name: "device2",
							UUID: "456",
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:     "rs1",
					NodeName: "node2",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "GPU2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
				{
					Name:     "rs2",
					NodeName: "node2",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU4",
							Model: "A100 80G",
							State: "Preparing",
						},
					},
				},
				{
					Name:     "rs3",
					NodeName: "node2",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU5",
							Model: "A100 40G",
							State: "Reschedule",
						},
					},
				},
			},
			model:          "A100 40G",
			nodeName:       "node1",
			expectedResult: 1,
		},
		{
			name: "pod allocated devices is 0",
			existingResourceClaim: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "claim1",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu.nvidia.com",
									Pool:   "test",
									Device: "device1",
								},
								{
									Driver: "gpu.nvidia.com",
									Pool:   "test",
									Device: "device2",
								},
							},
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Name:     "pod1",
									Resource: "pods",
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "device1",
											Pool:   "test",
											Driver: "gpu.nvidia.com",
										},
									},
								},
							},
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Name:   "rs1",
					Driver: "gpu.nvidia.com",
					Pool:   "test",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device1",
							UUID: "123",
						},
						{
							Name: "device2",
							UUID: "456",
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:     "rs1",
					NodeName: "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "GPU2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
				{
					Name:     "rs2",
					NodeName: "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU4",
							Model: "A100 80G",
							State: "Preparing",
						},
					},
				},
				{
					Name:     "rs3",
					NodeName: "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU5",
							Model: "A100 40G",
							State: "Reschedule",
						},
					},
				},
			},
			model:          "A100 40G",
			nodeName:       "node1",
			expectedResult: 3,
		},
		{
			name: "normal case",
			existingComposableResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource1",
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node1",
							Model:      "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:    "Online",
							DeviceID: "123",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource2",
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node2",
							Model:      "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:    "Online",
							DeviceID: "123",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "resource3",
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node1",
							Model:      "A100 40G",
						},
						Status: cdioperator.ComposableResourceStatus{
							State:    "Failed",
							DeviceID: "123",
						},
					},
				},
			},
			existingResourceClaim: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "claim1",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu.nvidia.com",
									Pool:   "test",
									Device: "device1",
								},
								{
									Driver: "gpu.nvidia.com",
									Pool:   "test",
									Device: "device2",
								},
							},
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Name:     "pod1",
									Resource: "pods",
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "device1",
											Pool:   "test",
											Driver: "gpu.nvidia.com",
										},
									},
								},
							},
						},
					},
				},
			},
			resourceSliceInfos: []types.ResourceSliceInfo{
				{
					Name:   "rs1",
					Driver: "gpu.nvidia.com",
					Pool:   "test",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "device1",
							UUID: "123",
						},
						{
							Name: "device2",
							UUID: "456",
						},
					},
				},
			},
			resourceClaimInfos: []types.ResourceClaimInfo{
				{
					Name:     "rs1",
					NodeName: "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "GPU2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
				{
					Name:     "rs2",
					NodeName: "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU4",
							Model: "A100 80G",
							State: "Preparing",
						},
					},
				},
				{
					Name:     "rs3",
					NodeName: "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU5",
							Model: "A100 40G",
							State: "Reschedule",
						},
					},
				},
			},
			model:          "A100 40G",
			nodeName:       "node1",
			expectedResult: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := scheme.Scheme

			clientObjects := []runtime.Object{}
			if tc.existingComposableResourceList != nil {
				for i := range tc.existingComposableResourceList.Items {
					clientObjects = append(clientObjects, &tc.existingComposableResourceList.Items[i])
				}
				s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})
			}
			if tc.existingResourceClaim != nil {
				for i := range tc.existingResourceClaim.Items {
					clientObjects = append(clientObjects, &tc.existingResourceClaim.Items[i])
				}
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := GetConfiguredDeviceCount(context.Background(), fakeClient, tc.model, tc.nodeName, tc.resourceClaimInfos, tc.resourceSliceInfos)

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

			if result != tc.expectedResult {
				t.Errorf("Unexpected result. Got: %v, Want: %v", result, tc.expectedResult)
			}
		})
	}
}

func TestDynamicAttach(t *testing.T) {
	testCases := []struct {
		name                         string
		existingComposabilityRequest *cdioperator.ComposabilityRequestList
		updateComposabilityRequest   *cdioperator.ComposabilityRequest
		count                        int64
		model                        string
		nodeName                     string
		resourceType                 string
		wantErr                      bool
		expectedErrMsg               string
	}{
		{
			name:                       "empty update ComposabilityRequest",
			updateComposabilityRequest: nil,
			model:                      "A100 40G",
			count:                      2,
			nodeName:                   "node1",
		},
		{
			name: "empty existing ComposabilityRequest",
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       2,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			count:          2,
			wantErr:        true,
			expectedErrMsg: "failed to get ComposabilityRequest: composabilityrequests.meta.k8s.io \"test\" not found",
		},
		{
			name: "normal case",
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       2,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			resourceType: "gpu",
			existingComposabilityRequest: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Size:       2,
								Model:      "A100 40G",
								TargetNode: "node1",
							},
						},
					},
				},
			},
			count: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			if tc.existingComposabilityRequest != nil {
				for i := range tc.existingComposabilityRequest.Items {
					clientObjects = append(clientObjects, &tc.existingComposabilityRequest.Items[i])
				}
			}

			s := scheme.Scheme
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposabilityRequest{}, &cdioperator.ComposabilityRequestList{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			err := DynamicAttach(context.Background(), fakeClient, tc.updateComposabilityRequest, tc.count, tc.resourceType, tc.model, tc.nodeName)

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
			if tc.updateComposabilityRequest == nil {
				crList := &cdioperator.ComposabilityRequestList{}
				err = fakeClient.List(context.Background(), crList)
				if err != nil {
					t.Fatalf("Failed to list ComposabilityRequests: %v", err)
				}

				if len(crList.Items) != 1 {
					t.Fatalf("Expected 1 ComposabilityRequest, got %d", len(crList.Items))
				}

				if crList.Items[0].Spec.Resource.Model != tc.model {
					t.Errorf("Expected Model %q, got %q", tc.model, crList.Items[0].Spec.Resource.Model)
				}
				if crList.Items[0].Spec.Resource.Size != tc.count {
					t.Errorf("Expected Size %d, got %d", tc.count, crList.Items[0].Spec.Resource.Size)
				}
				if crList.Items[0].Spec.Resource.TargetNode != tc.nodeName {
					t.Errorf("Expected TargetNode %q, got %q", tc.nodeName, crList.Items[0].Spec.Resource.TargetNode)
				}
			} else {
				existingCR := &cdioperator.ComposabilityRequest{}
				err := fakeClient.Get(context.Background(), k8stypes.NamespacedName{Name: tc.updateComposabilityRequest.Name}, existingCR)
				if err != nil {
					t.Errorf("failed to get ComposabilityRequest: %v", err)
				}

				if existingCR.Spec.Resource.Size != tc.count {
					t.Errorf("Expected Size %d, got %d", tc.count, existingCR.Spec.Resource.Size)
				}
			}
		})
	}
}

func TestDynamicDetach(t *testing.T) {
	now := time.Now()
	thirtySecondsAgo := now.Add(-30 * time.Second)

	testCases := []struct {
		name                         string
		existingComposabilityRequest *cdioperator.ComposabilityRequestList
		existingComposableResource   *cdioperator.ComposableResourceList
		updateComposabilityRequest   *cdioperator.ComposabilityRequest
		nodeName                     string
		labelPrefix                  string
		deviceNoRemoval              time.Duration
		count                        int64
		wantErr                      bool
		expectedErrMsg               string
		expectedSize                 int64
	}{
		{
			name:            "nextSize less than composabilityRequest size",
			deviceNoRemoval: time.Minute,
			count:           3,
			nodeName:        "node1",
			labelPrefix:     "composable.test",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "res1",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Second).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{State: "Online"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res2",
							DeletionTimestamp: &metav1.Time{Time: thirtySecondsAgo},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
					},
				},
			},
			existingComposabilityRequest: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Size:       2,
								Model:      "A100 40G",
								TargetNode: "node1",
							},
						},
					},
				},
			},
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       4,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			expectedSize: 3,
		},
		{
			name:            "nextSize greater than composabilityRequest size",
			deviceNoRemoval: time.Minute,
			count:           3,
			nodeName:        "node1",
			labelPrefix:     "composable.test",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "res1",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Second).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{State: "Online"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res2",
							DeletionTimestamp: &metav1.Time{Time: thirtySecondsAgo},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
					},
				},
			},
			existingComposabilityRequest: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Size:       1,
								Model:      "A100 40G",
								TargetNode: "node1",
							},
						},
					},
				},
			},
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       1,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			expectedSize: 1,
		},
		{
			name: "count less than resourceCount",

			deviceNoRemoval: time.Minute,
			count:           1,
			nodeName:        "node1",
			labelPrefix:     "composable.test",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "res1",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Second).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{State: "Online"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "res2",
							Annotations: map[string]string{
								"composable.test/last-used-time": time.Now().Add(-30 * time.Second).Format(time.RFC3339),
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
					},
				},
			},
			existingComposabilityRequest: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Size:       4,
								Model:      "A100 40G",
								TargetNode: "node1",
							},
						},
					},
				},
			},
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       4,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			expectedSize: 2,
		},
		{
			name:            "failed to get next size",
			deviceNoRemoval: time.Minute,
			count:           3,
			nodeName:        "node1",
			labelPrefix:     "composable.test",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "res1",
							Annotations: map[string]string{
								"composable.test/last-used-time": "test",
							},
						},
						Spec: cdioperator.ComposableResourceSpec{
							TargetNode: "node1",
						},
						Status: cdioperator.ComposableResourceStatus{State: "Online"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res2",
							DeletionTimestamp: &metav1.Time{Time: thirtySecondsAgo},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
					},
				},
			},
			existingComposabilityRequest: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Size:       2,
								Model:      "A100 40G",
								TargetNode: "node1",
							},
						},
					},
				},
			},
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       4,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to get next size:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			s := scheme.Scheme
			if tc.existingComposabilityRequest != nil {
				for i := range tc.existingComposabilityRequest.Items {
					clientObjects = append(clientObjects, &tc.existingComposabilityRequest.Items[i])
				}
			}
			if tc.existingComposableResource != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})

				for i := range tc.existingComposableResource.Items {
					clientObjects = append(clientObjects, &tc.existingComposableResource.Items[i])
				}
			}

			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposabilityRequest{}, &cdioperator.ComposabilityRequestList{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			err := DynamicDetach(context.Background(), fakeClient, tc.updateComposabilityRequest, tc.count, tc.nodeName, tc.labelPrefix, tc.deviceNoRemoval)

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

			existingCR := &cdioperator.ComposabilityRequest{}
			err = fakeClient.Get(context.Background(), k8stypes.NamespacedName{Name: tc.updateComposabilityRequest.Name}, existingCR)
			if err != nil {
				t.Errorf("failed to get ComposabilityRequest: %v", err)
			}

			if existingCR.Spec.Resource.Size != tc.expectedSize {
				t.Errorf("Expected Size %d, got %d", tc.expectedSize, existingCR.Spec.Resource.Size)
			}
		})
	}
}

func TestIsDeviceResourceSliceRed(t *testing.T) {
	testCases := []struct {
		name                      string
		deviceID                  string
		resourceSliceInfos        []types.ResourceSliceInfo
		expectedResult            bool
		expectedResourceSliceInfo *types.ResourceSliceInfo
		expectedDeviceName        string
	}{
		{
			name:     "device resourceSlice is red",
			deviceID: "123",
			resourceSliceInfos: []types.ResourceSliceInfo{
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
			expectedResult: true,
			expectedResourceSliceInfo: &types.ResourceSliceInfo{
				Name: "rs0",
				Devices: []types.ResourceSliceDevice{
					{
						Name: "gpu0",
						UUID: "123",
					},
				},
			},
			expectedDeviceName: "gpu0",
		},
		{
			name:     "device resourceSlice not red",
			deviceID: "456",
			resourceSliceInfos: []types.ResourceSliceInfo{
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
			expectedResult:            false,
			expectedResourceSliceInfo: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, resourceSliceInfo, deviceName := IsDeviceResourceSliceRed(tc.deviceID, tc.resourceSliceInfos)
			if result != tc.expectedResult {
				t.Errorf("Expected result %v, got %v", tc.expectedResult, result)
			}

			if !reflect.DeepEqual(resourceSliceInfo, tc.expectedResourceSliceInfo) {
				t.Errorf("Expected resourceSliceInfo %v, got %v", tc.expectedResourceSliceInfo, resourceSliceInfo)
			}

			if deviceName != tc.expectedDeviceName {
				t.Errorf("Expected deviceName %q, got %q", tc.expectedDeviceName, deviceName)
			}
		})
	}
}

func TestIsDeviceUsedByPod(t *testing.T) {
	testCases := []struct {
		name                      string
		deviceName                string
		resourceSliceInfo         types.ResourceSliceInfo
		existingResourceClaimList *resourceapi.ResourceClaimList
		expectedResult            bool
		wantErr                   bool
		expectedErrMsg            string
	}{
		{
			name:       "empty resource claim list",
			deviceName: "gpu0",
			resourceSliceInfo: types.ResourceSliceInfo{
				Name: "rs0",
				Devices: []types.ResourceSliceDevice{
					{
						Name: "gpu0",
						UUID: "123",
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{},
			expectedResult:            false,
			wantErr:                   false,
		},
		{
			name:       "device used by pod",
			deviceName: "gpu0",
			resourceSliceInfo: types.ResourceSliceInfo{
				Name: "rs0",
				Devices: []types.ResourceSliceDevice{
					{
						Name: "gpu0",
						UUID: "123",
					},
				},
				Pool:   "gpu-pool",
				Driver: "nvidia",
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "claim1",
							Namespace: "default",
						},
						Status: resourceapi.ResourceClaimStatus{
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Driver: "nvidia",
											Pool:   "gpu-pool",
											Device: "gpu0",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			wantErr:        false,
		},
		{
			name:       "device not used by pod",
			deviceName: "gpu1",
			resourceSliceInfo: types.ResourceSliceInfo{
				Name: "rs0",
				Devices: []types.ResourceSliceDevice{
					{
						Name: "gpu1",
						UUID: "123",
					},
				},
				Pool:   "gpu-pool",
				Driver: "nvidia",
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "claim1",
							Namespace: "default",
						},
						Status: resourceapi.ResourceClaimStatus{
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Driver: "nvidia",
											Pool:   "gpu-pool",
											Device: "gpu0",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: false,
			wantErr:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			s := scheme.Scheme
			if tc.existingResourceClaimList != nil {
				for i := range tc.existingResourceClaimList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceClaimList.Items[i])
				}
			}
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := IsDeviceUsedByPod(context.Background(), fakeClient, tc.deviceName, tc.resourceSliceInfo)

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

			if result != tc.expectedResult {
				t.Errorf("Expected result %v, got %v", tc.expectedResult, result)
			}
		})
	}
}

func TestGetDriverType(t *testing.T) {
	testcase := []struct {
		name           string
		model          string
		expectedResult string
	}{
		{
			name:           "gpu model",
			model:          "gpu.nvidia.com",
			expectedResult: "gpu",
		},
		{
			name:           "other model",
			model:          "test",
			expectedResult: "",
		},
	}

	for _, tc := range testcase {
		t.Run(tc.name, func(t *testing.T) {
			result := GetDriverType(tc.model)
			if result != tc.expectedResult {
				t.Errorf("Expected result %s, go %s", tc.expectedResult, result)
			}
		})
	}
}
