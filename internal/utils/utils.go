package utils

import (
	"context"
	"fmt"
	"strconv"
	"time"

	cdioperator "github.com/IBM/cdi-operator"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	nodes, err := clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
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
			nodeInfo.Max, _ = strconv.Atoi(MaxValue)
		}

		MinValue, exists := node.Labels["composable.test/nvidia-a100-80g-siz-min"]
		if exists {
			nodeInfo.Min, _ = strconv.Atoi(MinValue)
		}

	}

	return nodeInfoList, nil
}

func UpdateComposableResourceLastUsedTime(ctx context.Context, kubeClient client.Client) error {
	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("failed to list ComposableResourceList: %v", err)
	}

	for _, resource := range resourceList.Items {
		if resource.Status.State == "Online" {
			used, err := IsDeviceUsed(resource.Status.DeviceID)
			if err != nil {
				return err
			}
			if used {
				if resource.Annotations == nil {
					resource.Annotations = make(map[string]string)
				}

				currentTime := time.Now().Format(time.RFC3339)
				// TODO: get label-prefix from ConfigMap
				resource.Annotations["composable.test/last-used-time"] = currentTime
				if err := kubeClient.Update(ctx, &resource); err != nil {
					return fmt.Errorf("failed to update ComposableResource: %w", err)
				}

			}
		}
	}

	return nil
}

func IsDeviceUsed(uuid string) (bool, error) {

	return false, nil
}

func isDeviceAvailable() bool {
	return true
}
