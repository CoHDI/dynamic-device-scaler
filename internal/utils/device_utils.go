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
	"fmt"
	"time"

	cdioperator "github.com/CoHDI/composable-resource-operator/api/v1alpha1"
	"github.com/CoHDI/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetConfiguredDeviceCount returns the total number of configured devices for a specific model and node.
func GetConfiguredDeviceCount(ctx context.Context, kubeClient client.Client, model, nodeName string, resourceClaimInfos []types.ResourceClaimInfo, resourceSliceInfos []types.ResourceSliceInfo) (int64, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start getting configured device count")

	preparingAndRescheduleDeviceCount := getPreparingRescheduleDevicesCount(resourceClaimInfos, model, nodeName)

	podAllocatedDevicesCount, err := getPodAllocatedDevicesCount(ctx, kubeClient, model, nodeName, resourceSliceInfos)
	if err != nil {
		return 0, err
	}

	logger.V(1).Info("Finish getting configured device count", "preparingAndRescheduleDeviceCount", preparingAndRescheduleDeviceCount, "podAllocatedDevicesCount", podAllocatedDevicesCount)

	return preparingAndRescheduleDeviceCount + podAllocatedDevicesCount, nil
}

// getPreparingRescheduleDevicesCount returns the count of devices in preparing or reschedule state for a specific model and node.
func getPreparingRescheduleDevicesCount(resourceClaimInfos []types.ResourceClaimInfo, model, nodeName string) int64 {
	var count int64

	for _, rc := range resourceClaimInfos {
		if rc.NodeName != nodeName {
			continue
		}
		for _, device := range rc.Devices {
			if device.Model == model && (device.State == "Preparing" || device.State == "Reschedule") {
				count++
			}
		}
	}

	return count
}

// getPodAllocatedDevicesCount returns the count of devices allocated to pods for a specific model and node.
func getPodAllocatedDevicesCount(ctx context.Context, kubeClient client.Client, model, nodeName string, resourceSliceInfos []types.ResourceSliceInfo) (int64, error) {
	var count int64

	composableResourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, composableResourceList, &client.ListOptions{}); err != nil {
		return count, fmt.Errorf("failed to list composableResourceList: %v", err)
	}

	for _, resource := range composableResourceList.Items {
		if resource.Spec.TargetNode != nodeName {
			continue
		}
		if resource.Spec.Model == model {
			if resource.Status.State == "Online" {
				isRed, resourceSliceInfo, deviceName := IsDeviceResourceSliceRed(resource.Status.DeviceID, resourceSliceInfos)
				if isRed {
					isUsed, err := IsDeviceUsedByPod(ctx, kubeClient, deviceName, *resourceSliceInfo)
					if err != nil {
						return count, err
					}
					if isUsed {
						count++
					}
				}
			}
		}
	}

	return count, nil
}

// IsDeviceUsedByPod checks if a device is used by a pod.
func IsDeviceUsedByPod(ctx context.Context, kubeClient client.Client, deviceName string, resourceSliceInfo types.ResourceSliceInfo) (bool, error) {
	resourceClaimList := &resourceapi.ResourceClaimList{}
	if err := kubeClient.List(ctx, resourceClaimList, &client.ListOptions{}); err != nil {
		return false, fmt.Errorf("failed to list ResourceClaims: %v", err)
	}

	for _, resourceClaim := range resourceClaimList.Items {
		if resourceClaim.Status.Allocation != nil {
			for _, resourceClaimDevice := range resourceClaim.Status.Allocation.Devices.Results {
				if resourceSliceInfo.Pool == resourceClaimDevice.Pool &&
					resourceSliceInfo.Driver == resourceClaimDevice.Driver &&
					deviceName == resourceClaimDevice.Device {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// IsDeviceResourceSliceRed checks if a device is in a red state within a resource slice.
func IsDeviceResourceSliceRed(deviceID string, resourceSliceInfos []types.ResourceSliceInfo) (bool, *types.ResourceSliceInfo, string) {
	for _, resourceSlice := range resourceSliceInfos {
		for _, resourceSliceDevice := range resourceSlice.Devices {
			if resourceSliceDevice.UUID == deviceID {
				return true, &resourceSlice, resourceSliceDevice.Name
			}
		}
	}

	return false, nil, ""
}

// DynamicAttach dynamically attaches resources to a ComposabilityRequest.
func DynamicAttach(ctx context.Context, kubeClient client.Client, cr *cdioperator.ComposabilityRequest, count int64, resourceType, model, nodeName string) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Start dynamic attach")

	if cr == nil {
		return createNewComposabilityRequestCR(ctx, kubeClient, count, resourceType, model, nodeName)
	}

	return PatchComposabilityRequestSize(ctx, kubeClient, cr.Name, count)
}

// createNewComposabilityRequestCR creates a new ComposabilityRequest custom resource.
func createNewComposabilityRequestCR(ctx context.Context, kubeClient client.Client, count int64, resourceType, model, node string) error {
	logger := ctrl.LoggerFrom(ctx)

	logger.Info("Create new ComposabilityRequestCR",
		"resourceType", resourceType,
		"requestCount", count)

	newCR := &cdioperator.ComposabilityRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "composability-",
		},
		Spec: cdioperator.ComposabilityRequestSpec{
			Resource: cdioperator.ScalarResourceDetails{
				Type:       resourceType,
				Model:      model,
				Size:       count,
				TargetNode: node,
			},
		},
	}

	if err := kubeClient.Create(ctx, newCR); err != nil {
		return fmt.Errorf("failed to create ComposabilityRequest: %v", err)
	}

	return nil
}

// DynamicDetach dynamically detaches resources from a ComposabilityRequest.
func DynamicDetach(ctx context.Context, kubeClient client.Client, cr *cdioperator.ComposabilityRequest, count int64, nodeName, labelPrefix string, deviceNoRemoval time.Duration) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Start dynamic detach")

	nextSize, err := getNextSize(ctx, kubeClient, count, nodeName, labelPrefix, deviceNoRemoval)
	if err != nil {
		return fmt.Errorf("failed to get next size: %v", err)
	}

	if nextSize < cr.Spec.Resource.Size {
		return PatchComposabilityRequestSize(ctx, kubeClient, cr.Name, nextSize)
	}

	return nil
}

// getNextSize calculates the next size for a ComposabilityRequest based on the current resource usage.
func getNextSize(ctx context.Context, kubeClient client.Client, count int64, nodeName, labelPrefix string, deviceNoRemoval time.Duration) (int64, error) {
	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return 0, fmt.Errorf("failed to list ComposableResourceList: %v", err)
	}

	var resourceCount int64
	for _, resource := range resourceList.Items {
		if (resource.Status.State == "Online" || resource.Status.State == "Attaching") &&
			resource.Spec.TargetNode == nodeName && resource.DeletionTimestamp == nil {
			over, err := isLastUsedOverThreshold(resource, labelPrefix, deviceNoRemoval, true)
			if err != nil {
				return 0, err
			}

			if !over {
				resourceCount++
			}
		}
	}

	if count > resourceCount {
		return count, nil
	}

	return resourceCount, nil
}

// GetDriverType returns the driver type for a specific model.
func GetDriverType(model string) string {
	switch model {
	case "gpu.nvidia.com":
		return "gpu"
	default:
		return ""
	}
}
