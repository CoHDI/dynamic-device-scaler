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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetResourceClaimInfo(ctx context.Context, kubeClient client.Client) ([]types.ResourceClaimInfo, error) {
	var resourceClaimInfoList []types.ResourceClaimInfo

	resourceClaimList := &resourceapi.ResourceClaimList{}
	if err := kubeClient.List(ctx, resourceClaimList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list ResourceClaims: %v", err)
	}

	for _, rc := range resourceClaimList.Items {
		var resourceClaimInfo types.ResourceClaimInfo
		resourceClaimInfo.Name = rc.Name
		resourceClaimInfo.Namespace = rc.Namespace
		resourceClaimInfo.CreationTimestamp = rc.ObjectMeta.CreationTimestamp
		for _, device := range rc.Status.Devices {
			var deviceInfo types.ResourceClaimDevice
			deviceInfo.Name = device.Device

			// waiting for PR https://github.com/kubernetes/kubernetes/pull/130160 to be merged
			// Determine the state based on the added claim.status.Devices[].BindingConditions in https://github.com/kubernetes/kubernetes/pull/130160123
			if device.BindingConditions != nil {
				if device.Conditions != nil {
					if device.Conditions[0].Type == "FabricDeviceReschedule" && device.Conditions[0].Status == "True" {
						deviceInfo.State = "Reschedule"
					} else if device.Conditions[0].Type == "FabricDeviceFailed" && device.Conditions[0].Status == "True" {
						deviceInfo.State = "Failed"
					} else {
						deviceInfo.State = "Preparing"
					}
				}
			}

			// TODO: more judgment required
			if len(rc.Status.ReservedFor) > 0 {
				deviceInfo.UsedByPod = true
			}
			resourceClaimInfo.Devices = append(resourceClaimInfo.Devices, deviceInfo)
		}

		resourceClaimInfoList = append(resourceClaimInfoList, resourceClaimInfo)
	}

	return resourceClaimInfoList, nil
}

func GetResourceSliceInfo(ctx context.Context, kubeClient client.Client) ([]types.ResourceSliceInfo, error) {
	var resourceSliceInfoList []types.ResourceSliceInfo

	resourceSliceList := &resourceapi.ResourceSliceList{}
	if err := kubeClient.List(ctx, resourceSliceList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list ResourceClaims: %v", err)
	}

	for _, rs := range resourceSliceList.Items {
		var resourceSliceInfo types.ResourceSliceInfo

		resourceSliceInfo.Name = rs.Name
		resourceSliceInfo.CreationTimestamp = rs.CreationTimestamp
		resourceSliceInfo.Driver = rs.Spec.Driver
		resourceSliceInfo.NodeName = rs.Spec.NodeName

		if rs.Spec.NodeSelector != nil {
			for _, term := range rs.Spec.NodeSelector.NodeSelectorTerms {
				for _, expr := range term.MatchExpressions {
					switch expr.Key {
					case "fabric":
						if len(expr.Values) > 0 {
							resourceSliceInfo.FabricID = expr.Values[0]
						}
						//TODO: get more information from the fabric
					}
				}
			}
		}

		for _, device := range rs.Spec.Devices {
			if device.Basic != nil {
				var deviceInfo types.ResourceSliceDevice
				deviceInfo.Name = device.Name
				for attrName, attrValue := range device.Basic.Attributes {
					if attrName == "uuid" {
						deviceInfo.UUID = attrValue.String()
					}
				}
				resourceSliceInfo.Devices = append(resourceSliceInfo.Devices, deviceInfo)
			}
		}
	}

	return resourceSliceInfoList, nil
}

func GetNodeInfo(ctx context.Context, clientSet *kubernetes.Clientset, composableDRASpec types.ComposableDRASpec) ([]types.NodeInfo, error) {
	nodes, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Nodes: %v", err)
	}

	return processNodeInfo(nodes, composableDRASpec)
}

func processNodeInfo(nodes *v1.NodeList, composableDRASpec types.ComposableDRASpec) ([]types.NodeInfo, error) {
	var nodeInfoList []types.NodeInfo

	for _, node := range nodes.Items {
		var nodeInfo types.NodeInfo

		nodeInfo.Name = node.Name

		fabricIDValue, exists := node.Labels[composableDRASpec.LabelPrefix+"/fabric"]
		if exists {
			nodeInfo.FabricID = fabricIDValue
		}

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

				exit = false
				for _, modelConstraint := range nodeInfo.Models {
					if modelConstraint.DeviceName == deviceName {
						modelConstraint.MaxDevice = max
						modelConstraint.Model = model
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
				model, err := getModelName(composableDRASpec, deviceName)
				if err != nil {
					return nil, err
				}

				exit = false
				for _, modelConstraint := range nodeInfo.Models {
					if modelConstraint.DeviceName == deviceName {
						modelConstraint.MinDevice = min
						modelConstraint.Model = model
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

func getModelName(composableDRASpec types.ComposableDRASpec, deviceName string) (string, error) {
	for _, deviceInfo := range composableDRASpec.DeviceInfos {
		if deviceInfo.K8sDeviceName == deviceName {
			return deviceInfo.CDIModelName, nil
		}
	}

	return "", fmt.Errorf("unknown device name: %s", deviceName)
}

func GetConfigMapInfo(ctx context.Context, clientSet *kubernetes.Clientset) (types.ComposableDRASpec, error) {
	var composableDRASpec types.ComposableDRASpec

	//TODO: fix ns and name
	configMap, err := clientSet.CoreV1().ConfigMaps("composable-dra").Get(ctx, "composable-dra-dds", metav1.GetOptions{})
	if err != nil {
		return composableDRASpec, fmt.Errorf("failed to get ConfigMap: %v", err)
	}

	if err = yaml.Unmarshal([]byte(configMap.Data["device-info"]), &composableDRASpec.DeviceInfos); err != nil {
		return composableDRASpec, fmt.Errorf("failed to parse device-info: %v", err)
	}

	composableDRASpec.LabelPrefix = configMap.Data["label-prefix"]

	if err := yaml.Unmarshal([]byte(configMap.Data["fabric-id-range"]), &composableDRASpec.FabricIDRange); err != nil {
		return composableDRASpec, fmt.Errorf("fabric-id-range parsing failed: %v", err)
	}

	//TODO: add validation for spec

	return composableDRASpec, nil
}
