package utils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	v1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
		return nil, fmt.Errorf("failed to list ResourceClaims: %v", err)
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

		ResourceSliceLoop:
			for _, rs := range resourceSliceList.Items {
				if rs.Spec.Driver == device.Driver && rs.Spec.Pool.Name == device.Pool {
					for _, resourceSliceDevice := range rs.Spec.Devices {
						if resourceSliceDevice.Name == device.Device {
							model, err := getModelName(composableDRASpec, "", *resourceSliceDevice.Basic.Attributes["productName"].StringValue)
							logger.Info("Found model name for device", "device", device.Device, "model", model)
							if err != nil {
								return nil, err
							}
							deviceInfo.Model = model
							deviceInfo.ResourceSliceName = rs.Name
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

func GetResourceSliceInfo(ctx context.Context, kubeClient client.Client) ([]types.ResourceSliceInfo, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start collecting ResourceSlice info")

	var resourceSliceInfoList []types.ResourceSliceInfo

	resourceSliceList := &resourceapi.ResourceSliceList{}
	if err := kubeClient.List(ctx, resourceSliceList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list ResourceClaims: %v", err)
	}

	for _, rs := range resourceSliceList.Items {
		if hasBindingConditions(rs) {
			continue
		}

		var resourceSliceInfo types.ResourceSliceInfo

		resourceSliceInfo.Name = rs.Name
		resourceSliceInfo.CreationTimestamp = rs.CreationTimestamp
		resourceSliceInfo.Driver = rs.Spec.Driver
		resourceSliceInfo.NodeName = rs.Spec.NodeName
		resourceSliceInfo.Pool = rs.Spec.Pool.Name

		for _, device := range rs.Spec.Devices {
			if device.Basic != nil {
				var deviceInfo types.ResourceSliceDevice
				deviceInfo.Name = device.Name
				for attrName, attrValue := range device.Basic.Attributes {
					if attrName == "uuid" {
						deviceInfo.UUID = *attrValue.StringValue
					}
				}
				resourceSliceInfo.Devices = append(resourceSliceInfo.Devices, deviceInfo)
			}
		}

		resourceSliceInfoList = append(resourceSliceInfoList, resourceSliceInfo)
	}

	logger.V(1).Info("Finish collecting ResourceSlice info", "resourceSliceInfos", resourceSliceInfoList)

	return resourceSliceInfoList, nil
}

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

func processNodeInfo(nodes *v1.NodeList, composableDRASpec types.ComposableDRASpec) ([]types.NodeInfo, error) {
	var nodeInfoList []types.NodeInfo

	for _, node := range nodes.Items {
		var nodeInfo types.NodeInfo

		nodeInfo.Name = node.Name

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
				model, err := getModelName(composableDRASpec, deviceName, "")
				if err != nil {
					return nil, err
				}

				exit = false
				for i := range nodeInfo.Models {
					if nodeInfo.Models[i].DeviceName == deviceName {
						nodeInfo.Models[i].MaxDevice = max
						nodeInfo.Models[i].Model = model
						exit = true
						break
					}
				}

				if !exit {
					newModelConstraint := types.ModelConstraints{
						DeviceName: deviceName,
						Model:      model,
						MaxDevice:  max,
					}

					nodeInfo.Models = append(nodeInfo.Models, newModelConstraint)
				}
			} else if strings.HasSuffix(suffix, "-size-min") {
				min, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid integer in %s: %v", val, err)
				}

				deviceName := suffix[:len(suffix)-9]
				model, err := getModelName(composableDRASpec, deviceName, "")
				if err != nil {
					return nil, err
				}

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

func getModelName(composableDRASpec types.ComposableDRASpec, deviceName, productName string) (string, error) {
	if deviceName != "" {
		for _, deviceInfo := range composableDRASpec.DeviceInfos {
			if deviceInfo.K8sDeviceName == deviceName {
				return deviceInfo.CDIModelName, nil
			}
		}
	}

	if productName != "" {
		for _, deviceInfo := range composableDRASpec.DeviceInfos {
			if deviceInfo.DRAAttributes["productName"] == productName {
				return deviceInfo.CDIModelName, nil
			}
		}
	}

	return "", fmt.Errorf("unknown device name: %s", deviceName)
}

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

	composableDRASpec.LabelPrefix = configMap.Data["label-prefix"]

	if err := yaml.Unmarshal([]byte(configMap.Data["fabric-id-range"]), &composableDRASpec.FabricIDRange); err != nil {
		return composableDRASpec, fmt.Errorf("failed to parse fabric-id-range: %v", err)
	}

	logger.V(1).Info("Finish collecting ConfigMap info", "composableDRASpec", composableDRASpec)

	return composableDRASpec, nil
}

func hasConditionWithStatus(conditions []metav1.Condition, conditionType string, status metav1.ConditionStatus) bool {
	for _, c := range conditions {
		if c.Type == conditionType && c.Status == status {
			return true
		}
	}
	return false
}

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

func hasBindingConditions(rs resourceapi.ResourceSlice) bool {
	for _, device := range rs.Spec.Devices {
		if len(device.Basic.BindingConditions) > 0 {
			return true
		}
	}

	return false
}

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
