package utils

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSortByTime(t *testing.T) {
	now := time.Now().UTC()
	hourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)
	tests := []struct {
		name  string
		input []types.ResourceClaimInfo
		want  []types.ResourceClaimInfo
	}{
		{
			name: "normal descending order",
			input: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(twoHoursAgo)},
				{CreationTimestamp: metav1.NewTime(now)},
				{CreationTimestamp: metav1.NewTime(hourAgo)},
			},
			want: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(now)},
				{CreationTimestamp: metav1.NewTime(hourAgo)},
				{CreationTimestamp: metav1.NewTime(twoHoursAgo)},
			},
		},
		{
			name:  "empty slice",
			input: []types.ResourceClaimInfo{},
			want:  []types.ResourceClaimInfo{},
		},
		{
			name: "single element",
			input: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(now)},
			},
			want: []types.ResourceClaimInfo{
				{CreationTimestamp: metav1.NewTime(now)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]types.ResourceClaimInfo, len(tt.input))
			copy(got, tt.input)

			sortByTime(got)

			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.want))
			}
			for i := 0; i < len(got); i++ {
				if !got[i].CreationTimestamp.Time.Equal(tt.want[i].CreationTimestamp.Time) {
					t.Errorf("index %d time mismatch:\ngot:  %v\nwant: %v",
						i, got[i].CreationTimestamp.Time, tt.want[i].CreationTimestamp.Time)
				}

				if got[i].Name != tt.want[i].Name {
					t.Errorf("index %d name mismatch:\ngot:  %q\nwant: %q",
						i, got[i].Name, tt.want[i].Name)
				}
			}
		})
	}
}

func TestNotIn(t *testing.T) {
	tests := []struct {
		name     string
		target   int
		slice    []int
		expected bool
	}{
		{"target not in slice", 5, []int{1, 2, 3, 4}, true},
		{"target in slice", 3, []int{1, 2, 3, 4}, false},
		{"empty slice", 1, []int{}, true},
		{"single element slice, target not in", 2, []int{1}, true},
		{"single element slice, target in", 1, []int{1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := notIn(tt.target, tt.slice)
			if result != tt.expected {
				t.Errorf("notIn(%v, %v) = %v; expected %v", tt.target, tt.slice, result, tt.expected)
			}
		})
	}
}

func TestIsDeviceCoexistence(t *testing.T) {
	tests := []struct {
		name                string
		model1              string
		model2              string
		composableDRASpec   types.ComposableDRASpec
		expectedCoexistence bool
	}{
		{
			name:   "Models can coexist",
			model1: "ModelA",
			model2: "ModelB",
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             0,
						CDIModelName:      "ModelA",
						CannotCoexistWith: []int{},
					},
					{
						Index:             1,
						CDIModelName:      "ModelB",
						CannotCoexistWith: []int{},
					},
				},
			},
			expectedCoexistence: true,
		},
		{
			name:   "Models cannot coexist",
			model1: "ModelA",
			model2: "ModelB",
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             0,
						CDIModelName:      "ModelA",
						CannotCoexistWith: []int{1},
					},
					{
						Index:             1,
						CDIModelName:      "ModelB",
						CannotCoexistWith: []int{},
					},
				},
			},
			expectedCoexistence: false,
		},
		{
			name:   "Model not found",
			model1: "ModelX",
			model2: "ModelY",
			composableDRASpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:             0,
						CDIModelName:      "ModelA",
						CannotCoexistWith: []int{},
					},
					{
						Index:             1,
						CDIModelName:      "ModelB",
						CannotCoexistWith: []int{},
					},
				},
			},
			expectedCoexistence: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDeviceCoexistence(tt.model1, tt.model2, tt.composableDRASpec)
			if result != tt.expectedCoexistence {
				t.Errorf("expected %v, got %v", tt.expectedCoexistence, result)
			}
		})
	}
}

func TestIsLastUsedOverMinute(t *testing.T) {
	tests := []struct {
		name           string
		annotations    map[string]string
		expectedResult bool
		expectedErr    bool
		errMsg         string
	}{
		{
			name:           "No annotations",
			annotations:    nil,
			expectedResult: false,
			expectedErr:    true,
			errMsg:         "annotations not found",
		},
		{
			name:           "Annotation not found",
			annotations:    map[string]string{},
			expectedResult: false,
			expectedErr:    true,
			errMsg:         "annotation composable.test/last-used-time not found",
		},
		{
			name: "Invalid time format",
			annotations: map[string]string{
				"composable.test/last-used-time": "invalid-time-format",
			},
			expectedResult: false,
			expectedErr:    true,
			errMsg:         fmt.Errorf("failed to parse time: %v", fmt.Errorf("parsing time \"invalid-time-format\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"invalid-time-format\" as \"2006\"")).Error(),
		},
		{
			name: "Time less than a minute ago",
			annotations: map[string]string{
				"composable.test/last-used-time": time.Now().Add(-30 * time.Second).Format(time.RFC3339),
			},
			expectedResult: false,
			expectedErr:    false,
		},
		{
			name: "Time more than a minute ago",
			annotations: map[string]string{
				"composable.test/last-used-time": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
			},
			expectedResult: true,
			expectedErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
				Spec: cdioperator.ComposableResourceSpec{
					Type:       "gpu",
					Model:      "A100",
					TargetNode: "node1",
				},
			}

			result, err := isLastUsedOverMinute(resource)

			if tt.expectedErr {
				if err == nil {
					t.Errorf("Expected error, got none")
				} else if err.Error() != tt.errMsg {
					t.Errorf("Expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else if !tt.expectedErr {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				} else if result != tt.expectedResult {
					t.Errorf("Expected result: %v, got %v", tt.expectedResult, result)
				}
			}
		})
	}
}

func TestSetDevicesState(t *testing.T) {
	testCases := []struct {
		name              string
		resourceClaimInfo types.ResourceClaimInfo
		targetState       string
		conditionType     string
		existingRC        *resourceapi.ResourceClaim
		expectedRC        *resourceapi.ResourceClaim
		expectedRCInfo    types.ResourceClaimInfo
		wantErr           bool
	}{
		{
			name: "ResourceClaim with empty condition",
			resourceClaimInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Preparing",
					},
				},
			},
			targetState:   "Reschedule",
			conditionType: "FabricDeviceReschedule",
			existingRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device:     "device-1",
							Conditions: []metav1.Condition{},
						},
					},
				},
			},
			expectedRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expectedRCInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Reschedule",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ResourceClaim with non-empty condition",
			resourceClaimInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Preparing",
					},
				},
			},
			targetState:   "Reschedule",
			conditionType: "FabricDeviceReschedule",
			existingRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionFalse,
								},
							},
						},
					},
				},
			},
			expectedRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expectedRCInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Reschedule",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ResourceClaim with matched condition",
			resourceClaimInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Preparing",
					},
				},
			},
			targetState:   "Reschedule",
			conditionType: "FabricDeviceReschedule",
			existingRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expectedRC: &resourceapi.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Status: resourceapi.ResourceClaimStatus{
					Devices: []resourceapi.AllocatedDeviceStatus{
						{
							Device: "device-1",
							Conditions: []metav1.Condition{
								{
									Type:   "FabricDeviceReschedule",
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			expectedRCInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Reschedule",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ResourceClaim not found",
			resourceClaimInfo: types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "test-ns",
				Devices: []types.ResourceClaimDevice{
					{
						Name:  "device-1",
						State: "Preparing",
					},
				},
			},
			targetState:   "Ready",
			conditionType: "Ready",
			existingRC:    nil,
			expectedRC:    nil,
			wantErr:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			objs := []runtime.Object{}
			if tc.existingRC != nil {
				objs = append(objs, tc.existingRC)
			}

			s := scheme.Scheme
			s.AddKnownTypes(resourceapi.SchemeGroupVersion, &resourceapi.ResourceClaim{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).WithStatusSubresource(&resourceapi.ResourceClaim{}).Build()

			result, err := setDevicesState(ctx, fakeClient, tc.resourceClaimInfo, tc.targetState, tc.conditionType)

			if (err != nil) != tc.wantErr {
				t.Fatalf("setDevicesState() error = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr {
				updatedRC := &resourceapi.ResourceClaim{}
				namespacedName := k8stypes.NamespacedName{
					Name:      tc.resourceClaimInfo.Name,
					Namespace: tc.resourceClaimInfo.Namespace,
				}
				err := fakeClient.Get(ctx, namespacedName, updatedRC)
				if err != nil {
					t.Fatalf("failed to get updated ResourceClaim: %v", err)
				}

				for i := range updatedRC.Status.Devices {
					for j := range updatedRC.Status.Devices[i].Conditions {
						updatedRC.Status.Devices[i].Conditions[j].LastTransitionTime = metav1.Time{}
						if tc.expectedRC != nil && tc.expectedRC.Status.Devices != nil && len(tc.expectedRC.Status.Devices) > 0 {
							tc.expectedRC.Status.Devices[i].Conditions[j].LastTransitionTime = metav1.Time{}
						}
					}
				}

				if !reflect.DeepEqual(updatedRC.Status.Devices, tc.expectedRC.Status.Devices) {
					t.Errorf("ResourceClaim was not updated as expected.\nGot:  %#v\nWant: %#v", updatedRC.Status.Devices, tc.expectedRC.Status.Devices)
				}
				if !reflect.DeepEqual(result, tc.expectedRCInfo) {
					t.Errorf("ResourceClaimInfo was not updated as expected.\nGot:  %#v\nWant: %#v", result, tc.expectedRCInfo)
				}
			}
		})
	}
}
