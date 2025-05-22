package utils

import (
	"context"
	"encoding/json"
	"fmt"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const maxRetries = 2

func PatchNodeLabel(clientset kubernetes.Interface, nodeName string, key, value string) error {
	var lastErr error

	patchBody := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]string{
				key: value,
			},
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
			types.StrategicMergePatchType,
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
			types.NamespacedName{Name: resourceName},
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
			&cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
			},
			client.RawPatch(types.StrategicMergePatchType, patchBytes),
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
			types.NamespacedName{Name: cr.Name},
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
