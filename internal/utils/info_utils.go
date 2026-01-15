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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/CoHDI/dynamic-device-scaler/internal/types"
	v1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var driverUUIDAttrMap = map[string]string{
	"gpu.nvidia.com": "uuid",
}

// GetResourceClaimInfo retrieves ResourceClaim information.
func GetResourceClaimInfo(ctx context.Context, kubeClient client.Client, composableDRASpec types.ComposableDRASpec) ([]types.ResourceClaimInfo, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start collecting ResourceClaim info")

	var resourceClaimInfoList []types.ResourceClaimInfo

	resourceClaimList := &resourceapi.ResourceClaimList{}
	if err := kubeClient.List(ctx, resourceClaimList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list ResourceClaims: %v", err)
	}

	resourceSliceList := &resourceapi.ResourceSliceList{}
	if err := kubeClient.List(ctx, resourceSliceList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list ResourceSlices: %v", err)
	}

	for _, rc := range resourceClaimList.Items {
		if len(rc.Status.ReservedFor) == 0 || rc.Status.Allocation == nil {
			continue
		}

		var resourceClaimInfo types.ResourceClaimInfo

		for _, device := range rc.Status.Allocation.Devices.Results {
			if len(device.BindingConditions) == 0 {
				continue
			}
			var deviceInfo types.ResourceClaimDevice

			deviceInfo.Name = device.Device
			deviceInfo.Driver = device.Driver
			deviceInfo.Pool = device.Pool

		ResourceSliceLoop:
			for _, rs := range resourceSliceList.Items {
				if rs.Spec.Driver == device.Driver && rs.Spec.Pool.Name == device.Pool {
					for _, resourceSliceDevice := range rs.Spec.Devices {
						if resourceSliceDevice.Name == device.Device {
							re := regexp.MustCompile(`-fabric\d+$`)
							deviceName := re.ReplaceAllString(device.Pool, "")
							model, err := getModelName(composableDRASpec, deviceName)
							if err != nil {
								return nil, err
							}
							logger.Info("Found model name for device", "device", device.Device, "model", model)
							deviceInfo.Model = model
							break ResourceSliceLoop
						}
					}
				}
			}

			if len(rc.Status.Devices) == 0 {
				deviceInfo.State = "Preparing"
			} else {
				for _, deivedeviceInfo := range rc.Status.Devices {
					if deivedeviceInfo.Device == device.Device {
						if deivedeviceInfo.Conditions != nil {
							if hasConditionWithStatus(deivedeviceInfo.Conditions, "FabricDeviceReschedule", metav1.ConditionTrue) {
								deviceInfo.State = "Reschedule"
							} else if hasConditionWithStatus(deivedeviceInfo.Conditions, "FabricDeviceFailed", metav1.ConditionTrue) {
								deviceInfo.State = "Failed"
							} else if !hasMatchingBindingCondition(deivedeviceInfo.Conditions, device.BindingConditions, device.BindingFailureConditions) {
								deviceInfo.State = "Preparing"
							}
						}
					}
				}
			}

			resourceClaimInfo.Devices = append(resourceClaimInfo.Devices, deviceInfo)
		}

		if len(resourceClaimInfo.Devices) == 0 {
			continue
		}

		resourceClaimInfo.Name = rc.Name
		resourceClaimInfo.Namespace = rc.Namespace
		resourceClaimInfo.CreationTimestamp = rc.ObjectMeta.CreationTimestamp
		if rc.Status.Allocation.NodeSelector != nil {
			resourceClaimInfo.NodeName = getNodeName(*rc.Status.Allocation.NodeSelector)
		}

		resourceClaimInfoList = append(resourceClaimInfoList, resourceClaimInfo)
	}

	logger.V(1).Info("Finish collecting ResourceClaim info", "resourceClaimInfos", resourceClaimInfoList)

	return resourceClaimInfoList, nil
}

// GetResourceSliceInfo retrieves ResourceSlice information.
func GetResourceSliceInfo(ctx context.Context, kubeClient client.Client) ([]types.ResourceSliceInfo, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start collecting ResourceSlice info")

	var resourceSliceInfoList []types.ResourceSliceInfo

	resourceSliceList := &resourceapi.ResourceSliceList{}
	if err := kubeClient.List(ctx, resourceSliceList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list ResourceSlices: %v", err)
	}

	for _, rs := range resourceSliceList.Items {
		if len(rs.Spec.Devices) == 0 {
			continue
		}
		if hasBindingConditions(rs) {
			continue
		}
		if rs.Spec.NodeName == nil {
			continue
		}

		var resourceSliceInfo types.ResourceSliceInfo

		resourceSliceInfo.Name = rs.Name
		resourceSliceInfo.CreationTimestamp = rs.CreationTimestamp
		resourceSliceInfo.Driver = rs.Spec.Driver
		resourceSliceInfo.NodeName = *rs.Spec.NodeName
		resourceSliceInfo.Pool = rs.Spec.Pool.Name

		for _, device := range rs.Spec.Devices {
			var deviceInfo types.ResourceSliceDevice
			deviceInfo.Name = device.Name
			for attrName, attrValue := range device.Attributes {
				if driverUUIDAttrMap[rs.Spec.Driver] == string(attrName) {
					deviceInfo.UUID = *attrValue.StringValue
				}
			}
			resourceSliceInfo.Devices = append(resourceSliceInfo.Devices, deviceInfo)
		}

		resourceSliceInfoList = append(resourceSliceInfoList, resourceSliceInfo)
	}

	logger.V(1).Info("Finish collecting ResourceSlice info", "resourceSliceInfos", resourceSliceInfoList)

	return resourceSliceInfoList, nil
}

// GetNodeInfo retrieves Node information.
func GetNodeInfo(ctx context.Context, clientSet kubernetes.Interface, composableDRASpec types.ComposableDRASpec) ([]types.NodeInfo, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start collecting Node info")

	nodes, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Nodes: %v", err)
	}

	nodeInfos, err := processNodeInfo(nodes, composableDRASpec)
	if err != nil {
		return nil, err
	}

	logger.V(1).Info("Finish collecting Node info", "nodeInfos", nodeInfos)
	return nodeInfos, nil
}

// processNodeInfo processes Node information.
func processNodeInfo(nodes *v1.NodeList, composableDRASpec types.ComposableDRASpec) ([]types.NodeInfo, error) {
	var nodeInfoList []types.NodeInfo

	for _, node := range nodes.Items {
		var nodeInfo types.NodeInfo

		nodeInfo.Name = node.Name

		maxDeviceSet := make(map[string]bool)
		minDeviceSet := make(map[string]bool)
		labels := node.Labels
		for key, val := range labels {
			if !strings.HasPrefix(key, composableDRASpec.LabelPrefix+"/") {
				continue
			}

			suffix := key[len(composableDRASpec.LabelPrefix+"/"):]
			var exit bool
			if strings.HasSuffix(suffix, "-size-max") {
				max, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid integer in %s: %v", val, err)
				}

				deviceName := suffix[:len(suffix)-9]
				model, err := getModelName(composableDRASpec, deviceName)
				if err != nil {
					return nil, err
				}
				maxDeviceSet[deviceName] = true

				exit = false
				for i := range nodeInfo.Models {
					if nodeInfo.Models[i].DeviceName == deviceName {
						nodeInfo.Models[i].MaxDevice = max
						nodeInfo.Models[i].Model = model
						nodeInfo.Models[i].MaxDeviceSet = true
						exit = true
						break
					}
				}

				if !exit {
					newModelConstraint := types.ModelConstraints{
						DeviceName:   deviceName,
						Model:        model,
						MaxDevice:    max,
						MaxDeviceSet: true,
					}

					nodeInfo.Models = append(nodeInfo.Models, newModelConstraint)
				}
			} else if strings.HasSuffix(suffix, "-size-min") {
				min, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid integer in %s: %v", val, err)
				}

				deviceName := suffix[:len(suffix)-9]
				model, err := getModelName(composableDRASpec, deviceName)
				if err != nil {
					return nil, err
				}
				minDeviceSet[deviceName] = true

				exit = false
				for i := range nodeInfo.Models {
					if nodeInfo.Models[i].DeviceName == deviceName {
						nodeInfo.Models[i].MinDevice = min
						nodeInfo.Models[i].Model = model
						exit = true
						break
					}
				}

				if !exit {
					newModelConstraint := types.ModelConstraints{
						DeviceName: deviceName,
						Model:      model,
						MinDevice:  min,
					}

					nodeInfo.Models = append(nodeInfo.Models, newModelConstraint)
				}
			}
		}

		nodeInfoList = append(nodeInfoList, nodeInfo)
	}

	return nodeInfoList, nil
}

// getModelName retrieves the model name for a specific device.
func getModelName(composableDRASpec types.ComposableDRASpec, deviceName string) (string, error) {
	for _, deviceInfo := range composableDRASpec.DeviceInfos {
		if deviceInfo.K8sDeviceName == deviceName {
			return deviceInfo.CDIModelName, nil
		}
	}

	return "", fmt.Errorf("unknown device name: %s", deviceName)
}

// GetConfigMapInfo retrieves ConfigMap information.
func GetConfigMapInfo(ctx context.Context, clientSet kubernetes.Interface) (types.ComposableDRASpec, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start collecting ConfigMap info")

	var composableDRASpec types.ComposableDRASpec

	configMap, err := clientSet.CoreV1().ConfigMaps("composable-dra").Get(ctx, "composable-dra-dds", metav1.GetOptions{})
	if err != nil {
		return composableDRASpec, fmt.Errorf("failed to get ConfigMap: %v", err)
	}

	if err = yaml.Unmarshal([]byte(configMap.Data["device-info"]), &composableDRASpec.DeviceInfos); err != nil {
		return composableDRASpec, fmt.Errorf("failed to parse device-info: %v", err)
	}
	if err = validateDeviceInfo(composableDRASpec.DeviceInfos); err != nil {
		return composableDRASpec, fmt.Errorf("invalid device-info: %v, error: %v", composableDRASpec.DeviceInfos, err)
	}

	composableDRASpec.LabelPrefix = configMap.Data["label-prefix"]
	if !isValidLabelNamePrefix(composableDRASpec.LabelPrefix) {
		return composableDRASpec, fmt.Errorf("invalid label prefix: %s", composableDRASpec.LabelPrefix)
	}

	if err := yaml.Unmarshal([]byte(configMap.Data["fabric-id-range"]), &composableDRASpec.FabricIDRange); err != nil {
		return composableDRASpec, fmt.Errorf("failed to parse fabric-id-range: %v", err)
	}
	if len(composableDRASpec.FabricIDRange) > 100 {
		return composableDRASpec, fmt.Errorf("fabric-id-range exceeds 100 item limit: %v", composableDRASpec.FabricIDRange)
	}

	logger.V(1).Info("Finish collecting ConfigMap info", "composableDRASpec", composableDRASpec)

	return composableDRASpec, nil
}

func validateDeviceInfo(infos []types.DeviceInfo) error {
	for _, info := range infos {
		if info.Index < 0 || info.Index > 10000 {
			return fmt.Errorf("index must be between 0 and 10000")
		}
		if len(info.CDIModelName) > 1024 {
			return fmt.Errorf("cdi-model-name exceeds 1KB limit")
		}
		if len(info.DriverName) > 1024 {
			return fmt.Errorf("driver-name exceeds 1KB limit")
		}
		if err := validateDNSLabel(info.K8sDeviceName, 50); err != nil {
			return fmt.Errorf("k8s-device-name invalid: %v", err)
		}
		if len(info.CannotCoexistWith) > 100 {
			return fmt.Errorf("cannot-coexist-with exceeds 100 item limit")
		}
	}

	return nil
}

// validateDNSLabel validates a DNS label according to Kubernetes naming conventions.
func validateDNSLabel(name string, maxLength int) error {
	if len(name) == 0 {
		return errors.New("cannot be empty")
	}
	if len(name) > maxLength {
		return fmt.Errorf("exceeds %d character limit", maxLength)
	}

	first := rune(name[0])
	if !unicode.IsLetter(first) && !unicode.IsDigit(first) {
		return errors.New("must start with letter or digit")
	}

	last := rune(name[len(name)-1])
	if !unicode.IsLetter(last) && !unicode.IsDigit(last) {
		return errors.New("must end with letter or digit")
	}

	for _, c := range name {
		switch {
		case unicode.IsLetter(c) || unicode.IsDigit(c):
		case c == '-':
		default:
			return fmt.Errorf("contains invalid character '%c'", c)
		}
	}

	return nil
}

// isValidLabelNamePrefix checks if a label name prefix is valid.
func isValidLabelNamePrefix(s string) bool {
	if len(s) > 100 {
		return false
	}

	if s == "" {
		return false
	}

	first := rune(s[0])
	if !unicode.IsLetter(first) && !unicode.IsDigit(first) {
		return false
	}

	last := rune(s[len(s)-1])
	if !unicode.IsLetter(last) && !unicode.IsDigit(last) {
		return false
	}

	prev := rune(0)
	for _, c := range s {
		switch {
		case unicode.IsLetter(c) || unicode.IsDigit(c):
		case c == '.' || c == '-':
		default:
			return false
		}

		if c == '.' && prev == '.' {
			return false
		}
		prev = c
	}

	return true
}

// hasConditionWithStatus checks if a specific condition with a given status exists.
func hasConditionWithStatus(conditions []metav1.Condition, conditionType string, status metav1.ConditionStatus) bool {
	for _, c := range conditions {
		if c.Type == conditionType && c.Status == status {
			return true
		}
	}
	return false
}

// hasMatchingBindingCondition checks if there is a matching binding condition.
func hasMatchingBindingCondition(
	conditions []metav1.Condition,
	bindingConditions []string,
	bindingFailureConditions []string,
) bool {
	if len(conditions) == 0 {
		return false
	}

	conditionSet := make(map[string]struct{}, len(bindingConditions)+len(bindingFailureConditions))

	for _, cond := range bindingConditions {
		conditionSet[cond] = struct{}{}
	}

	for _, cond := range bindingFailureConditions {
		conditionSet[cond] = struct{}{}
	}

	if len(conditionSet) == 0 {
		return false
	}

	for _, condition := range conditions {
		if _, exists := conditionSet[condition.Type]; exists {
			if condition.Status == metav1.ConditionTrue {
				return true
			}
		}
	}

	return false
}

// hasBindingConditions checks if a ResourceSlice has any binding conditions.
func hasBindingConditions(rs resourceapi.ResourceSlice) bool {
	for _, device := range rs.Spec.Devices {
		if len(device.BindingConditions) > 0 {
			return true
		}
	}

	return false
}

// getNodeName retrieves the node name from a NodeSelector.
func getNodeName(selector v1.NodeSelector) string {
	for _, term := range selector.NodeSelectorTerms {
		for _, field := range term.MatchFields {
			if field.Key == "metadata.name" && field.Operator == "In" {
				return field.Values[0]
			}
		}
	}
	return ""
}
