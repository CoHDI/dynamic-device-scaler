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
	"sort"
	"time"

	"slices"

	"github.com/CoHDI/dynamic-device-scaler/internal/types"
	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	resourceapi "k8s.io/api/resource/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// sortByTime sorts ResourceClaimInfo slices by their creation timestamp.
func sortByTime(resourceClaims []types.ResourceClaimInfo, order string) {
	sort.Slice(resourceClaims, func(i, j int) bool {
		timeI := resourceClaims[i].CreationTimestamp.Time
		timeJ := resourceClaims[j].CreationTimestamp.Time

		if order == "Ascending" {
			return timeI.Before(timeJ)
		} else {
			return timeI.After(timeJ)
		}
	})
}

// RescheduleFailedNotification handles the rescheduling of failed notifications.
func RescheduleFailedNotification(ctx context.Context, kubeClient client.Client, node types.NodeInfo, resourceClaimInfos []types.ResourceClaimInfo, resourceSliceInfos []types.ResourceSliceInfo, composableDRASpec types.ComposableDRASpec) ([]types.ResourceClaimInfo, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start RescheduleFailedNotification")

	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := kubeClient.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return resourceClaimInfos, fmt.Errorf("failed to list composabilityRequestList: %w", err)
	}

	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return resourceClaimInfos, fmt.Errorf("failed to list composableResource: %v", err)
	}

	resourceSliceList := &resourceapi.ResourceSliceList{}
	if err := kubeClient.List(ctx, resourceSliceList, &client.ListOptions{}); err != nil {
		return resourceClaimInfos, fmt.Errorf("failed to list ResourceSlices: %v", err)
	}

	sortByTime(resourceClaimInfos, "Descending")

	var err error
	var exit bool

outerLoop:
	for k, rc := range resourceClaimInfos {
		for i, rcDevice := range rc.Devices {
			if rcDevice.State != "Preparing" {
				continue outerLoop
			}
			for j, otherDevice := range rc.Devices {
				if i != j && rcDevice.Model != otherDevice.Model {
					if !isDeviceCoexistence(rcDevice.Model, otherDevice.Model, composableDRASpec) {
						logger.V(1).Info("Setting FabricDeviceFailed to Failed due to incompatibility between devices",
							"rcDevice", rcDevice.Model,
							"otherDevice", otherDevice.Model)
						resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
						if err != nil {
							return resourceClaimInfos, fmt.Errorf("failed to set devices state: %w", err)
						}
						continue outerLoop
					}
				}
			}

			exit = false
		rsLoop:
			for _, rs := range resourceSliceList.Items {
				if rs.Spec.NodeName == nil && rs.Spec.Driver == rcDevice.Driver && rs.Spec.Pool.Name == rcDevice.Pool {
					for _, resourceSliceDevice := range rs.Spec.Devices {
						if resourceSliceDevice.Name == rcDevice.Name {
							exit = true
							break rsLoop
						}
					}
				}
			}
			if !exit {
				logger.V(1).Info("Setting FabricDeviceFailed to Failed due to device was not found in resourceSlice", "device", rcDevice.Name)
				resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
				if err != nil {
					return resourceClaimInfos, fmt.Errorf("failed to set devices state: %w", err)
				}
				continue outerLoop
			}

			for _, composabilityRequest := range composabilityRequestList.Items {
				if composabilityRequest.Spec.Resource.Size > 0 &&
					composabilityRequest.Spec.Resource.TargetNode == rc.NodeName {
					if !isDeviceCoexistence(rcDevice.Model, composabilityRequest.Spec.Resource.Model, composableDRASpec) {
						logger.V(1).Info("Setting FabricDeviceFailed to Failed due to device is incompatible with composability request resource model",
							"device", rcDevice.Name,
							"deviceModel", rcDevice.Model,
							"composabilityRequestResourceModel", composabilityRequest.Spec.Resource.Model)
						resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
						if err != nil {
							return resourceClaimInfos, fmt.Errorf("failed to set devices state: %w", err)
						}
						continue outerLoop
					}
				}
			}

			for _, resource := range resourceList.Items {
				if (resource.Status.State == "Online" || resource.Status.State == "Attaching") &&
					resource.Spec.TargetNode == rc.NodeName {
					if !isDeviceCoexistence(rcDevice.Model, resource.Spec.Model, composableDRASpec) {
						logger.V(1).Info("Setting FabricDeviceFailed to Failed due to device is incompatible with resource model",
							"device", rcDevice.Name,
							"deviceModel", rcDevice.Model,
							"resourceModel", resource.Spec.Model,
							"resourceState", resource.Status.State)
						resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
						if err != nil {
							return resourceClaimInfos, fmt.Errorf("failed to set devices state: %w", err)
						}
						continue outerLoop
					}
				}
			}

			for i, rc2 := range resourceClaimInfos {
				if rc.Name != rc2.Name {
					for _, rc2Device := range rc2.Devices {
						if rc2Device.State == "Preparing" && rcDevice.Model != rc2Device.Model {
							if !isDeviceCoexistence(rcDevice.Model, rc2Device.Model, composableDRASpec) {
								logger.V(1).Info("Setting FabricDeviceFailed to Failed due to device is incompatible with another device in resource claim",
									"device1", rcDevice.Name,
									"device1Model", rcDevice.Model,
									"device2", rc2Device.Name,
									"device2Model", rc2Device.Model,
									"resourceClaim", rc2.Name)
								resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
								if err != nil {
									return resourceClaimInfos, fmt.Errorf("failed to set devices state: %w", err)
								}
								logger.V(1).Info("Setting FabricDeviceFailed to Failed due to device is incompatible with another device in resource claim",
									"device1", rcDevice.Name,
									"device1Model", rcDevice.Model,
									"device2", rc2Device.Name,
									"device2Model", rc2Device.Model,
									"resourceClaim", rc2.Name)
								resourceClaimInfos[i], err = setDevicesState(ctx, kubeClient, rc2, "Failed", "FabricDeviceFailed")
								if err != nil {
									return resourceClaimInfos, fmt.Errorf("failed to set devices state: %w", err)
								}
								continue outerLoop
							}
						}
					}
				}
			}
		}

		modelMap := getUniqueModelsWithCounts(rc)
		for model := range modelMap {
			configuredDeviceCount, err := GetConfiguredDeviceCount(ctx, kubeClient, model, node.Name, resourceClaimInfos, resourceSliceInfos)
			if err != nil {
				return resourceClaimInfos, fmt.Errorf("failed to get configured device count: %w", err)
			}
			maxDevice, _ := GetModelLimit(node, model)

			if configuredDeviceCount > maxDevice {
				logger.V(1).Info("Setting FabricDeviceFailed to Failed due to configured device count exceeds maximum limit",
					"configuredDeviceCount", configuredDeviceCount,
					"model", model,
					"maxDeviceLimit", maxDevice)
				resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
				if err != nil {
					return resourceClaimInfos, fmt.Errorf("failed to set devices state: %w", err)
				}
			}
		}
	}

	return resourceClaimInfos, nil
}

// GetModelLimit retrieves the model limits for a given node and model.
func GetModelLimit(node types.NodeInfo, model string) (max int64, min int64) {
	const (
		defaultMaxDevice = 50
		defaultMinDevice = 0
	)

	max, min = defaultMaxDevice, defaultMinDevice

	for _, modelConstraint := range node.Models {
		if modelConstraint.Model == model {
			if modelConstraint.MaxDeviceSet {
				max = int64(modelConstraint.MaxDevice)
			}
			min = int64(modelConstraint.MinDevice)
			return
		}
	}

	return
}

// RescheduleNotification handles the rescheduling of notifications.
func RescheduleNotification(ctx context.Context, kubeClient client.Client, resourceClaimInfos []types.ResourceClaimInfo, resourceSliceInfos []types.ResourceSliceInfo, labelPrefix string, deviceNoAllocation time.Duration) ([]types.ResourceClaimInfo, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start RescheduleNotification")

	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return resourceClaimInfos, fmt.Errorf("failed to list composableResource: %v", err)
	}

	if len(resourceList.Items) == 0 {
		logger.Info("No ComposableResource found, skipping reschedule notification")
		return resourceClaimInfos, nil
	}

	sortByTime(resourceClaimInfos, "Ascending")

	var err error

outerLoop:
	for k, rc := range resourceClaimInfos {
		for _, rcDevice := range rc.Devices {
			if rcDevice.State != "Preparing" {
				continue outerLoop
			}
		}
		resourceMatched := make(map[string]bool)
		modelMap := getUniqueModelsWithCounts(rc)
	MiddleLoop:
		for model, count := range modelMap {
			matchedCount := 0
			for _, resource := range resourceList.Items {
				if resource.Spec.Model == model && resource.Spec.TargetNode == rc.NodeName {
					if !resourceMatched[resource.Name] && resource.Status.State == "Online" && resource.DeletionTimestamp == nil {
						isRed, resourceSliceInfo, deviceName := IsDeviceResourceSliceRed(resource.Status.DeviceID, resourceSliceInfos)
						if isRed {
							isUsed, err := IsDeviceUsedByPod(ctx, kubeClient, deviceName, *resourceSliceInfo)
							if err != nil {
								return resourceClaimInfos, fmt.Errorf("failed to check if device is used by pod: %w", err)
							}
							if isUsed {
								continue
							}

							isOvertime, err := isLastUsedOverThreshold(resource, labelPrefix, deviceNoAllocation, false)
							if err != nil {
								logger.Info(fmt.Sprintf("warning: failed to check if last used time is over threshold: %v", err))
							}
							if !isOvertime {
								continue
							}

							resourceMatched[resource.Name] = true
							matchedCount++

							if matchedCount >= count {
								continue MiddleLoop
							}
						}
					}
				}
			}
			continue outerLoop
		}

		resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Reschedule", "FabricDeviceReschedule")
		if err != nil {
			return resourceClaimInfos, fmt.Errorf("failed to set devices state: %w", err)
		}

		currentTime := time.Now().Format(time.RFC3339)
		for resourceName := range resourceMatched {
			err = PatchComposableResourceAnnotation(ctx, kubeClient, resourceName, labelPrefix+"/last-used-time", currentTime)
			if err != nil {
				return resourceClaimInfos, fmt.Errorf("failed to patch composable resource annotation: %w", err)
			}
		}
	}

	return resourceClaimInfos, nil
}

// getUniqueModelsWithCounts retrieves the unique models and their counts from a ResourceClaimInfo.
func getUniqueModelsWithCounts(resourceClaimInfo types.ResourceClaimInfo) map[string]int {
	modelMap := make(map[string]int)

	for _, device := range resourceClaimInfo.Devices {
		if device.State == "Preparing" {
			modelMap[device.Model]++
		}
	}

	return modelMap
}

// isLastUsedOverThreshold checks if the last used time of a resource is over a certain threshold.
func isLastUsedOverThreshold(resource cdioperator.ComposableResource, labelPrefix string, threshold time.Duration, useCreateTimeWhenNotExists bool) (bool, error) {
	label := labelPrefix + "/last-used-time"
	annotations := resource.GetAnnotations()

	if annotations != nil {
		if lastUsedStr, exists := annotations[label]; exists {
			lastUsedTime, err := time.Parse(time.RFC3339, lastUsedStr)
			if err != nil {
				return false, fmt.Errorf("failed to parse time: %v", err)
			}
			duration := time.Now().UTC().Sub(lastUsedTime.UTC())
			return duration > threshold, nil
		}
	}

	if useCreateTimeWhenNotExists {
		createTime := resource.GetCreationTimestamp()
		if createTime.IsZero() {
			return false, fmt.Errorf("creation timestamp is missing")
		}
		duration := time.Now().UTC().Sub(createTime.UTC())
		return duration > threshold, nil
	}

	return true, nil
}

// isDeviceCoexistence checks if two device models can coexist based on the ComposableDRASpec.
func isDeviceCoexistence(model1, model2 string, composableDRASpec types.ComposableDRASpec) bool {
	if model1 == model2 {
		return true
	}

	indexToModel := make(map[int]string)
	for _, deviceInfo := range composableDRASpec.DeviceInfos {
		indexToModel[deviceInfo.Index] = deviceInfo.CDIModelName
	}

	for _, deviceInfo := range composableDRASpec.DeviceInfos {
		if deviceInfo.CDIModelName == model1 {
			for _, idx := range deviceInfo.CannotCoexistWith {
				if model, exists := indexToModel[idx]; exists && model == model2 {
					return false
				}
			}
			break
		}
	}

	return true
}

// setDevicesState sets the state of devices in a ResourceClaimInfo.
func setDevicesState(ctx context.Context, kubeClient client.Client, resourceClaimInfo types.ResourceClaimInfo, targetState string, conditionType string) (types.ResourceClaimInfo, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start setDevicesState",
		"resourceClaimInfoName", resourceClaimInfo.Name,
		"conditionType", conditionType,
		"targetState", targetState)

	for k := range resourceClaimInfo.Devices {
		resourceClaimInfo.Devices[k].State = targetState
	}

	return resourceClaimInfo, PatchResourceClaimDeviceConditions(ctx, kubeClient, resourceClaimInfo.Name, resourceClaimInfo.Namespace, conditionType)
}

// notIn checks if a target element is not present in a slice.
func notIn[T comparable](target T, slice []T) bool {
	return !slices.Contains(slice, target)
}
