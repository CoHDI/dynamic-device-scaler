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
	"sort"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/CoHDI/dynamic-device-scaler/internal/types"
	"github.com/stretchr/testify/assert"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetResourceClaimInfo(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name                      string
		existingResourceClaimList *resourceapi.ResourceClaimList
		existingResourceSliceList *resourceapi.ResourceSliceList
		composableDRASpec         types.ComposableDRASpec
		expectedResourceClaimInfo []types.ResourceClaimInfo
		wantErr                   bool
		expectedErrMsg            string
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
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 80GB",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
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
									Driver: "gpu.nvidia.com",
									Device: "gpu-1",
									Pool:   "test-pool",
									Conditions: []metav1.Condition{
										{
											Type:   "FabricDeviceReschedule",
											Status: metav1.ConditionTrue,
										},
									},
								},
								{
									Driver: "gpu.nvidia.com",
									Device: "gpu-2",
									Pool:   "test-pool",
									Conditions: []metav1.Condition{
										{
											Type:   "FabricDeviceFailed",
											Status: metav1.ConditionTrue,
										},
									},
								},
								{
									Driver: "gpu.nvidia.com",
									Device: "gpu-3",
									Pool:   "test-pool",
									Conditions: []metav1.Condition{
										{
											Type:   "test-condition",
											Status: metav1.ConditionTrue,
										},
									},
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device:            "gpu-1",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceReschedule"},
										},
										{
											Device:            "gpu-2",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceFailed"},
										},
										{
											Device:            "gpu-3",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"test"},
										},
									},
								},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchFields: []corev1.NodeSelectorRequirement{
												{
													Key:      "metadata.name",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"node1"},
												},
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
							Name:              "test-resourceslice-1",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Spec: resourceapi.ResourceSliceSpec{
							Driver:   "gpu.nvidia.com",
							NodeName: "node1",
							Devices: []resourceapi.Device{
								{
									Name: "gpu-1",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("1234")},
											"productName": {StringValue: ptr.To("NVIDIA A100 80GB")},
										},
									},
								},
								{
									Name: "gpu-2",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("5678")},
											"productName": {StringValue: ptr.To("NVIDIA A100 80GB")},
										},
									},
								},
							},
							Pool: resourceapi.ResourcePool{
								Name: "test-pool",
							},
						},
					},
				},
			},
			expectedResourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:              "test-claim-1",
					Namespace:         "default",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now.Truncate(time.Second)},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu-1",
							State:             "Reschedule",
							Model:             "A100 80G",
							ResourceSliceName: "test-resourceslice-1",
						},
						{
							Name:              "gpu-2",
							State:             "Failed",
							Model:             "A100 80G",
							ResourceSliceName: "test-resourceslice-1",
						},
						{
							Name:  "gpu-3",
							State: "Preparing",
						},
					},
				},
			},
		},
		{
			name: "resourceClaim with empty reservedFor",
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 80G",
						K8sDeviceName: "nvidia-a100-80g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 80GB",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-claim-1",
							Namespace:         "default",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Status: resourceapi.ResourceClaimStatus{
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{},
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu.nvidia.com",
									Device: "gpu-1",
									Pool:   "test-pool",
									Conditions: []metav1.Condition{
										{
											Type:   "FabricDeviceReschedule",
											Status: metav1.ConditionTrue,
										},
									},
								},
								{
									Driver: "gpu.nvidia.com",
									Device: "gpu-2",
									Pool:   "test-pool",
									Conditions: []metav1.Condition{
										{
											Type:   "FabricDeviceFailed",
											Status: metav1.ConditionTrue,
										},
									},
								},
								{
									Driver: "gpu.nvidia.com",
									Device: "gpu-3",
									Pool:   "test-pool",
									Conditions: []metav1.Condition{
										{
											Type:   "test-condition",
											Status: metav1.ConditionTrue,
										},
									},
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device:            "gpu-1",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceReschedule"},
										},
										{
											Device:            "gpu-2",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceFailed"},
										},
										{
											Device:            "gpu-3",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"test"},
										},
									},
								},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchFields: []corev1.NodeSelectorRequirement{
												{
													Key:      "metadata.name",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"node1"},
												},
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
							Name:              "test-resourceslice-1",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Spec: resourceapi.ResourceSliceSpec{
							Driver:   "gpu.nvidia.com",
							NodeName: "node1",
							Devices: []resourceapi.Device{
								{
									Name: "gpu-1",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("1234")},
											"productName": {StringValue: ptr.To("NVIDIA A100 80GB")},
										},
									},
								},
								{
									Name: "gpu-2",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("5678")},
											"productName": {StringValue: ptr.To("NVIDIA A100 80GB")},
										},
									},
								},
							},
							Pool: resourceapi.ResourcePool{
								Name: "test-pool",
							},
						},
					},
				},
			},
		},
		{
			name: "device without binding conditions",
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 80G",
						K8sDeviceName: "nvidia-a100-80g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 80GB",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
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
									Driver: "gpu.nvidia.com",
									Device: "gpu-0",
									Pool:   "test-pool",
								},
								{
									Driver: "gpu.nvidia.com",
									Device: "gpu-1",
									Pool:   "test-pool",
									Conditions: []metav1.Condition{
										{
											Type:   "FabricDeviceReschedule",
											Status: metav1.ConditionTrue,
										},
									},
								},
								{
									Driver: "gpu.nvidia.com",
									Device: "gpu-2",
									Pool:   "test-pool",
									Conditions: []metav1.Condition{
										{
											Type:   "FabricDeviceFailed",
											Status: metav1.ConditionTrue,
										},
									},
								},
								{
									Driver: "gpu.nvidia.com",
									Device: "gpu-3",
									Pool:   "test-pool",
									Conditions: []metav1.Condition{
										{
											Type:   "test-condition",
											Status: metav1.ConditionTrue,
										},
									},
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "gpu-1",
											Driver: "gpu.nvidia.com",
											Pool:   "test-pool",
										},
										{
											Device: "gpu-2",
											Driver: "gpu.nvidia.com",
											Pool:   "test-pool",
										},
										{
											Device: "gpu-3",
											Driver: "gpu.nvidia.com",
											Pool:   "test-pool",
										},
									},
								},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchFields: []corev1.NodeSelectorRequirement{
												{
													Key:      "metadata.name",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"node1"},
												},
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
							Name:              "test-resourceslice-1",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Spec: resourceapi.ResourceSliceSpec{
							Driver:   "gpu.nvidia.com",
							NodeName: "node1",
							Devices: []resourceapi.Device{
								{
									Name: "gpu-1",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("1234")},
											"productName": {StringValue: ptr.To("NVIDIA A100 80GB")},
										},
									},
								},
								{
									Name: "gpu-2",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("5678")},
											"productName": {StringValue: ptr.To("NVIDIA A100 80GB")},
										},
									},
								},
							},
							Pool: resourceapi.ResourcePool{
								Name: "test-pool",
							},
						},
					},
				},
			},
		},
		{
			name: "resourceClaim with empty Status.Devices",
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 80G",
						K8sDeviceName: "nvidia-a100-80g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 80GB",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
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
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device:            "gpu-1",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceReschedule"},
										},
										{
											Device:            "gpu-2",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceFailed"},
										},
										{
											Device:            "gpu-3",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"test"},
										},
									},
								},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchFields: []corev1.NodeSelectorRequirement{
												{
													Key:      "metadata.name",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"node1"},
												},
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
							Name:              "test-resourceslice-1",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Spec: resourceapi.ResourceSliceSpec{
							Driver:   "gpu.nvidia.com",
							NodeName: "node1",
							Devices: []resourceapi.Device{
								{
									Name: "gpu-1",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("1234")},
											"productName": {StringValue: ptr.To("NVIDIA A100 80GB")},
										},
									},
								},
								{
									Name: "gpu-2",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("5678")},
											"productName": {StringValue: ptr.To("NVIDIA A100 80GB")},
										},
									},
								},
							},
							Pool: resourceapi.ResourcePool{
								Name: "test-pool",
							},
						},
					},
				},
			},
			expectedResourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:              "test-claim-1",
					Namespace:         "default",
					NodeName:          "node1",
					CreationTimestamp: metav1.Time{Time: now.Truncate(time.Second)},
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu-1",
							State:             "Preparing",
							Model:             "A100 80G",
							ResourceSliceName: "test-resourceslice-1",
						},
						{
							Name:              "gpu-2",
							State:             "Preparing",
							Model:             "A100 80G",
							ResourceSliceName: "test-resourceslice-1",
						},
						{
							Name:  "gpu-3",
							State: "Preparing",
						},
					},
				},
			},
		},
		{
			name: "resourceSlice with error model",
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 80G",
						K8sDeviceName: "nvidia-a100-80g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 80GB",
						},
					},
				},
			},
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
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
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device:            "gpu-1",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceReschedule"},
										},
										{
											Device:            "gpu-2",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceFailed"},
										},
										{
											Device:            "gpu-3",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"test"},
										},
									},
								},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchFields: []corev1.NodeSelectorRequirement{
												{
													Key:      "metadata.name",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"node1"},
												},
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
							Name:              "test-resourceslice-1",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Spec: resourceapi.ResourceSliceSpec{
							Driver:   "gpu.nvidia.com",
							NodeName: "node1",
							Devices: []resourceapi.Device{
								{
									Name: "gpu-1",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("1234")},
											"productName": {StringValue: ptr.To("NVIDIA Test")},
										},
									},
								},
								{
									Name: "gpu-2",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("5678")},
											"productName": {StringValue: ptr.To("NVIDIA Test")},
										},
									},
								},
							},
							Pool: resourceapi.ResourcePool{
								Name: "test-pool",
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "unknown device name:",
		},
		{
			name: "failed to list ResourceClaims",
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
									Name: "gpu-1",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("1234")},
											"productName": {StringValue: ptr.To("NVIDIA Test")},
										},
									},
								},
								{
									Name: "gpu-2",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid":        {StringValue: ptr.To("5678")},
											"productName": {StringValue: ptr.To("NVIDIA Test")},
										},
									},
								},
							},
							Pool: resourceapi.ResourcePool{
								Name: "test-pool",
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to list ResourceClaims:",
		},
		{
			name: "failed to list ResourceSlices",
			existingResourceClaimList: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
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
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device:            "gpu-1",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceReschedule"},
										},
										{
											Device:            "gpu-2",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"FabricDeviceFailed"},
										},
										{
											Device:            "gpu-3",
											Driver:            "gpu.nvidia.com",
											Pool:              "test-pool",
											BindingConditions: []string{"test"},
										},
									},
								},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchFields: []corev1.NodeSelectorRequirement{
												{
													Key:      "metadata.name",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"node1"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to list ResourceSlices:",
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

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := GetResourceClaimInfo(context.Background(), fakeClient, tc.composableDRASpec)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			assert.ElementsMatch(t, result, tc.expectedResourceClaimInfo)
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
			name:           "failed to list ResourceSlices",
			wantErr:        true,
			expectedErrMsg: "failed to list ResourceSlices:",
		},
		{
			name: "resourceSlice with binding conditions",
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
										BindingConditions: []string{"test"},
									},
								},
								{
									Name: "gpu-1",
									Basic: &resourceapi.BasicDevice{
										Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
											"uuid": {StringValue: ptr.To("5678")},
										},
										BindingConditions: []string{"test"},
									},
								},
							},
						},
					},
				},
			},
		},
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
			s := runtime.NewScheme()

			if tc.existingResourceSliceList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &resourceapi.ResourceSlice{}, &resourceapi.ResourceSliceList{})
				for i := range tc.existingResourceSliceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceSliceList.Items[i])
				}
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := GetResourceSliceInfo(context.Background(), fakeClient)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			assert.ElementsMatch(t, result, tc.expectedResourceSliceInfo)
		})
	}
}

func sortNodeInfos(nis []types.NodeInfo) {
	sort.Slice(nis, func(i, j int) bool {
		return nis[i].Name < nis[j].Name
	})

	for idx := range nis {
		sort.Slice(nis[idx].Models, func(i, j int) bool {
			a := nis[idx].Models[i]
			b := nis[idx].Models[j]
			if a.Model != b.Model {
				return a.Model < b.Model
			}
			return a.DeviceName < b.DeviceName
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
					Name: "node1",
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
			name: "node label with invalid integer",
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
			expectedErrMsg: "invalid integer in ss: strconv.Atoi:",
		},
		{
			name: "node without expected prefix label",
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
								"test/nvidia-a100-80g":          "true",
								"test/fabric":                   "123",
								"test/nvidia-a100-80g-size-min": "2",
								"test/nvidia-a100-80g-size-max": "6",
							},
						},
					},
				},
			},
			expectedNodeInfos: []types.NodeInfo{
				{
					Name: "node1",
				},
			},
		},
		{
			name: "node with error max info label",
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
								"composable.fsastech.com/nvidia-a100-80g-size-min": "0",
								"composable.fsastech.com/nvidia-a100-80g-size-max": "ss",
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "invalid integer in ss: strconv.Atoi: ",
		},
		{
			name: "node with error model info label",
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
								"composable.fsastech.com/nvidia-a100-80g-size-min": "0",
								"composable.fsastech.com/nvidia-a100-40g-size-max": "5",
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "unknown device name:",
		},
		{
			name: "multiple sets of label",
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 80G",
						K8sDeviceName: "nvidia-a100-80g",
					},
					{
						Index:         2,
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
								"composable.fsastech.com/nvidia-a100-40g":          "true",
								"composable.fsastech.com/fabric":                   "123",
								"composable.fsastech.com/nvidia-a100-80g-size-min": "2",
								"composable.fsastech.com/nvidia-a100-80g-size-max": "6",
								"composable.fsastech.com/nvidia-a100-40g-size-max": "5",
								"composable.fsastech.com/nvidia-a100-40g-size-min": "1",
							},
						},
					},
				},
			},
			expectedNodeInfos: []types.NodeInfo{
				{
					Name: "node1",
					Models: []types.ModelConstraints{
						{
							Model:      "A100 80G",
							DeviceName: "nvidia-a100-80g",
							MinDevice:  2,
							MaxDevice:  6,
						},
						{
							Model:      "A100 40G",
							DeviceName: "nvidia-a100-40g",
							MinDevice:  1,
							MaxDevice:  5,
						},
					},
				},
			},
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
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			sortNodeInfos(result)
			sortNodeInfos(tc.expectedNodeInfos)

			if !reflect.DeepEqual(result, tc.expectedNodeInfos) {
				t.Errorf("Expected node infos %v, got %v", tc.expectedNodeInfos, result)
			}
		})
	}
}

func TestGetModelName(t *testing.T) {
	tests := []struct {
		name              string
		deviceName        string
		productName       string
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
		{
			name:        "normal product name",
			productName: "NVIDIA A100 40GB PCIe",
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB PCIe",
						},
					},
				},
			},
			expectedResult: "A100 40G",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := getModelName(tc.composableDRASpec, tc.deviceName, tc.productName)
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
    productName: "NVIDIA A100 40GB PCIe"
  label-key-model: "composable-a100-40G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-40"
  cannot-coexist-with: [2, 3, 4]
            `,
				"label-prefix":    "composable.fsastech.com",
				"fabric-id-range": "[1, 2, 3]",
			},
			createConfigMap: true,
			wantSpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:        1,
						CDIModelName: "A100 40G",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB PCIe",
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
    productName: "NVIDIA A100 40GB PCIe"
  label-key-model: "composable-a100-40G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-40"
  cannot-coexist-with: [2, 3, 4]
            `,
				"label-prefix":    "composable.fsastech.com",
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

func TestHasMatchingBindingCondition(t *testing.T) {
	trueConditionA := metav1.Condition{Type: "TypeA", Status: metav1.ConditionTrue}
	falseConditionA := metav1.Condition{Type: "TypeA", Status: metav1.ConditionFalse}
	trueConditionB := metav1.Condition{Type: "TypeB", Status: metav1.ConditionTrue}
	trueConditionC := metav1.Condition{Type: "TypeC", Status: metav1.ConditionTrue}

	tests := []struct {
		name           string
		conditions     []metav1.Condition
		binding        []string
		bindingFailure []string
		expected       bool
	}{
		{
			name:           "Match in bindingConditions",
			conditions:     []metav1.Condition{trueConditionA},
			binding:        []string{"TypeA"},
			bindingFailure: []string{},
			expected:       true,
		},
		{
			name:           "Match in bindingFailureConditions",
			conditions:     []metav1.Condition{trueConditionB},
			binding:        []string{},
			bindingFailure: []string{"TypeB"},
			expected:       true,
		},
		{
			name:           "Match in both lists",
			conditions:     []metav1.Condition{trueConditionA},
			binding:        []string{"TypeA"},
			bindingFailure: []string{"TypeA"},
			expected:       true,
		},
		{
			name:           "Condition exists but wrong status",
			conditions:     []metav1.Condition{falseConditionA},
			binding:        []string{"TypeA"},
			bindingFailure: []string{},
			expected:       false,
		},
		{
			name:           "No matching condition type",
			conditions:     []metav1.Condition{trueConditionC},
			binding:        []string{"TypeA"},
			bindingFailure: []string{"TypeB"},
			expected:       false,
		},
		{
			name:           "Empty conditions list",
			conditions:     []metav1.Condition{},
			binding:        []string{"TypeA"},
			bindingFailure: []string{"TypeB"},
			expected:       false,
		},
		{
			name:           "No binding conditions specified",
			conditions:     []metav1.Condition{trueConditionA},
			binding:        []string{},
			bindingFailure: []string{},
			expected:       false,
		},
		{
			name:           "Multiple conditions with match",
			conditions:     []metav1.Condition{falseConditionA, trueConditionB, trueConditionC},
			binding:        []string{"TypeB"},
			bindingFailure: []string{"TypeD"},
			expected:       true,
		},
		{
			name:           "Multiple conditions without match",
			conditions:     []metav1.Condition{falseConditionA, trueConditionC},
			binding:        []string{"TypeA"},
			bindingFailure: []string{"TypeB"},
			expected:       false,
		},
		{
			name:           "Match in bindingFailure with multiple conditions",
			conditions:     []metav1.Condition{trueConditionA, falseConditionA, trueConditionB},
			binding:        []string{"TypeC"},
			bindingFailure: []string{"TypeB"},
			expected:       true,
		},
		{
			name:           "Nil conditions slice",
			conditions:     nil,
			binding:        []string{"TypeA"},
			bindingFailure: []string{"TypeB"},
			expected:       false,
		},
		{
			name:           "Nil binding lists",
			conditions:     []metav1.Condition{trueConditionA},
			binding:        nil,
			bindingFailure: nil,
			expected:       false,
		},
		{
			name:           "Condition in bindingFailure but status false",
			conditions:     []metav1.Condition{falseConditionA},
			binding:        nil,
			bindingFailure: []string{"TypeA"},
			expected:       false,
		},
		{
			name:           "Match with duplicate in binding lists",
			conditions:     []metav1.Condition{trueConditionA},
			binding:        []string{"TypeA", "TypeA"},
			bindingFailure: []string{"TypeB", "TypeA"},
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasMatchingBindingCondition(
				tt.conditions,
				tt.binding,
				tt.bindingFailure,
			)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v for test case: %s", tt.expected, result, tt.name)
			}
		})
	}
}

func TestGetNodeName(t *testing.T) {
	tests := []struct {
		name     string
		selector corev1.NodeSelector
		expected string
	}{
		{
			name: "Single node",
			selector: corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchFields: []corev1.NodeSelectorRequirement{
							{
								Key:      "metadata.name",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"node1"},
							},
						},
					},
				},
			},
			expected: "node1",
		},
		{
			name:     "No nodes",
			selector: corev1.NodeSelector{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNodeName(tt.selector)

			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
