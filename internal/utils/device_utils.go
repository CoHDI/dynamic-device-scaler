package utils

import (
	"context"
	"fmt"
	"time"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetConfiguredDeviceCount(ctx context.Context, kubeClient client.Client, model string, resourceClaimInfos []types.ResourceClaimInfo, resourceSliceInfos []types.ResourceSliceInfo) (int64, error) {
	preparingDeviceCount := getPreparingDevicesCount(resourceClaimInfos, model)

	podAllocatedDevicesCount, err := getPodAllocatedDevicesCount(ctx, kubeClient, model, resourceSliceInfos)
	if err != nil {
		return 0, err
	}

	return preparingDeviceCount + podAllocatedDevicesCount, nil
}

func getPreparingDevicesCount(resourceClaimInfos []types.ResourceClaimInfo, model string) int64 {
	var count int64

	for _, rc := range resourceClaimInfos {
		for _, device := range rc.Devices {
			if device.Model == model {
				if device.State == "Preparing" {
					count++
				}
			}
		}
	}

	return count
}

func getPodAllocatedDevicesCount(ctx context.Context, kubeClient client.Client, model string, resourceSliceInfos []types.ResourceSliceInfo) (int64, error) {
	var count int64

	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := kubeClient.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return count, fmt.Errorf("failed to list composabilityRequestList: %v", err)
	}

	for _, request := range composabilityRequestList.Items {
		if request.Spec.Resource.Model == model {
			for _, resource := range request.Status.Resources {
				if resource.State == "Online" {
					isRed, resourceSliceInfo := IsDeviceResourceSliceRed(resource.DeviceIDUUID, resourceSliceInfos)
					if isRed {
						isUsed, err := IsDeviceUsedByPod(ctx, kubeClient, resource.DeviceIDUUID, *resourceSliceInfo)
						if err != nil {
							return count, err
						}
						if isUsed {
							count++
						}
					}
				}
			}
		}
	}

	return count, nil
}

func IsDeviceUsedByPod(ctx context.Context, kubeClient client.Client, deviceID string, resourceSliceInfo types.ResourceSliceInfo) (bool, error) {
	resourceClaimList := &resourceapi.ResourceClaimList{}
	if err := kubeClient.List(ctx, resourceClaimList, &client.ListOptions{}); err != nil {
		return false, fmt.Errorf("failed to list ResourceClaims: %v", err)
	}

	for _, resourceSliceDevice := range resourceSliceInfo.Devices {
		if resourceSliceDevice.UUID == deviceID {
			for _, resourceClaim := range resourceClaimList.Items {
				for _, resourceClaimDevice := range resourceClaim.Status.Devices {
					if resourceSliceInfo.Pool == resourceClaimDevice.Pool &&
						resourceSliceInfo.Driver == resourceClaimDevice.Driver &&
						resourceSliceDevice.Name == resourceClaimDevice.Device {
						if resourceClaim.Status.ReservedFor != nil {
							if resourceClaim.Status.ReservedFor[0].Resource == "pods" {
								return true, nil
							}
						}
					}
				}
			}
		}
	}

	return false, nil
}

func IsDeviceResourceSliceRed(deviceID string, resourceSliceInfos []types.ResourceSliceInfo) (bool, *types.ResourceSliceInfo) {
	for _, resourceSlice := range resourceSliceInfos {
		for _, resourceSliceDevice := range resourceSlice.Devices {
			if resourceSliceDevice.UUID == deviceID {
				return resourceSlice.State == types.ResourceSliceStateRed, &resourceSlice
			}
		}
	}

	return false, nil
}

func DynamicAttach(ctx context.Context, kubeClient client.Client, cr *cdioperator.ComposabilityRequest, count int64, model, nodeName string) error {
	if cr == nil {
		return createNewComposabilityRequestCR(ctx, kubeClient, count, model, nodeName)
	}

	return PatchComposabilityRequestSize(ctx, kubeClient, cr, count)
}

func createNewComposabilityRequestCR(ctx context.Context, kubeClient client.Client, count int64, model, node string) error {
	newCR := &cdioperator.ComposabilityRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "composability-",
		},
		Spec: cdioperator.ComposabilityRequestSpec{
			Resource: cdioperator.ScalarResourceDetails{
				Type:       "gpu",
				Model:      model,
				Size:       count,
				TargetNode: node,
			},
		},
	}

	if err := kubeClient.Create(ctx, newCR); err != nil {
		return fmt.Errorf("failed to create ComposabilityRequest: %v", err)
	}

	return nil
}

func DynamicDetach(ctx context.Context, kubeClient client.Client, cr *cdioperator.ComposabilityRequest, count int64, labelPrefix string, deviceNoAllocation time.Duration) error {
	if count < cr.Spec.Resource.Size {
		nextSize, err := getNextSize(ctx, kubeClient, count, labelPrefix, deviceNoAllocation)
		if err != nil {
			return fmt.Errorf("failed to get next size: %v", err)
		}

		if nextSize < cr.Spec.Resource.Size {
			return PatchComposabilityRequestSize(ctx, kubeClient, cr, nextSize)
		}
	}

	return nil
}

func getNextSize(ctx context.Context, kubeClient client.Client, count int64, labelPrefix string, deviceNoAllocation time.Duration) (int64, error) {
	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return 0, fmt.Errorf("failed to list ComposableResourceList: %v", err)
	}

	var resourceCount int64
	for _, resource := range resourceList.Items {
		if (resource.Status.State == "Online" || resource.Status.State == "Attaching") && resource.DeletionTimestamp != nil {
			over, err := isLastUsedOverTime(resource, labelPrefix, deviceNoAllocation)
			if err != nil || !over {
				continue
			}
			resourceCount++
		}
	}

	if count > resourceCount {
		return count, nil
	}

	return resourceCount, nil
}
