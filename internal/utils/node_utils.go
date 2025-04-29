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

func sortByTime(resourceClaims []types.ResourceClaimInfo) {
	sort.Slice(resourceClaims, func(i, j int) bool {
		return resourceClaims[i].CreationTimestamp.After(resourceClaims[j].CreationTimestamp.Time)
	})
}

func RescheduleFailedNotification(ctx context.Context, kubeClient client.Client, node types.NodeInfo, resourceClaims []types.ResourceClaimInfo, composableDRASpec types.ComposableDRASpec) ([]types.ResourceClaimInfo, error) {
	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := kubeClient.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return resourceClaims, err
	}

	sortByTime(resourceClaims)

	var err error

outerLoop:
	for k, rc := range resourceClaims {
		for i, rcDevice := range rc.Devices {
			if rcDevice.State == "Preparing" {
				for j, otherDevice := range rc.Devices {
					if i != j && rcDevice.Model != otherDevice.Model {
						if !isDeviceCoexistence(rcDevice.Model, otherDevice.Model, composableDRASpec) {
							resourceClaims[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
							if err != nil {
								return resourceClaims, err
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
									resourceClaims[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
									if err != nil {
										return resourceClaims, err
									}
									continue outerLoop
								}
							}
						}
					} else if request.Spec.Resource.Size > 0 {
						if !isDeviceCoexistence(rcDevice.Model, request.Spec.Resource.Model, composableDRASpec) {
							resourceClaims[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
							if err != nil {
								return resourceClaims, err
							}
							continue outerLoop
						}
					}
				}

				for i, rc2 := range resourceClaims {
					if rc.Name != rc2.Name {
						for _, rc2Device := range rc2.Devices {
							if rc2Device.State == "Preparing" && rcDevice.Model != rc2Device.Model {
								if !isDeviceCoexistence(rcDevice.Model, rc2Device.Model, composableDRASpec) {
									resourceClaims[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
									if err != nil {
										return resourceClaims, err
									}
									resourceClaims[i], err = setDevicesState(ctx, kubeClient, rc2, "Failed", "FabricDeviceFailed")
									if err != nil {
										return resourceClaims, err
									}
								}
							}
						}
					}
				}
			}
		}
	}

	deviceCount, err := GetConfiguredDeviceCount(ctx, kubeClient, resourceClaims)
	if err != nil {
		return resourceClaims, err
	}

	for _, info := range node.Models {
		for _, modelConstraint := range node.Models {
			if info.Model == modelConstraint.Model {
				if deviceCount[info.Model] > modelConstraint.MaxDevice {
					for k, rc := range resourceClaims {
						for _, device := range rc.Devices {
							if device.Model == info.Model {
								resourceClaims[k], err = setDevicesState(ctx, kubeClient, rc, "Failed", "FabricDeviceFailed")
								if err != nil {
									return resourceClaims, err
								}
							}
						}
					}
				}
			}
		}
	}

	return resourceClaims, nil
}

func RescheduleNotification(ctx context.Context, kubeClient client.Client, node types.NodeInfo, resourceClaims []types.ResourceClaimInfo) ([]types.ResourceClaimInfo, error) {
	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return resourceClaims, fmt.Errorf("failed to list ComposableResourceList: %v", err)
	}

	sortByTime(resourceClaims)

	var err error

	resourceUsed := make(map[string]bool)

outerLoop:
	for k, rc := range resourceClaims {
		for _, device := range rc.Devices {
			if device.State == "Preparing" {
				isRed, err := isResourceSliceRed(ctx, kubeClient, device.Name)
				if err != nil {
					return resourceClaims, err
				}
				if !isRed {
					continue outerLoop
				}
				for _, resource := range resourceList.Items {
					if resourceUsed[resource.Name] {
						continue
					}
					if device.Model == resource.Spec.Model &&
						device.Name == resource.Spec.TargetNode &&
						resource.Status.State != "Online" &&
						!device.UsedByPod {
						over, err := isLastUsedOverMinute(resource)
						if err != nil || !over {
							continue
						}
						resourceUsed[resource.Name] = true
						continue outerLoop
					}
				}
			}
		}
		resourceClaims[k], err = setDevicesState(ctx, kubeClient, rc, "Reschedule", "FabricDeviceReschedule")
		if err != nil {
			return resourceClaims, err
		}
		for _, resource := range resourceList.Items {
			if resourceUsed[resource.Name] {
				currentTime := time.Now().Format(time.RFC3339)
				resource.Annotations["composable.test/last-used-time"] = currentTime
				if err := kubeClient.Update(ctx, &resource); err != nil {
					return resourceClaims, fmt.Errorf("failed to update ComposableResource: %w", err)
				}
			}
		}
	}

	return resourceClaims, nil
}

func isLastUsedOverMinute(resource cdioperator.ComposableResource) (bool, error) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return false, fmt.Errorf("annotations not found")
	}
	lastUsedStr, exists := annotations["composable.test/last-used-time"]
	if !exists {
		return false, fmt.Errorf("annotation %s not found", "composable.test/last-used-time")
	}

	lastUsedTime, err := time.Parse(time.RFC3339, lastUsedStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse time: %v", err)
	}

	now := time.Now().UTC()
	duration := now.Sub(lastUsedTime.UTC())

	return duration > time.Minute, nil
}

func isResourceSliceRed(ctx context.Context, kubeClient client.Client, claimDeviceName string) (bool, error) {
	resourceSliceList := &resourceapi.ResourceSliceList{}
	if err := kubeClient.List(ctx, resourceSliceList, &client.ListOptions{}); err != nil {
		return false, fmt.Errorf("failed to list ResourceClaims: %v", err)
	}

	for _, rs := range resourceSliceList.Items {
		for _, device := range rs.Spec.Devices {
			if device.Name == claimDeviceName {
				//TODO: wait for KEP5007
				// if len(device.Basic.BindingConditions) == 0 {
				// 	return true, nil
				// }
			}
		}
	}

	return false, nil
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

func setDevicesState(ctx context.Context, kubeClient client.Client, resourceClaim types.ResourceClaimInfo, targetState string, conditionType string) (types.ResourceClaimInfo, error) {
	for k, device := range resourceClaim.Devices {
		if device.State == "Preparing" {
			resourceClaim.Devices[k].State = targetState
		}
	}

	namespacedName := k8stypes.NamespacedName{
		Name:      resourceClaim.Name,
		Namespace: resourceClaim.Namespace,
	}

	var rc resourceapi.ResourceClaim

	if err := kubeClient.Get(ctx, namespacedName, &rc); err != nil {
		return resourceClaim, fmt.Errorf("failed to get ResourceClaim: %v", err)
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
		return resourceClaim, nil
	}

	if err := kubeClient.Status().Update(ctx, &rc); err != nil {
		return resourceClaim, fmt.Errorf("failed to update ResourceClaim: %v", err)
	}

	return resourceClaim, nil
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
