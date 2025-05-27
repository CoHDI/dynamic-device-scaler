package utils

import (
	"context"
	"encoding/json"
	"fmt"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const maxRetries = 2

func patchNodeLabel(clientset kubernetes.Interface, nodeName string, addLabels, deleteLabels []string) error {
	var lastErr error

	labelsPatch := make(map[string]interface{})
	for _, label := range addLabels {
		labelsPatch[label] = "true"
	}
	for _, label := range deleteLabels {
		labelsPatch[label] = nil
	}

	patchBody := map[string]any{
		"metadata": map[string]any{
			"labels": labelsPatch,
		},
	}

	patchBytes, err := json.Marshal(patchBody)
	if err != nil {
		return fmt.Errorf("patch marshal error: %w", err)
	}

	for range maxRetries {
		_, err = clientset.CoreV1().Nodes().Patch(
			context.TODO(),
			nodeName,
			k8stypes.StrategicMergePatchType,
			patchBytes,
			metav1.PatchOptions{},
		)

		if err == nil {
			return nil
		}

		if apierrors.IsConflict(err) {
			lastErr = err
			continue
		}
		return fmt.Errorf("patch failed: %w", err)
	}

	return fmt.Errorf("max retries (%d) reached, last error: %v", maxRetries, lastErr)
}

func PatchComposableResourceAnnotation(ctx context.Context, kubeClient client.Client, resourceName, key, value string) error {
	var lastErr error

	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				key: value,
			},
		},
	}
	patchBytes, _ := json.Marshal(patch)

	for range maxRetries {
		currentCR := &cdioperator.ComposableResource{}
		if err := kubeClient.Get(
			ctx,
			k8stypes.NamespacedName{Name: resourceName},
			currentCR,
		); err != nil {
			return fmt.Errorf("failed to get latest ComposableResource: %w", err)
		}

		if currentCR.Annotations != nil {
			if currentVal, exists := currentCR.Annotations[key]; exists && currentVal == value {
				return nil
			}
		}

		err := kubeClient.Patch(
			ctx,
			&cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
			},
			client.RawPatch(k8stypes.StrategicMergePatchType, patchBytes),
		)

		if err == nil {
			return nil
		}

		if apierrors.IsConflict(err) {
			lastErr = err
			continue
		}
		return fmt.Errorf("failed to patch ComposableResource: %w", err)
	}

	return fmt.Errorf("max retries (%d) reached, last error: %v", maxRetries, lastErr)
}

func PatchComposabilityRequestSize(ctx context.Context, kubeClient client.Client, cr *cdioperator.ComposabilityRequest, count int64) error {
	var lastErr error

	for range maxRetries {
		existingCR := &cdioperator.ComposabilityRequest{}
		err := kubeClient.Get(
			ctx,
			k8stypes.NamespacedName{Name: cr.Name},
			existingCR,
		)
		if err != nil {
			return fmt.Errorf("failed to get ComposabilityRequest: %v", err)
		}

		modifiedCR := existingCR.DeepCopy()
		modifiedCR.Spec.Resource.Size = count

		patch := client.StrategicMergeFrom(existingCR.DeepCopy())
		if err := kubeClient.Patch(ctx, modifiedCR, patch); err != nil {
			if apierrors.IsConflict(err) {
				lastErr = err
				continue
			}
			return fmt.Errorf("failed to patch ComposabilityRequest: %v", err)
		}
		return nil
	}
	return fmt.Errorf("max retries (%d) reached, last error: %v", maxRetries, lastErr)
}

func UpdateNodeLabel(ctx context.Context, kubeClient client.Client, clientSet kubernetes.Interface, nodeName string, composableDRASpec types.ComposableDRASpec) error {
	var installedDevices []string

	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := kubeClient.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("failed to list composabilityRequestList: %v", err)
	}

	for _, cr := range composabilityRequestList.Items {
		if cr.Spec.Resource.TargetNode == nodeName {
			if cr.Spec.Resource.Size > 0 {
				if notIn(cr.Spec.Resource.Model, installedDevices) {
					installedDevices = append(installedDevices, cr.Spec.Resource.Model)
				}
			}
		}
	}

	resourceList := &cdioperator.ComposableResourceList{}
	if err := kubeClient.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("failed to list ComposableResourceList: %v", err)
	}

	for _, rs := range resourceList.Items {
		if rs.Spec.TargetNode == nodeName {
			if rs.Status.State == "Online" {
				if notIn(rs.Spec.Model, installedDevices) {
					installedDevices = append(installedDevices, rs.Spec.Model)
				}
			}
		}
	}

	var addLabels, deleteLabels []string
	var notCoexistID []int

	for _, device := range installedDevices {
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

	return patchNodeLabel(clientSet, nodeName, addLabels, deleteLabels)
}
