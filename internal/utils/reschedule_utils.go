package utils

import (
	"context"
	"fmt"
	"sort"
	"time"

	"slices"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
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
			if rcDevice.State == "Preparing" {
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

				for _, request := range composabilityRequestList.Items {
					if rcDevice.Model == request.Spec.Resource.Model {
						for _, modelConstraint := range node.Models {
							if modelConstraint.Model == rcDevice.Model {
								if request.Spec.Resource.Size > int64(modelConstraint.MaxDevice) {
									resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
									if err != nil {
										return resourceClaimInfos, err
									}
									continue outerLoop
								}
							}
						}
					} else if request.Spec.Resource.Size > 0 {
						if !isDeviceCoexistence(rcDevice.Model, request.Spec.Resource.Model, composableDRASpec) {
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
								}
							}
						}
					}
				}
			}
		}
	}

	for _, info := range node.Models {
		for _, modelConstraint := range node.Models {
			if info.Model == modelConstraint.Model {
				cofiguredDeviceCount, err := GetConfiguredDeviceCount(ctx, kubeClient, info.Model, resourceClaimInfos, resourceSliceInfos)
				if err != nil {
					return resourceClaimInfos, err
				}
				if cofiguredDeviceCount > int64(modelConstraint.MaxDevice) {
					for k, rc := range resourceClaimInfos {
						for _, device := range rc.Devices {
							if device.Model == info.Model {
								resourceClaimInfos[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
								if err != nil {
									return resourceClaimInfos, err
								}
							}
						}
					}
				}
			}
		}
	}

	return resourceClaimInfos, nil
}

func RescheduleNotification(ctx context.Context, kubeClient client.Client, node types.NodeInfo, resourceClaimInfos []types.ResourceClaimInfo, resourceSliceInfos []types.ResourceSliceInfo, labelPrefix string, deviceNoAllocation time.Duration) ([]types.ResourceClaimInfo, error) {
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
			for _, resource := range resourceList.Items {
				matchedCount := 0
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
							if matchedCount < count {
								resourceMatched[resource.Name] = true
								matchedCount++
							} else {
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
		if device.State == "Preaparing" {
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
	label := labelPrefix + "/last-used-time"
	lastUsedStr, exists := annotations[label]
	if !exists {
		return false, fmt.Errorf("annotation %s not found", label)
	}

	lastUsedTime, err := time.Parse(time.RFC3339, lastUsedStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse time: %v", err)
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
		}
	}

	return true
}

func setDevicesState(ctx context.Context, kubeClient client.Client, resourceClaimInfo types.ResourceClaimInfo, targetState string, conditionType string) (types.ResourceClaimInfo, error) {
	for k, device := range resourceClaimInfo.Devices {
		if device.State == "Preparing" {
			resourceClaimInfo.Devices[k].State = targetState
		}
	}

	namespacedName := k8stypes.NamespacedName{
		Name:      resourceClaimInfo.Name,
		Namespace: resourceClaimInfo.Namespace,
	}

	var rc resourceapi.ResourceClaim

	if err := kubeClient.Get(ctx, namespacedName, &rc); err != nil {
		return resourceClaimInfo, fmt.Errorf("failed to get ResourceClaim: %v", err)
	}

	modified := false

	for i := range rc.Status.Devices {
		device := &rc.Status.Devices[i]

		newCondition := metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(time.Now()),
		}

		conditionExists := false
		for j, existingCond := range device.Conditions {
			if existingCond.Type == conditionType {
				if existingCond.Status != newCondition.Status {
					device.Conditions[j] = newCondition
					modified = true
				}
				conditionExists = true
				break
			}
		}

		if !conditionExists {
			device.Conditions = append(device.Conditions, newCondition)
			modified = true
		}
	}

	if !modified {
		return resourceClaimInfo, nil
	}

	if err := kubeClient.Status().Update(ctx, &rc); err != nil {
		return resourceClaimInfo, fmt.Errorf("failed to update ResourceClaim: %v", err)
	}

	return resourceClaimInfo, nil
}

func UpdateNodeLabel(ctx context.Context, kubeClient client.Client, clientSet kubernetes.Interface, nodeInfo types.NodeInfo) error {
	var devices []string

	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := kubeClient.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("failed to list composabilityRequestList: %v", err)
	}

	for _, cr := range composabilityRequestList.Items {
		if cr.Spec.Resource.TargetNode == nodeInfo.Name {
			if cr.Spec.Resource.Size > 0 {
				if notIn(cr.Spec.Resource.Model, devices) {
					devices = append(devices, cr.Spec.Resource.Model)
				}
			}
		}
	}

	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("failed to list ComposableResourceList: %v", err)
	}

	for _, rs := range resourceList.Items {
		if rs.Spec.TargetNode == nodeInfo.Name {
			if rs.Status.State == "Online" {
				if notIn(rs.Spec.Model, devices) {
					devices = append(devices, rs.Spec.Model)
				}
			}
		}
	}

	var addLabels, deleteLabels []string
	var notCoexistID []int

	composableDRASpec, err := GetConfigMapInfo(ctx, clientSet)
	if err != nil {
		return err
	}

	for _, device := range devices {
		for _, deviceInfo := range composableDRASpec.DeviceInfos {
			if device == deviceInfo.CDIModelName {
				notCoexistID = append(notCoexistID, deviceInfo.CannotCoexistWith...)
			}
		}
	}

	for _, deviceInfo := range composableDRASpec.DeviceInfos {
		if notIn(deviceInfo.Index, notCoexistID) {
			label := composableDRASpec.LabelPrefix + "/" + deviceInfo.K8sDeviceName
			addLabels = append(addLabels, label)
		} else {
			label := composableDRASpec.LabelPrefix + "/" + deviceInfo.K8sDeviceName
			deleteLabels = append(deleteLabels, label)
		}
	}

	node, err := clientSet.CoreV1().Nodes().Get(ctx, nodeInfo.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %v", err)
	}

	updateNode := node.DeepCopy()
	if updateNode.Labels == nil {
		updateNode.Labels = make(map[string]string)
	}
	for _, label := range addLabels {
		updateNode.Labels[label] = "true"
	}
	for _, label := range deleteLabels {
		delete(updateNode.Labels, label)
	}
	if _, err = clientSet.CoreV1().Nodes().Update(ctx, updateNode, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update node labels: %v", err)
	}

	return nil
}

func notIn[T comparable](target T, slice []T) bool {
	return !slices.Contains(slice, target)
}
