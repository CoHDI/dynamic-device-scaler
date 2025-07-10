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
	resourceapi "k8s.io/api/resource/v1beta1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func RescheduleFailedNotification(ctx context.Context, kubeClient client.Client, node types.NodeInfo, resourceClaimInfos []types.ResourceClaimInfo, resourceSliceInfos []types.ResourceSliceInfo, composableDRASpec types.ComposableDRASpec) ([]types.ResourceClaimInfo, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start RescheduleFailedNotification")

	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := kubeClient.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return resourceClaimInfos, err
	}

	sortByTime(resourceClaimInfos, "Descending")

	var err error

outerLoop:
	for k, rc := range resourceClaimInfos {
		for i, rcDevice := range rc.Devices {
			if rcDevice.State != "Preparing" {
				continue outerLoop
			}
			for j, otherDevice := range rc.Devices {
				if i != j && rcDevice.Model != otherDevice.Model {
					if !isDeviceCoexistence(rcDevice.Model, otherDevice.Model, composableDRASpec) {
						resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
						if err != nil {
							return resourceClaimInfos, err
						}
						continue outerLoop
					}
				}
			}

			resourceSlice := &resourceapi.ResourceSlice{}
			if err := kubeClient.Get(ctx, k8stypes.NamespacedName{Name: rcDevice.ResourceSliceName}, resourceSlice); err != nil {
				return resourceClaimInfos, err
			}
			exit := false
			for _, resourceSliceDevice := range resourceSlice.Spec.Devices {
				if rcDevice.Name == resourceSliceDevice.Name {
					exit = true
					break
				}
			}
			if !exit {
				resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
				if err != nil {
					return resourceClaimInfos, err
				}
				continue outerLoop
			}

			for _, composabilityRequest := range composabilityRequestList.Items {
				if composabilityRequest.Spec.Resource.Size > 0 &&
					composabilityRequest.Spec.Resource.TargetNode == rc.NodeName {
					if !isDeviceCoexistence(rcDevice.Model, composabilityRequest.Spec.Resource.Model, composableDRASpec) {
						resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
						if err != nil {
							return resourceClaimInfos, err
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
								resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
								if err != nil {
									return resourceClaimInfos, err
								}
								resourceClaimInfos[i], err = setDevicesState(ctx, kubeClient, rc2, "Failed", "FabricDeviceFailed")
								if err != nil {
									return resourceClaimInfos, err
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
			cofiguredDeviceCount, err := GetConfiguredDeviceCount(ctx, kubeClient, model, node.Name, resourceClaimInfos, resourceSliceInfos)
			if err != nil {
				return resourceClaimInfos, err
			}
			maxDevice, _ := GetModelLimit(node, model)

			if cofiguredDeviceCount > maxDevice {
				resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
				if err != nil {
					return resourceClaimInfos, err
				}
			}
		}
	}

	return resourceClaimInfos, nil
}

func GetModelLimit(node types.NodeInfo, model string) (max int64, min int64) {
	for _, modelConstraint := range node.Models {
		if modelConstraint.Model == model {
			max = int64(modelConstraint.MaxDevice)
			min = int64(modelConstraint.MinDevice)
		}
	}

	return
}

func RescheduleNotification(ctx context.Context, kubeClient client.Client, resourceClaimInfos []types.ResourceClaimInfo, resourceSliceInfos []types.ResourceSliceInfo, labelPrefix string, deviceNoAllocation time.Duration) ([]types.ResourceClaimInfo, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start RescheduleNotification")

	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return resourceClaimInfos, fmt.Errorf("failed to list composableResourceList: %v", err)
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
					if !resourceMatched[resource.Name] && resource.Status.State == "Online" {
						isRed, resourceSliceInfo, deviceName := IsDeviceResourceSliceRed(resource.Status.CDIDeviceID, resourceSliceInfos)
						if isRed {
							isUsed, err := IsDeviceUsedByPod(ctx, kubeClient, deviceName, *resourceSliceInfo)
							if err != nil {
								return resourceClaimInfos, err
							}
							if isUsed {
								continue
							}
							logger.Info("Test info", "RescheduleNotification", "not used by pod")

							isOvertime, err := isLastUsedOverThreshold(resource, labelPrefix, deviceNoAllocation, true)
							if err != nil {
								return resourceClaimInfos, err
							}
							if !isOvertime {
								continue
							}
							logger.Info("Test info", "RescheduleNotification", "overtime")

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
			return resourceClaimInfos, err
		}

		currentTime := time.Now().Format(time.RFC3339)
		for resourceName := range resourceMatched {
			err = PatchComposableResourceAnnotation(ctx, kubeClient, resourceName, labelPrefix+"/last-used-time", currentTime)
			if err != nil {
				return resourceClaimInfos, err
			}
		}
	}

	return resourceClaimInfos, nil
}

func getUniqueModelsWithCounts(resourceClaimInfo types.ResourceClaimInfo) map[string]int {
	modelMap := make(map[string]int)

	for _, device := range resourceClaimInfo.Devices {
		if device.State == "Preparing" {
			modelMap[device.Model]++
		}
	}

	return modelMap
}

func isLastUsedOverThreshold(resource cdioperator.ComposableResource, labelPrefix string, threshold time.Duration, defaultWhenNotExists bool) (bool, error) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return defaultWhenNotExists, nil
	}

	label := labelPrefix + "/last-used-time"
	lastUsedStr, exists := annotations[label]
	if !exists {
		return defaultWhenNotExists, nil
	}

	lastUsedTime, err := time.Parse(time.RFC3339, lastUsedStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse time: %v", err)
	}

	now := time.Now().UTC()
	duration := now.Sub(lastUsedTime.UTC())

	return duration > threshold, nil
}

func isDeviceCoexistence(model1, model2 string, composableDRASpec types.ComposableDRASpec) bool {
	for i := range composableDRASpec.DeviceInfos {
		if composableDRASpec.DeviceInfos[i].CDIModelName == model1 {
			for _, j := range composableDRASpec.DeviceInfos[i].CannotCoexistWith {
				if composableDRASpec.DeviceInfos[j-1].CDIModelName == model2 {
					return false
				}
			}
			break
		}
	}

	return true
}

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

func notIn[T comparable](target T, slice []T) bool {
	return !slices.Contains(slice, target)
}
