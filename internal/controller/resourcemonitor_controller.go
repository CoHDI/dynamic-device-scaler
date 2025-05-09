/*
Copyright 2025.

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

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	"github.com/InfraDDS/dynamic-device-scaler/internal/utils"
	"github.com/go-logr/logr"
	resourceapi "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ResourceMonitorReconciler reconciles a ResourceMonitor object
type ResourceMonitorReconciler struct {
	client.Client
	ClientSet          *kubernetes.Clientset
	Log                logr.Logger
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
	_ = logf.FromContext(ctx)

	resourceClaimInfos, resourceSliceInfos, nodeInfos, composableDRASpec, err := r.collectInfo(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.updateComposableResourceLastUsedTime(ctx, resourceSliceInfos, resourceClaimInfos, composableDRASpec.LabelPrefix)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.handleNodes(ctx, nodeInfos, resourceClaimInfos, composableDRASpec)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.ScanInterval}, err
}

func (r *ResourceMonitorReconciler) collectInfo(ctx context.Context) ([]types.ResourceClaimInfo, []types.ResourceSliceInfo, []types.NodeInfo, types.ComposableDRASpec, error) {
	var composableDRASpec types.ComposableDRASpec

	resourceClaimInfos, err := utils.GetResourceClaimInfo(ctx, r.Client)
	if err != nil {
		return nil, nil, nil, composableDRASpec, err
	}

	resourceSliceInfos, err := utils.GetResourceSliceInfo(ctx, r.Client)
	if err != nil {
		return nil, nil, nil, composableDRASpec, err
	}

	composableDRASpec, err = utils.GetConfigMapInfo(ctx, r.ClientSet)
	if err != nil {
		return nil, nil, nil, composableDRASpec, err
	}

	nodeInfos, err := utils.GetNodeInfo(ctx, r.ClientSet, composableDRASpec)
	if err != nil {
		return nil, nil, nil, composableDRASpec, err
	}

	return resourceClaimInfos, resourceSliceInfos, nodeInfos, composableDRASpec, nil
}

func (r *ResourceMonitorReconciler) updateComposableResourceLastUsedTime(ctx context.Context, resourceSliceInfoList []types.ResourceSliceInfo, resourceClaimInfoList []types.ResourceClaimInfo, labelPrefix string) error {
	resourceList := &cdioperator.ComposableResourceList{}
	if err := r.List(ctx, resourceList, &client.ListOptions{}); err != nil {
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
					resource.Annotations[labelPrefix+"/last-used-time"] = currentTime
					if err := r.Update(ctx, &resource); err != nil {
						return fmt.Errorf("failed to update ComposableResource: %w", err)
					}
					break ResourceLoop
				}
			}
		}
	}

	return nil
}

func (r *ResourceMonitorReconciler) handleNodes(ctx context.Context, nodeInfos []types.NodeInfo, resourceClaimInfos []types.ResourceClaimInfo, composableDRASpec types.ComposableDRASpec) error {
	var err error
	for _, nodeInfo := range nodeInfos {
		resourceClaimInfos, err = utils.RescheduleFailedNotification(ctx, r.Client, nodeInfo, resourceClaimInfos, composableDRASpec)
		if err != nil {
			return err
		}

		resourceClaimInfos, err = utils.RescheduleNotification(ctx, r.Client, nodeInfo, resourceClaimInfos)
		if err != nil {
			return err
		}

		err = r.handleDevices(ctx, nodeInfo, resourceClaimInfos, composableDRASpec)
		if err != nil {
			return err
		}

		err = utils.UpdateNodeLabel(ctx, r.Client, r.ClientSet, nodeInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ResourceMonitorReconciler) handleDevices(ctx context.Context, nodeInfo types.NodeInfo, resourceClaimInfos []types.ResourceClaimInfo, composableDRASpec types.ComposableDRASpec) error {
	deviceCount, err := utils.GetConfiguredDeviceCount(ctx, r.Client, resourceClaimInfos)
	if err != nil {
		return err
	}

	composabilityRequestList := &cdioperator.ComposabilityRequestList{}
	if err := r.List(ctx, composabilityRequestList, &client.ListOptions{}); err != nil {
		return err
	}

	var requiredCount, actualCount int64
	var exit bool
	for _, device := range composableDRASpec.DeviceInfos {
		requiredCount = int64(deviceCount[device.CDIModelName])
		exit = false
		for _, cr := range composabilityRequestList.Items {
			if cr.Spec.Resource.Model == device.CDIModelName {
				actualCount = cr.Spec.Resource.Size
				if requiredCount > actualCount {
					err := utils.DynamicAttach(ctx, r.Client, &cr, requiredCount, device.CDIModelName, nodeInfo.Name)
					if err != nil {
						return err
					}
				} else if requiredCount < actualCount {
					err := utils.DynamicDetach(ctx, r.Client, &cr, requiredCount)
					if err != nil {
						return err
					}
				}
				exit = true
				break
			}
		}
		if !exit && requiredCount > 0 {
			err := utils.DynamicAttach(ctx, r.Client, nil, requiredCount, device.CDIModelName, nodeInfo.Name)
			if err != nil {
				return err
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
