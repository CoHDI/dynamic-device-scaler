package utils

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	cdioperator "github.com/IBM/cdi-operator"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetClusterInfo(ctx context.Context) {

}

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
			// Determine the state based on the added claim.status.Devices[].BindingConditions in https://github.com/kubernetes/kubernetes/pull/130160
			deviceInfo.State = "Preparing"
			resourceClaimInfo.Devices = append(resourceClaimInfo.Devices, deviceInfo)
		}

		// TODO: more judgment required
		if len(rc.Status.ReservedFor) > 0 {
			resourceClaimInfo.UsedByPod = true
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

func GetNodeInfo(ctx context.Context, clientSet *kubernetes.Clientset) ([]types.NodeInfo, error) {
	var nodeInfoList []types.NodeInfo

	nodes, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Nodes: %v", err)
	}

	for _, node := range nodes.Items {
		var nodeInfo types.NodeInfo

		nodeInfo.Name = node.Name

		// TODO: get label-prefix from ConfigMap
		fabricIDValue, exists := node.Labels["composable.test/fabric"]
		if exists {
			nodeInfo.FabricID = fabricIDValue
		}

		MaxValue, exists := node.Labels["composable.test/nvidia-a100-80g-siz-max"]
		if exists {
			nodeInfo.MaxDevice, _ = strconv.Atoi(MaxValue)
		}

		MinValue, exists := node.Labels["composable.test/nvidia-a100-80g-siz-min"]
		if exists {
			nodeInfo.MinDevice, _ = strconv.Atoi(MinValue)
		}

	}

	return nodeInfoList, nil
}

func GetConfigMapInfo(ctx context.Context, clientSet *kubernetes.Clientset) (*types.ComposableDRASpec, error) {
	var spec types.ComposableDRASpec

	//TODO: fix ns and name
	configMap, err := clientSet.CoreV1().ConfigMaps("default").Get(ctx, "composable-dra-dds", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap: %v", err)
	}

	if err = yaml.Unmarshal([]byte(configMap.Data["device-info"]), &spec.DeviceInfo); err != nil {
		return nil, fmt.Errorf("failed to parse device-info: %v", err)
	}

	spec.LabelPrefix = configMap.Data["label-prefix"]

	if err := yaml.Unmarshal([]byte(configMap.Data["fabric-id-range"]), &spec.FabricIDRange); err != nil {
		return nil, fmt.Errorf("fabric-id-range parsing failed: %v", err)
	}

	//TODO: add validation for spec

	return &spec, nil
}

func GetComposablityRequestInfo(ctx context.Context, kubeClient client.Client) (*cdioperator.ComposabilityRequestList, error) {
	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := kubeClient.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return nil, err
	}

	return composabilityRequestList, nil
}

func UpdateComposableResourceLastUsedTime(ctx context.Context, kubeClient client.Client, resourceSliceInfoList []types.ResourceSliceInfo, resourceClaimInfoList []types.ResourceClaimInfo) error {
	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("failed to list ComposableResourceList: %v", err)
	}

	for _, resource := range resourceList.Items {
		if resource.Status.State != "Online" {
			continue
		}

		var deviceName string

		found := false
	ResourceSliceLoop:
		for _, rs := range resourceSliceInfoList {
			for _, device := range rs.Devices {
				if device.UUID == resource.Status.DeviceID {
					found = true
					deviceName = device.Name
					break ResourceSliceLoop
				}
			}
		}

		if !found {
			continue
		}

	ResourceLoop:
		for _, rc := range resourceClaimInfoList {
			for _, device := range rc.Devices {
				if device.Name == deviceName {
					if resource.Annotations == nil {
						resource.Annotations = make(map[string]string)
					}

					currentTime := time.Now().Format(time.RFC3339)
					// TODO: get label-prefix from ConfigMap
					resource.Annotations["composable.test/last-used-time"] = currentTime
					if err := kubeClient.Update(ctx, &resource); err != nil {
						return fmt.Errorf("failed to update ComposableResource: %w", err)
					}
					break ResourceLoop
				}
			}
		}
	}

	return nil
}

func ResendFailed(node types.NodeInfo, resourceClaimInfoList []types.ResourceClaimInfo) error {
	sortByTime(resourceClaimInfoList)

	return nil
}

func ResendSchedule(node types.NodeInfo, resourceClaimInfoList []types.ResourceClaimInfo) error {

	return nil
}

func sortByTime(resourceClaims []types.ResourceClaimInfo) {
	sort.Slice(resourceClaims, func(i, j int) bool {
		return resourceClaims[i].CreationTimestamp.After(resourceClaims[j].CreationTimestamp.Time)
	})
}

// hasDeviceConflict determine whether there is a conflict in the device on the node
func hasDeviceConflict(ctx context.Context, kubeClient client.Client, node types.NodeInfo, resourceClaims []types.ResourceClaimInfo) (bool, error) {
	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := kubeClient.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return true, err
	}

outerLoop:
	for _, rc := range resourceClaims {
		for i, rcDevice := range rc.Devices {
			if rcDevice.State == "Preparing" {
				for j, otherDevice := range rc.Devices {
					if i != j && rcDevice.Model != otherDevice.Model {
						if !isDeviceCoexistence(rcDevice.Model, otherDevice.Model) {
							setDevicesFailed(ctx, kubeClient, &rc)
							continue outerLoop
						}
					}
				}

				for _, request := range composabilityRequestList.Items {
					if rcDevice.Model == request.Spec.Resource.Model {
						if request.Spec.Resource.Size > node.MaxDevice {
							setDevicesFailed(ctx, kubeClient, &rc)
							continue outerLoop
						}
					} else if request.Spec.Resource.Size > 0 {
						if !isDeviceCoexistence(rcDevice.Model, request.Spec.Resource.Model) {
							setDevicesFailed(ctx, kubeClient, &rc)
							continue outerLoop
						}
					}
				}

				for _, rc2 := range resourceClaims {
					if rc.Name != rc2.Name {
						for _, rc2Device := range rc2.Devices {
							if rc2Device.State == "Preparing" && rcDevice.Model != rc2Device.Model {
								if !isDeviceCoexistence(rcDevice.Model, rc2Device.Model) {
									setDevicesFailed(ctx, kubeClient, &rc)
									continue outerLoop
								}
							}
						}
					}
				}
			}
		}
	}

	return false, nil
}

// isDeviceCoexistence determines whether two devices can coexist based on the content in ConfigMap
func isDeviceCoexistence(device1, device2 string) bool {
	return true
}

func setDevicesFailed(ctx context.Context, kubeClient client.Client, resourceClaim *types.ResourceClaimInfo) error {
	for _, device := range resourceClaim.Devices {
		if device.State == "Preparing" {
			device.State = "Failed"
		}
	}

	namespacedName := k8stypes.NamespacedName{
		Name:      resourceClaim.Name,
		Namespace: resourceClaim.Namespace,
	}
	var rc resourceapi.ResourceClaim

	if err := kubeClient.Get(ctx, namespacedName, &rc); err != nil {
		return fmt.Errorf("failed to get ResourceClaim: %v", err)
	}

	newCondition := metav1.Condition{
		Type:   "FabricDeviceFailed",
		Status: metav1.ConditionTrue,
	}

	for _, device := range rc.Status.Devices {
		device.Conditions = append(device.Conditions, newCondition)
	}

	return nil
}
