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
	"encoding/json"
	"fmt"
	"time"

	cdioperator "github.com/CoHDI/composable-resource-operator/api/v1alpha1"
	"github.com/CoHDI/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const maxRetries = 2

// patchNodeLabel patches the labels of a node.
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

// PatchComposableResourceAnnotation patches the annotation of a ComposableResource.
func PatchComposableResourceAnnotation(ctx context.Context, kubeClient client.Client, resourceName, key, value string) error {
	logger := ctrl.LoggerFrom(ctx)

	logger.Info("Start patch ComposableResource annotation",
		"name", resourceName,
		"key", key,
		"value", value)

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
			client.RawPatch(k8stypes.MergePatchType, patchBytes),
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

// PatchComposabilityRequestSize patches the size of a ComposabilityRequest.
func PatchComposabilityRequestSize(ctx context.Context, kubeClient client.Client, requestName string, count int64) error {
	logger := ctrl.LoggerFrom(ctx)

	logger.Info("Start patch ComposabilityRequest size",
		"name", requestName,
		"patchSize", count)

	var lastErr error

	for range maxRetries {
		existingCR := &cdioperator.ComposabilityRequest{}
		err := kubeClient.Get(
			ctx,
			k8stypes.NamespacedName{Name: requestName},
			existingCR,
		)
		if err != nil {
			return fmt.Errorf("failed to get ComposabilityRequest: %v", err)
		}

		patchOpts := []map[string]interface{}{
			{
				"op":    "replace",
				"path":  "/spec/resource/size",
				"value": count,
			},
		}

		patchBytes, err := json.Marshal(patchOpts)
		if err != nil {
			return fmt.Errorf("patch marshal error: %w", err)
		}

		if err := kubeClient.Patch(ctx, existingCR, client.RawPatch(k8stypes.JSONPatchType, patchBytes)); err != nil {
			if apierrors.IsConflict(err) {
				lastErr = err
				continue
			}
			return fmt.Errorf("failed to patch ComposabilityRequest: %v", err)
		}
		logger.V(1).Info("Successfully patched ComposabilityRequest size")
		return nil
	}
	return fmt.Errorf("max retries (%d) reached, last error: %v", maxRetries, lastErr)
}

// PatchResourceClaimDeviceConditions patches the device conditions of a ResourceClaim.
func PatchResourceClaimDeviceConditions(ctx context.Context, kubeClient client.Client, name, namespace, conditionType string) error {
	logger := ctrl.LoggerFrom(ctx)

	logger.Info("Start patch ResourceClaim DeviceConditions",
		"name", name,
		"namespace", namespace,
		"conditionType", conditionType)

	var lastErr error

	for range maxRetries {
		existingRC := &resourceapi.ResourceClaim{}
		err := kubeClient.Get(
			ctx,
			k8stypes.NamespacedName{Name: name, Namespace: namespace},
			existingRC,
		)
		if err != nil {
			return fmt.Errorf("failed to get ResourceClaim: %v", err)
		}

		if len(existingRC.Status.Devices) == 0 && existingRC.Status.Allocation != nil && len(existingRC.Status.Allocation.Devices.Results) > 0 {
			for _, allocationDevice := range existingRC.Status.Allocation.Devices.Results {
				device := &resourceapi.AllocatedDeviceStatus{
					Driver: allocationDevice.Driver,
					Device: allocationDevice.Device,
					Pool:   allocationDevice.Pool,
				}
				existingRC.Status.Devices = append(existingRC.Status.Devices, *device)
			}
		}

		if len(existingRC.Status.Devices) == 0 {
			logger.V(1).Info("No devices found in ResourceClaim status, skipping patch")
			return nil
		}

		for i := range existingRC.Status.Devices {
			device := &existingRC.Status.Devices[i]
			newCondition := metav1.Condition{
				Type:               conditionType,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now()),
				Reason:             "DeviceConditionUpdated",
				Message:            fmt.Sprintf("Device %s condition %s updated", device.Device, conditionType),
			}

			conditionExists := false
			for j, existingCond := range device.Conditions {
				if existingCond.Type == conditionType {
					if existingCond.Status != newCondition.Status {
						device.Conditions[j] = newCondition
					}
					conditionExists = true
					break
				}
			}

			if !conditionExists {
				device.Conditions = append(device.Conditions, newCondition)
			}
		}

		patchData := map[string]interface{}{
			"status": map[string]interface{}{
				"devices": existingRC.Status.Devices,
			},
		}
		patchBytes, err := json.Marshal(patchData)
		if err != nil {
			return fmt.Errorf("failed to marshal patch: %v", err)
		}

		err = kubeClient.Status().Patch(ctx, existingRC, client.RawPatch(k8stypes.MergePatchType, patchBytes))
		if err != nil {
			if apierrors.IsConflict(err) {
				lastErr = err
				continue
			}
			return fmt.Errorf("failed to patch ResourceClaim status: %v", err)
		}

		logger.V(1).Info("Successfully patched device conditions")
		return nil
	}
	return fmt.Errorf("max retries (%d) reached, last error: %v", maxRetries, lastErr)
}

// UpdateNodeLabel updates the labels of a node.
func UpdateNodeLabel(ctx context.Context, kubeClient client.Client, clientSet kubernetes.Interface, nodeName string, composableDRASpec types.ComposableDRASpec) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start updating Node label")

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

	logger.V(1).Info("Start patch Node label", "add labels", addLabels, "delete labels", deleteLabels)

	return patchNodeLabel(clientSet, nodeName, addLabels, deleteLabels)
}
