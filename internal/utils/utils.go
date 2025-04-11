package utils

import (
	"context"
	"fmt"
	"time"

	cdioperator "github.com/IBM/cdi-operator"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1beta1"
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
			var deviceInfo types.Device
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
