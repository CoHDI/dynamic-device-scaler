package utils

import (
	"context"
	"testing"
	"time"

	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func TestSetDevicesReschedule(t *testing.T) {

	testCases := []struct {
		name           string
		setupClient    func() client.Client
		inputClaim     *types.ResourceClaimInfo
		wantErr        bool
		wantStates     map[string]string
		wantConditions map[string][]metav1.Condition
	}{
		{
			name: "successful reschedule with preparing devices",
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithObjects(&resourceapi.ResourceClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "default",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.DeviceStatus{
								{ID: "device-1", Conditions: []metav1.Condition{}},
								{ID: "device-2", Conditions: []metav1.Condition{}},
							},
						},
					}).
					Build()
			},
			inputClaim: &types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "default",
				Devices: []types.ResourceClaimDevice{
					{Name: "device-1", State: "Preparing"},
					{Name: "device-2", State: "Active"},
				},
			},
			wantErr: false,
			wantStates: map[string]string{
				"device-1": "Reschedule",
				"device-2": "Active",
			},
			wantConditions: map[string][]metav1.Condition{
				"device-1": {
					{Type: "FabricDeviceReschedule", Status: metav1.ConditionTrue},
				},
				"device-2": {
					{Type: "FabricDeviceReschedule", Status: metav1.ConditionTrue},
				},
			},
		},
		{
			name: "no devices to reschedule",
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithObjects(&resourceapi.ResourceClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "default",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{Device: "device-1", Conditions: []metav1.Condition{}},
							},
						},
					}).
					Build()
			},
			inputClaim: &types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "default",
				Devices: []types.ResourceClaimDevice{
					{Name: "device-1", State: "Active"},
				},
			},
			wantErr: false,
			wantStates: map[string]string{
				"device-1": "Active",
			},
			wantConditions: map[string][]metav1.Condition{
				"device-1": {
					{Type: "FabricDeviceReschedule", Status: metav1.ConditionTrue},
				},
			},
		},
		{
			name: "resource claim not found",
			setupClient: func() client.Client {
				return fake.NewClientBuilder().Build()
			},
			inputClaim: &types.ResourceClaimInfo{
				Name:      "non-existent",
				Namespace: "default",
				Devices: []types.ResourceClaimDevice{
					{Name: "device-1", State: "Preparing"},
				},
			},
			wantErr: true,
		},
		{
			name: "existing conditions preserved",
			setupClient: func() client.Client {
				return fake.NewClientBuilder().
					WithObjects(&resourceapi.ResourceClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-claim",
							Namespace: "default",
						},
						Status: resourceapi.ResourceClaimStatus{
							Devices: []resourceapi.AllocatedDeviceStatus{
								{Device: "device-1", Conditions: []metav1.Condition{
									{Type: "ExistingCondition", Status: metav1.ConditionTrue},
								}},
							},
						},
					}).
					Build()
			},
			inputClaim: &types.ResourceClaimInfo{
				Name:      "test-claim",
				Namespace: "default",
				Devices: []types.ResourceClaimDevice{
					{Name: "device-1", State: "Preparing"},
				},
			},
			wantErr: false,
			wantConditions: map[string][]metav1.Condition{
				"device-1": {
					{Type: "ExistingCondition", Status: metav1.ConditionTrue},
					{Type: "FabricDeviceReschedule", Status: metav1.ConditionTrue},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := tc.setupClient()

			err := setDevicesReschedule(context.Background(), kubeClient, tc.inputClaim)

			if (err != nil) != tc.wantErr {
				t.Fatalf("setDevicesReschedule() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}

			for _, device := range tc.inputClaim.Devices {
				wantState, ok := tc.wantStates[device.Name]
				if !ok {
					continue
				}
				if device.State != wantState {
					t.Errorf("device %s state = %s, want %s", device.Name, device.State, wantState)
				}
			}

			var rc resourceapi.ResourceClaim
			if err := kubeClient.Get(
				context.Background(),
				client.ObjectKey{Name: tc.inputClaim.Name, Namespace: tc.inputClaim.Namespace},
				&rc,
			); err != nil {
				t.Fatalf("failed to get updated ResourceClaim: %v", err)
			}

			for _, deviceStatus := range rc.Status.Devices {
				wantConditions, ok := tc.wantConditions[deviceStatus.ID]
				if !ok {
					continue
				}

				if len(deviceStatus.Conditions) != len(wantConditions) {
					t.Errorf("device %s conditions count = %d, want %d",
						deviceStatus.ID, len(deviceStatus.Conditions), len(wantConditions))
					continue
				}

				for i, cond := range deviceStatus.Conditions {
					if cond.Type != wantConditions[i].Type || cond.Status != wantConditions[i].Status {
						t.Errorf("device %s condition[%d] = %s/%s, want %s/%s",
							deviceStatus.ID, i, cond.Type, cond.Status,
							wantConditions[i].Type, wantConditions[i].Status)
					}
				}
			}
		})
	}
}
