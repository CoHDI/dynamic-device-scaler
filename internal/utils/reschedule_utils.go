package utils

import (
	"context"
	"fmt"
	"sort"
	"time"

	"slices"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
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
	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := kubeClient.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return resourceClaimInfos, err
	}

	sortByTime(resourceClaimInfos, "Descending")

	var err error

outerLoop:
	for k, rc := range resourceClaimInfos {
		for i, rcDevice := range rc.Devices {
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

			if rcDevice.State == "Preparing" {
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
	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return resourceClaimInfos, fmt.Errorf("failed to list composableResourceList: %v", err)
	}

	sortByTime(resourceClaimInfos, "Ascending")

	var err error

OuterLoop:
	for k, rc := range resourceClaimInfos {
		resourceMatched := make(map[string]bool)
		modelMap := getUniqueModelsWithCounts(rc)
	MiddleLoop:
		for model, count := range modelMap {
			matchedCount := 0
			for _, resource := range resourceList.Items {
				if resource.Spec.Model == model && resource.Spec.TargetNode == rc.NodeName {
					if !resourceMatched[resource.Name] && resource.Status.State == "Online" {
						isRed, resourceSliceInfo := IsDeviceResourceSliceRed(resource.Status.CDIDeviceID, resourceSliceInfos)
						if isRed {
							isUsed, err := IsDeviceUsedByPod(ctx, kubeClient, resource.Status.CDIDeviceID, *resourceSliceInfo)
							if err != nil {
								return resourceClaimInfos, err
							}
							if isUsed {
								continue
							}

							isOvertime, err := isLastUsedOverTime(resource, labelPrefix, deviceNoAllocation)
							if err != nil {
								return resourceClaimInfos, err
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
			continue OuterLoop
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

func isLastUsedOverTime(resource cdioperator.ComposableResource, labelPrefix string, deviceNoAllocation time.Duration) (bool, error) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return false, fmt.Errorf("annotations not found")
	}

	var lastUsedTime time.Time
	var err error

	label := labelPrefix + "/last-used-time"
	lastUsedStr, exists := annotations[label]
	if !exists {
		lastUsedTime = resource.CreationTimestamp.Time
	} else {

		lastUsedTime, err = time.Parse(time.RFC3339, lastUsedStr)
		if err != nil {
			return false, fmt.Errorf("failed to parse time: %v", err)
		}
	}

	now := time.Now().UTC()
	duration := now.Sub(lastUsedTime.UTC())

	return duration > deviceNoAllocation, nil
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
	for k := range resourceClaimInfo.Devices {
		resourceClaimInfo.Devices[k].State = targetState
	}

	return resourceClaimInfo, PatchResourceClaimDeviceConditions(ctx, kubeClient, resourceClaimInfo.Name, resourceClaimInfo.Namespace, conditionType)
}

func notIn[T comparable](target T, slice []T) bool {
	return !slices.Contains(slice, target)
}
