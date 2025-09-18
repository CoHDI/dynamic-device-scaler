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

package controller

import (
	"context"
	"fmt"
	"time"

	cdioperator "github.com/CoHDI/composable-resource-operator/api/v1alpha1"
	"github.com/CoHDI/dynamic-device-scaler/internal/types"
	"github.com/CoHDI/dynamic-device-scaler/internal/utils"

	resourceapi "k8s.io/api/resource/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// ResourceMonitorReconciler reconciles a ResourceMonitor object
type ResourceMonitorReconciler struct {
	client.Client
	ClientSet          kubernetes.Interface
	Scheme             *runtime.Scheme
	ScanInterval       time.Duration
	DeviceNoRemoval    time.Duration
	DeviceNoAllocation time.Duration
}

//+kubebuilder:rbac:groups=resource.k8s.io,resources=resourceclaims,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=resource.k8s.io,resources=resourceclaims/status,verbs=get;update;patch

//+kubebuilder:rbac:groups=resource.k8s.io,resources=resourceslices,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=resource.k8s.io,resources=resourceslices/status,verbs=get;update;patch

//+kubebuilder:rbac:groups=cro.hpsys.ibm.ie.com,resources=composabilityrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cro.hpsys.ibm.ie.com,resources=composabilityrequests/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=cro.hpsys.ibm.ie.com,resources=composableresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cro.hpsys.ibm.ie.com,resources=composableresources/status,verbs=get;update;patch

//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get

//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;patch;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceMonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := ctrl.Log.WithName("DDS")
	ctx = ctrl.LoggerInto(ctx, reqLogger)

	reqLogger.Info("Start reconcile")

	resourceClaimInfos, resourceSliceInfos, nodeInfos, composableDRASpec, err := r.collectInfo(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.updateComposableResourceLastUsedTime(ctx, resourceSliceInfos, composableDRASpec.LabelPrefix)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.handleNodes(ctx, nodeInfos, resourceClaimInfos, resourceSliceInfos, composableDRASpec)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.ScanInterval}, err
}

// collectInfo gathers information about resource claims, resource slices, nodes, and composable DRA specifications.
func (r *ResourceMonitorReconciler) collectInfo(ctx context.Context) ([]types.ResourceClaimInfo, []types.ResourceSliceInfo, []types.NodeInfo, types.ComposableDRASpec, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Start collecting information")

	var composableDRASpec types.ComposableDRASpec

	composableDRASpec, err := utils.GetConfigMapInfo(ctx, r.ClientSet)
	if err != nil {
		return nil, nil, nil, composableDRASpec, fmt.Errorf("failed to get ComposableDRASpec from ConfigMap: %v", err)
	}

	resourceClaimInfos, err := utils.GetResourceClaimInfo(ctx, r.Client, composableDRASpec)
	if err != nil {
		return nil, nil, nil, composableDRASpec, fmt.Errorf("failed to get ResourceClaimInfo: %v", err)
	}

	resourceSliceInfos, err := utils.GetResourceSliceInfo(ctx, r.Client)
	if err != nil {
		return nil, nil, nil, composableDRASpec, fmt.Errorf("failed to get ResourceSliceInfo: %v", err)
	}

	nodeInfos, err := utils.GetNodeInfo(ctx, r.ClientSet, composableDRASpec)
	if err != nil {
		return nil, nil, nil, composableDRASpec, fmt.Errorf("failed to get NodeInfo: %v", err)
	}

	return resourceClaimInfos, resourceSliceInfos, nodeInfos, composableDRASpec, nil
}

// updateComposableResourceLastUsedTime updates the last used time of ComposableResources based on their usage.
func (r *ResourceMonitorReconciler) updateComposableResourceLastUsedTime(ctx context.Context, resourceSliceInfos []types.ResourceSliceInfo, labelPrefix string) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Start updating ComposableResource last used time")

	resourceList := &cdioperator.ComposableResourceList{}
	if err := r.List(ctx, resourceList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("failed to list ComposableResourceList: %v", err)
	}

	for _, resource := range resourceList.Items {
		if resource.Status.State == "Online" {
			isRed, resourceSliceInfo, deviceName := utils.IsDeviceResourceSliceRed(resource.Status.DeviceID, resourceSliceInfos)
			if isRed {
				isUsed, err := utils.IsDeviceUsedByPod(ctx, r.Client, deviceName, *resourceSliceInfo)
				if err != nil {
					return fmt.Errorf("failed to check if device is used by pod: %w", err)
				}
				if isUsed {
					currentTime := time.Now().Format(time.RFC3339)
					if err := utils.PatchComposableResourceAnnotation(ctx, r.Client, resource.Name, labelPrefix+"/last-used-time", currentTime); err != nil {
						return fmt.Errorf("failed to update ComposableResource: %w", err)
					}
				}
			}
		}
	}

	return nil
}

// handleNodes processes the node and updates the resource claims and slices accordingly.
func (r *ResourceMonitorReconciler) handleNodes(ctx context.Context, nodeInfos []types.NodeInfo, resourceClaimInfos []types.ResourceClaimInfo, resourceSliceInfos []types.ResourceSliceInfo, composableDRASpec types.ComposableDRASpec) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Start handling nodes")

	var err error
	for _, nodeInfo := range nodeInfos {
		var nodeResourceClaimInfos []types.ResourceClaimInfo

		for _, resourceClaimInfo := range resourceClaimInfos {
			if resourceClaimInfo.NodeName == nodeInfo.Name {
				nodeResourceClaimInfos = append(nodeResourceClaimInfos, resourceClaimInfo)
			}
		}

		newLogger := logger.WithValues("nodeName", nodeInfo.Name)
		ctx = ctrl.LoggerInto(ctx, newLogger)

		nodeResourceClaimInfos, err = utils.RescheduleFailedNotification(ctx, r.Client, nodeInfo, nodeResourceClaimInfos, resourceSliceInfos, composableDRASpec)
		if err != nil {
			return fmt.Errorf("failed to reschedule failed notification: %v", err)
		}

		nodeResourceClaimInfos, err = utils.RescheduleNotification(ctx, r.Client, nodeResourceClaimInfos, resourceSliceInfos, composableDRASpec.LabelPrefix, r.DeviceNoAllocation)
		if err != nil {
			return fmt.Errorf("failed to reschedule notification: %v", err)
		}

		err = r.handleDevices(ctx, nodeInfo, nodeResourceClaimInfos, resourceSliceInfos, composableDRASpec)
		if err != nil {
			return fmt.Errorf("failed to handle devices: %v", err)
		}

		err = utils.UpdateNodeLabel(ctx, r.Client, r.ClientSet, nodeInfo.Name, composableDRASpec)
		if err != nil {
			return fmt.Errorf("failed to update node label: %v", err)
		}
	}

	return nil
}

// handleDevices processes the device and updates the resource claims and slices accordingly.
func (r *ResourceMonitorReconciler) handleDevices(ctx context.Context, nodeInfo types.NodeInfo, resourceClaimInfos []types.ResourceClaimInfo, resourceSliceInfos []types.ResourceSliceInfo, composableDRASpec types.ComposableDRASpec) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(1).Info("Start handling node devices")

	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := r.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("failed to list ComposabilityRequestList: %v", err)
	}

	var actualCount int64
	var requestExit bool
	for _, device := range composableDRASpec.DeviceInfos {
		requestExit = false

		newLogger := logger.WithValues("deviceModel", device.CDIModelName)
		ctx = ctrl.LoggerInto(ctx, newLogger)

		cofiguredDeviceCount, err := utils.GetConfiguredDeviceCount(ctx, r.Client, device.CDIModelName, nodeInfo.Name, resourceClaimInfos, resourceSliceInfos)
		if err != nil {
			return fmt.Errorf("failed to get configured device count: %v", err)
		}

		logger.V(1).Info("Configured devices count", "count", cofiguredDeviceCount)

		maxCountLimit, minCountLimit := utils.GetModelLimit(nodeInfo, device.CDIModelName)
		if cofiguredDeviceCount > maxCountLimit {
			return fmt.Errorf("configured device count %d exceeds max limit %d", cofiguredDeviceCount, maxCountLimit)
		}

		if cofiguredDeviceCount < minCountLimit {
			cofiguredDeviceCount = minCountLimit
		}

		logger.V(1).Info("Actual cofiguredDeviceCount", "count", cofiguredDeviceCount)

		for _, cr := range composabilityRequestList.Items {
			if cr.Spec.Resource.Model == device.CDIModelName && cr.Spec.Resource.TargetNode == nodeInfo.Name {
				actualCount = cr.Spec.Resource.Size
				if cofiguredDeviceCount > actualCount {
					err := utils.DynamicAttach(ctx, r.Client, &cr, cofiguredDeviceCount, cr.Spec.Resource.Type, device.CDIModelName, nodeInfo.Name)
					if err != nil {
						return fmt.Errorf("failed to attach devices: %v", err)
					}
				} else if cofiguredDeviceCount < actualCount {
					err := utils.DynamicDetach(ctx, r.Client, &cr, cofiguredDeviceCount, nodeInfo.Name, composableDRASpec.LabelPrefix, r.DeviceNoRemoval)
					if err != nil {
						return fmt.Errorf("failed to detach devices: %v", err)
					}
				}
				requestExit = true
				break
			}
		}

		if !requestExit && cofiguredDeviceCount > 0 {
			resourceType := utils.GetDriverType(device.DriverName)
			err := utils.DynamicAttach(ctx, r.Client, nil, cofiguredDeviceCount, resourceType, device.CDIModelName, nodeInfo.Name)
			if err != nil {
				return fmt.Errorf("failed to attach devices: %v", err)
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceMonitorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	eventHandler := handler.EnqueueRequestForObject{}

	return ctrl.NewControllerManagedBy(mgr).
		Watches(&resourceapi.ResourceClaim{}, &eventHandler).
		Watches(&resourceapi.ResourceSlice{}, &eventHandler).
		Named("resourcemonitor").
		Complete(r)
}
