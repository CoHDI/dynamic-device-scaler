package utils

import (
	"fmt"
	"testing"
	"time"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
