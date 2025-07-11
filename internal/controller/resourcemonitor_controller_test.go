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

package controller

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/CoHDI/dynamic-device-scaler/internal/types"
	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"
)

func TestCollectInfo(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name                      string
		existingResourceClaimList *resourceapi.ResourceClaimList
		existingResourceSliceList *resourceapi.ResourceSliceList
		existingNode              *corev1.NodeList
		configMapData             map[string]string
		expectedResourceClaimInfo []types.ResourceClaimInfo
		expectedResourceSliceInfo []types.ResourceSliceInfo
		expectedNodeInfo          []types.NodeInfo
		expectedConfigMapInfo     types.ComposableDRASpec
		wantErr                   bool
		expectedErrMsg            string
	}{
		{
			name: "normal case",
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
							NodeName: ptr.To("node1"),
							Devices: []resourceapi.Device{
								{
									Name: "gpu-1",
									Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
										"uuid":        {StringValue: ptr.To("1234")},
										"productName": {StringValue: ptr.To("NVIDIA A100 80GB PCIe")},
									},
								},
								{
									Name: "gpu-2",
									Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
										"uuid":        {StringValue: ptr.To("5678")},
										"productName": {StringValue: ptr.To("NVIDIA A100 80GB PCIe")},
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
			configMapData: map[string]string{
				"device-info": `
- index: 1
  cdi-model-name: "A100 80G"
  dra-attributes:
    productName: "NVIDIA A100 80GB PCIe"
  label-key-model: "composable-a100-80G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-80g"
  cannot-coexist-with: [2, 3, 4]
            `,
				"label-prefix":    "composable.fsastech.com",
				"fabric-id-range": "[1, 2, 3]",
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
			expectedResourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:              "test-resourceslice-1",
					CreationTimestamp: metav1.Time{Time: now.Truncate(time.Second)},
					Driver:            "gpu.nvidia.com",
					NodeName:          "node1",
					Pool:              "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu-1",
							UUID: "1234",
						},
						{
							Name: "gpu-2",
							UUID: "5678",
						},
					},
				},
			},
			expectedNodeInfo: []types.NodeInfo{
				{
					Name: "node1",
					Models: []types.ModelConstraints{
						{
							Model:        "A100 80G",
							DeviceName:   "nvidia-a100-80g",
							MinDevice:    2,
							MaxDevice:    6,
							MaxDeviceSet: true,
						},
					},
				},
			},
			expectedConfigMapInfo: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:        1,
						CDIModelName: "A100 80G",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 80GB PCIe",
						},
						LabelKeyModel:     "composable-a100-80G",
						DriverName:        "gpu.nvidia.com",
						K8sDeviceName:     "nvidia-a100-80g",
						CannotCoexistWith: []int{2, 3, 4},
					},
				},
				FabricIDRange: []int{1, 2, 3},
			},
		},
		{
			name: "failed to get configMap info",
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
							NodeName: ptr.To("node1"),
							Devices: []resourceapi.Device{
								{
									Name: "gpu-1",
									Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
										"uuid":        {StringValue: ptr.To("1234")},
										"productName": {StringValue: ptr.To("NVIDIA A100 80GB PCIe")},
									},
								},
								{
									Name: "gpu-2",
									Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
										"uuid":        {StringValue: ptr.To("5678")},
										"productName": {StringValue: ptr.To("NVIDIA A100 80GB PCIe")},
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
			configMapData: map[string]string{
				"device-info":     "invalid yaml",
				"label-prefix":    "composable.fsastech.com",
				"fabric-id-range": "[1, 2, 3]",
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
			expectedErrMsg: "failed to get ComposableDRASpec from ConfigMap:",
		},
		{
			name: "failed to get resourceClaim info",
			existingResourceSliceList: &resourceapi.ResourceSliceList{
				Items: []resourceapi.ResourceSlice{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-resourceslice-1",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Spec: resourceapi.ResourceSliceSpec{
							Driver:   "gpu.nvidia.com",
							NodeName: ptr.To("node1"),
							Devices: []resourceapi.Device{
								{
									Name: "gpu-1",
									Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
										"uuid":        {StringValue: ptr.To("1234")},
										"productName": {StringValue: ptr.To("NVIDIA A100 80GB PCIe")},
									},
								},
								{
									Name: "gpu-2",
									Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
										"uuid":        {StringValue: ptr.To("5678")},
										"productName": {StringValue: ptr.To("NVIDIA A100 80GB PCIe")},
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
			configMapData: map[string]string{
				"device-info": `
- index: 1
  cdi-model-name: "A100 80G"
  dra-attributes:
    productName: "NVIDIA A100 80GB PCIe"
  label-key-model: "composable-a100-80G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-80g"
  cannot-coexist-with: [2, 3, 4]
            `,
				"label-prefix":    "composable.fsastech.com",
				"fabric-id-range": "[1, 2, 3]",
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
			expectedConfigMapInfo: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:        1,
						CDIModelName: "A100 80G",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 80GB PCIe",
						},
						LabelKeyModel:     "composable-a100-80G",
						DriverName:        "gpu.nvidia.com",
						K8sDeviceName:     "nvidia-a100-80g",
						CannotCoexistWith: []int{2, 3, 4},
					},
				},
				FabricIDRange: []int{1, 2, 3},
			},
			wantErr:        true,
			expectedErrMsg: "failed to get ResourceClaimInfo:",
		},
		{
			name: "failed to get node info",
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
							NodeName: ptr.To("node1"),
							Devices: []resourceapi.Device{
								{
									Name: "gpu-1",
									Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
										"uuid":        {StringValue: ptr.To("1234")},
										"productName": {StringValue: ptr.To("NVIDIA A100 80GB PCIe")},
									},
								},
								{
									Name: "gpu-2",
									Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
										"uuid":        {StringValue: ptr.To("5678")},
										"productName": {StringValue: ptr.To("NVIDIA A100 80GB PCIe")},
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
			configMapData: map[string]string{
				"device-info": `
- index: 1
  cdi-model-name: "A100 80G"
  dra-attributes:
    productName: "NVIDIA A100 80GB PCIe"
  label-key-model: "composable-a100-80G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-80g"
  cannot-coexist-with: [2, 3, 4]
            `,
				"label-prefix":    "composable.fsastech.com",
				"fabric-id-range": "[1, 2, 3]",
			},
			existingNode: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Labels: map[string]string{
								"composable.fsastech.com/nvidia-a100-80g":          "true",
								"composable.fsastech.com/fabric":                   "123",
								"composable.fsastech.com/nvidia-a100-80g-size-min": "test",
								"composable.fsastech.com/nvidia-a100-80g-size-max": "6",
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
			expectedResourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:              "test-resourceslice-1",
					CreationTimestamp: metav1.Time{Time: now.Truncate(time.Second)},
					Driver:            "gpu.nvidia.com",
					NodeName:          "node1",
					Pool:              "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu-1",
							UUID: "1234",
						},
						{
							Name: "gpu-2",
							UUID: "5678",
						},
					},
				},
			},
			expectedConfigMapInfo: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:        1,
						CDIModelName: "A100 80G",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 80GB PCIe",
						},
						LabelKeyModel:     "composable-a100-80G",
						DriverName:        "gpu.nvidia.com",
						K8sDeviceName:     "nvidia-a100-80g",
						CannotCoexistWith: []int{2, 3, 4},
					},
				},
				FabricIDRange: []int{1, 2, 3},
			},
			wantErr:        true,
			expectedErrMsg: "failed to get NodeInfo:",
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

			kubeObjects := []runtime.Object{}
			if tc.existingNode != nil {
				kubeObjects = append(kubeObjects, tc.existingNode)
			}

			allObjects := append(kubeObjects, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "composable-dra-dds",
					Namespace: "composable-dra",
				},
				Data: tc.configMapData,
			})

			clientSet := k8sfake.NewSimpleClientset(allObjects...)

			resourceController := &ResourceMonitorReconciler{
				Client:    fakeClient,
				ClientSet: clientSet,
			}

			resourceClaimInfo, resourceSliceInfo, nodeInfo, configMapInfo, err := resourceController.collectInfo(context.Background())

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

			assert.ElementsMatch(t, resourceClaimInfo, tc.expectedResourceClaimInfo)
			assert.ElementsMatch(t, resourceSliceInfo, tc.expectedResourceSliceInfo)

			sortNodeInfos(nodeInfo)
			sortNodeInfos(tc.expectedNodeInfo)

			if !reflect.DeepEqual(nodeInfo, tc.expectedNodeInfo) {
				t.Errorf("Expected node infos %v, got %v", tc.expectedNodeInfo, nodeInfo)
			}
			if !reflect.DeepEqual(configMapInfo, tc.expectedConfigMapInfo) {
				t.Errorf("got %+v, want %+v", configMapInfo, tc.expectedConfigMapInfo)
			}
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

func TestUpdateComposableResourceLastUsedTime(t *testing.T) {
	testCases := []struct {
		name                  string
		existingResourceList  *cdioperator.ComposableResourceList
		existingResourceClaim *resourceapi.ResourceClaimList
		resourceSliceInfoList []types.ResourceSliceInfo
		labelPrefix           string
		wantErr               bool
		expectedErrMsg        string
		expectedUpdate        bool
	}{
		{
			name:        "failed to list ComposableResourceList",
			labelPrefix: "test",
			existingResourceClaim: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rc0",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu.nvidia.com",
									Pool:   "gpu-pool",
									Device: "gpu0",
								},
							},
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Name:     "pod0",
									Resource: "pods",
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "gpu0",
											Pool:   "gpu-pool",
											Driver: "gpu.nvidia.com",
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to list ComposableResourceList:",
		},
		{
			name:        "failed to check if device is used by pod",
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
					Pool:   "gpu-pool",
					Driver: "gpu.nvidia.com",
				},
			},
			expectedUpdate: false,
			wantErr:        true,
			expectedErrMsg: "failed to check if device is used by pod:",
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
			existingResourceClaim: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rc0",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu.nvidia.com",
									Pool:   "gpu-pool",
									Device: "gpu0",
								},
							},
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Name:     "pod0",
									Resource: "pods",
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "gpu0",
											Pool:   "gpu-pool",
											Driver: "gpu.nvidia.com",
										},
									},
								},
							},
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
			existingResourceClaim: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rc0",
							Namespace: "test",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu.nvidia.com",
									Pool:   "gpu-pool",
									Device: "gpu0",
								},
							},
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Name:     "pod0",
									Resource: "pods",
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "gpu0",
											Pool:   "gpu-pool",
											Driver: "gpu.nvidia.com",
										},
									},
								},
							},
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
					Pool:   "gpu-pool",
					Driver: "gpu.nvidia.com",
				},
			},
			expectedUpdate: false,
			wantErr:        true,
			expectedErrMsg: "failed to update ComposableResource:",
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
			existingResourceClaim: &resourceapi.ResourceClaimList{
				Items: []resourceapi.ResourceClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rc0",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{
									Driver: "gpu.nvidia.com",
									Pool:   "gpu-pool",
									Device: "gpu0",
								},
							},
							ReservedFor: []resourceapi.ResourceClaimConsumerReference{
								{
									Name:     "pod0",
									Resource: "pods",
								},
							},
							Allocation: &resourceapi.AllocationResult{
								Devices: resourceapi.DeviceAllocationResult{
									Results: []resourceapi.DeviceRequestAllocationResult{
										{
											Device: "gpu0",
											Pool:   "gpu-pool",
											Driver: "gpu.nvidia.com",
										},
									},
								},
							},
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
					Pool:   "gpu-pool",
					Driver: "gpu.nvidia.com",
				},
			},
			expectedUpdate: true,
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

			if tc.existingResourceClaim != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &resourceapi.ResourceClaim{}, &resourceapi.ResourceClaimList{})
				for i := range tc.existingResourceClaim.Items {
					clientObjects = append(clientObjects, &tc.existingResourceClaim.Items[i])
				}
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			resourceController := &ResourceMonitorReconciler{
				Client: fakeClient,
			}

			err := resourceController.updateComposableResourceLastUsedTime(context.Background(), tc.resourceSliceInfoList, tc.labelPrefix)
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

			resource := &cdioperator.ComposableResource{}
			err = fakeClient.Get(context.Background(), client.ObjectKey{
				Name: "rs0",
			}, resource)
			if err != nil {
				t.Errorf("failed to get resource: %v", err)
			}

			if tc.expectedUpdate {
				assert.Contains(t, resource.Annotations, tc.labelPrefix+"/last-used-time")
				_, err := time.Parse(time.RFC3339, resource.Annotations[tc.labelPrefix+"/last-used-time"])
				assert.NoError(t, err)
			} else {
				assert.Nil(t, resource.Annotations)
			}

		})
	}
}

func TestHandleDevices(t *testing.T) {
	tests := []struct {
		name                 string
		nodeInfo             types.NodeInfo
		resourceClaimInfo    []types.ResourceClaimInfo
		resourceSliceInfo    []types.ResourceSliceInfo
		composableDRASpec    types.ComposableDRASpec
		existingResourceList *cdioperator.ComposableResourceList
		existingRequestList  *cdioperator.ComposabilityRequestList
		wantErr              bool
		expectedErrMsg       string
		expectedRequestSize  int
	}{
		{
			name: "failed to list ComposabilityRequestList",
			nodeInfo: types.NodeInfo{
				Name: "node1",
			},
			resourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:      "test-claim-1",
					Namespace: "default",
					NodeName:  "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu0",
							State:             "Preparing",
							Model:             "A100 80G",
							ResourceSliceName: "test-resourceslice-1",
						},
					},
				},
			},
			resourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:     "test-resourceslice-1",
					NodeName: "node1",
					Driver:   "gpu.nvidia.com",
					Pool:     "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "1234",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB",
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to list ComposabilityRequestList:",
		},
		{
			name: "failed to get configured device count",
			nodeInfo: types.NodeInfo{
				Name: "node1",
			},
			resourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:      "test-claim-1",
					Namespace: "default",
					NodeName:  "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu0",
							State:             "Preparing",
							Model:             "A100 80G",
							ResourceSliceName: "test-resourceslice-1",
						},
					},
				},
			},
			resourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:     "test-resourceslice-1",
					NodeName: "node1",
					Driver:   "gpu.nvidia.com",
					Pool:     "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "1234",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-request-1",
							Namespace: "default",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:  "gpu",
								Model: "A100 40G",
								Size:  2,
							},
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to get configured device count:",
		},
		{
			name: "attach devices",
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:      "A100 40G",
						DeviceName: "nvidia-a100-40g",
						MaxDevice:  6,
						MinDevice:  3,
					},
				},
			},
			resourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:      "test-claim-1",
					Namespace: "default",
					NodeName:  "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu0",
							State:             "Preparing",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-1",
						},
						{
							Name:              "gpu1",
							State:             "Preparing",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-1",
						},
					},
				},
			},
			resourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:     "test-resourceslice-1",
					NodeName: "node1",
					Driver:   "gpu.nvidia.com",
					Pool:     "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "1234",
						},
						{
							Name: "gpu1",
							UUID: "5678",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-request-1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Model:      "A100 40G",
								TargetNode: "node1",
								Size:       1,
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{},
			expectedRequestSize:  3,
		},
		{
			name: "detach devices",
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:      "A100 40G",
						DeviceName: "nvidia-a100-40g",
						MaxDevice:  6,
						MinDevice:  0,
					},
				},
			},
			resourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:      "test-claim-1",
					Namespace: "default",
					NodeName:  "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu0",
							State:             "Preparing",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-1",
						},
						{
							Name:              "gpu1",
							State:             "Preparing",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-1",
						},
					},
				},
			},
			resourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:     "test-resourceslice-1",
					NodeName: "node1",
					Driver:   "gpu.nvidia.com",
					Pool:     "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "1234",
						},
						{
							Name: "gpu1",
							UUID: "5678",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-request-1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Model:      "A100 40G",
								TargetNode: "node1",
								Size:       5,
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{},
			expectedRequestSize:  2,
		},
		{
			name: "create request",
			nodeInfo: types.NodeInfo{
				Name: "node1",
				Models: []types.ModelConstraints{
					{
						Model:      "A100 40G",
						DeviceName: "nvidia-a100-40g",
						MaxDevice:  6,
						MinDevice:  0,
					},
				},
			},
			resourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:      "test-claim-1",
					Namespace: "default",
					NodeName:  "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu0",
							State:             "Preparing",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-1",
						},
					},
				},
			},
			resourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:     "test-resourceslice-1",
					NodeName: "node1",
					Driver:   "gpu.nvidia.com",
					Pool:     "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "1234",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB",
						},
					},
				},
			},
			existingRequestList:  &cdioperator.ComposabilityRequestList{},
			existingResourceList: &cdioperator.ComposableResourceList{},
			expectedRequestSize:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

			resourceController := &ResourceMonitorReconciler{
				Client: fakeClient,
			}

			err := resourceController.handleDevices(context.Background(), tc.nodeInfo, tc.resourceClaimInfo, tc.resourceSliceInfo, tc.composableDRASpec)
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

			crList := &cdioperator.ComposabilityRequestList{}
			err = fakeClient.List(context.Background(), crList)
			if err != nil {
				t.Fatalf("Failed to list ComposabilityRequests: %v", err)
			}

			if len(crList.Items) != 1 {
				t.Fatalf("Expected 1 ComposabilityRequest, got %d", len(crList.Items))
			}

			if crList.Items[0].Spec.Resource.Size != int64(tc.expectedRequestSize) {
				t.Errorf("Expected Size %d, got %d", tc.expectedRequestSize, crList.Items[0].Spec.Resource.Size)
			}
		})
	}
}

func TestHandleNodes(t *testing.T) {
	tests := []struct {
		name                      string
		nodeInfo                  []types.NodeInfo
		resourceClaimInfo         []types.ResourceClaimInfo
		resourceSliceInfo         []types.ResourceSliceInfo
		composableDRASpec         types.ComposableDRASpec
		existingResourceList      *cdioperator.ComposableResourceList
		existingRequestList       *cdioperator.ComposabilityRequestList
		existingResourceClaimList *resourceapi.ResourceClaimList
		existingResourceSliceList *resourceapi.ResourceSliceList
		existingNode              *corev1.Node
		wantErr                   bool
		expectedErrMsg            string
		expectedRequestSize       int
	}{
		{
			name: "failed to reschedule failed notification",
			nodeInfo: []types.NodeInfo{
				{
					Name: "node1",
					Models: []types.ModelConstraints{
						{
							Model:      "A100 40G",
							DeviceName: "nvidia-a100-40g",
							MaxDevice:  6,
							MinDevice:  0,
						},
					},
				},
			},
			resourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:      "test-claim-1",
					Namespace: "default",
					NodeName:  "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu0",
							State:             "Failed",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-1",
						},
					},
				},
				{
					Name:      "test-claim-2",
					Namespace: "default",
					NodeName:  "node2",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu1",
							State:             "Failed",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-2",
						},
					},
				},
			},
			resourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:     "test-resourceslice-1",
					NodeName: "node1",
					Driver:   "gpu.nvidia.com",
					Pool:     "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "1234",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB",
						},
					},
				},
			},
			wantErr:        true,
			expectedErrMsg: "failed to reschedule failed notification:",
		},
		{
			name: "failed to update node label",
			nodeInfo: []types.NodeInfo{
				{
					Name: "node1",
					Models: []types.ModelConstraints{
						{
							Model:      "A100 40G",
							DeviceName: "nvidia-a100-40g",
							MaxDevice:  6,
							MinDevice:  0,
						},
					},
				},
			},
			resourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:      "test-claim-1",
					Namespace: "default",
					NodeName:  "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu0",
							State:             "Failed",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-1",
						},
					},
				},
				{
					Name:      "test-claim-2",
					Namespace: "default",
					NodeName:  "node2",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu1",
							State:             "Preparing",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-2",
						},
					},
				},
			},
			resourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:     "test-resourceslice-1",
					NodeName: "node1",
					Driver:   "gpu.nvidia.com",
					Pool:     "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "1234",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-request-1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Model:      "A100 40G",
								TargetNode: "node1",
								Size:       5,
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{},
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
			wantErr:        true,
			expectedErrMsg: "failed to update node label:",
		},
		{
			name: "normal case",
			nodeInfo: []types.NodeInfo{
				{
					Name: "node1",
					Models: []types.ModelConstraints{
						{
							Model:      "A100 40G",
							DeviceName: "nvidia-a100-40g",
							MaxDevice:  6,
							MinDevice:  0,
						},
					},
				},
			},
			resourceClaimInfo: []types.ResourceClaimInfo{
				{
					Name:      "test-claim-1",
					Namespace: "default",
					NodeName:  "node1",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu0",
							State:             "Failed",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-1",
						},
					},
				},
				{
					Name:      "test-claim-2",
					Namespace: "default",
					NodeName:  "node2",
					Devices: []types.ResourceClaimDevice{
						{
							Name:              "gpu1",
							State:             "Preparing",
							Model:             "A100 40G",
							ResourceSliceName: "test-resourceslice-2",
						},
					},
				},
			},
			resourceSliceInfo: []types.ResourceSliceInfo{
				{
					Name:     "test-resourceslice-1",
					NodeName: "node1",
					Driver:   "gpu.nvidia.com",
					Pool:     "test-pool",
					Devices: []types.ResourceSliceDevice{
						{
							Name: "gpu0",
							UUID: "1234",
						},
					},
				},
			},
			composableDRASpec: types.ComposableDRASpec{
				LabelPrefix: "composable.fsastech.com",
				DeviceInfos: []types.DeviceInfo{
					{
						Index:         1,
						CDIModelName:  "A100 40G",
						K8sDeviceName: "nvidia-a100-40g",
						DRAAttributes: map[string]string{
							"productName": "NVIDIA A100 40GB",
						},
					},
				},
			},
			existingRequestList: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-request-1",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Model:      "A100 40G",
								TargetNode: "node1",
								Size:       5,
							},
						},
					},
				},
			},
			existingResourceList: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{},
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
			existingNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						"composable.fsastech.com/nvidia-a100-80g": "true",
					},
				},
			},
		},
	}

	for _, tc := range tests {
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

			if tc.existingResourceSliceList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &resourceapi.ResourceSlice{}, &resourceapi.ResourceSliceList{})
				for i := range tc.existingResourceSliceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceSliceList.Items[i])
				}
			}

			if tc.existingResourceClaimList != nil {
				s.AddKnownTypes(metav1.SchemeGroupVersion, &resourceapi.ResourceClaim{}, &resourceapi.ResourceClaimList{})
				for i := range tc.existingResourceClaimList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceClaimList.Items[i])
				}
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			resourceController := &ResourceMonitorReconciler{
				Client:    fakeClient,
				ClientSet: kubeClient,
			}

			err := resourceController.handleNodes(context.Background(), tc.nodeInfo, tc.resourceClaimInfo, tc.resourceSliceInfo, tc.composableDRASpec)
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
		})
	}
}
