package utils

import (
	"context"
	"fmt"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetConfiguredDeviceCount(ctx context.Context, kubeClient client.Client, resourceClaims []types.ResourceClaimInfo) (map[string]int, error) {
	deviceCount := make(map[string]int)

	for _, rc := range resourceClaims {
		isRed, err := isResourceSliceRed(ctx, kubeClient, rc.Name)
		if err != nil {
			return deviceCount, err
		}
		for _, device := range rc.Devices {
			if device.State != "Preparing" {
				continue
			}

			deviceCount[device.Model]++

			if isRed && device.UsedByPod {
				deviceCount[device.Model]++
			}
		}
	}

	return deviceCount, nil
}

func DynamicAttach(ctx context.Context, kubeClient client.Client, cr *cdioperator.ComposabilityRequest, count int64, model, nodeName string) error {
	if cr == nil {
		return createNewComposabilityRequestCR(ctx, kubeClient, count, model, nodeName)
	}

	return updateComposabilityRequestCR(ctx, kubeClient, cr, count)
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

func updateComposabilityRequestCR(ctx context.Context, kubeClient client.Client, cr *cdioperator.ComposabilityRequest, count int64) error {
	existingCR := &cdioperator.ComposabilityRequest{}
	err := kubeClient.Get(ctx, k8stypes.NamespacedName{Name: cr.Name}, existingCR)
	if err != nil {
		return fmt.Errorf("failed to get ComposabilityRequest: %v", err)
	}

	patch := client.MergeFrom(existingCR.DeepCopy())
	existingCR.Spec.Resource.Size = count

	if err := kubeClient.Patch(ctx, existingCR, patch); err != nil {
		return fmt.Errorf("failed to patch ComposabilityRequest: %v", err)
	}

	return nil
}

func DynamicDetach(ctx context.Context, kubeClient client.Client, cr *cdioperator.ComposabilityRequest, count int64) error {
	if count < cr.Spec.Resource.Size {
		nextSize, err := getNextSize(ctx, kubeClient, count)
		if err != nil {
			return fmt.Errorf("failed to get next size: %v", err)
		}

		if nextSize < cr.Spec.Resource.Size {
			return updateComposabilityRequestCR(ctx, kubeClient, cr, nextSize)
		}
	}

	return nil
}

func getNextSize(ctx context.Context, kubeClient client.Client, count int64) (int64, error) {
	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return 0, fmt.Errorf("failed to list ComposableResourceList: %v", err)
	}

	var resourceCount int64
	for _, resource := range resourceList.Items {
		if (resource.Status.State == "Online" || resource.Status.State == "Attaching") && resource.DeletionTimestamp != nil {
			over, err := isLastUsedOverMinute(resource)
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
