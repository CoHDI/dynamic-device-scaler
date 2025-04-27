package utils

import (
	"context"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetConfigMapInfo(t *testing.T) {
	tests := []struct {
		name            string
		configMapData   map[string]string
		createConfigMap bool
		wantSpec        types.ComposableDRASpec
		wantErr         bool
		errMsg          string
	}{
		{
			name: "Success info",
			configMapData: map[string]string{
				"device-info": `
- index: 1
  cdi-model-name: "A100 40G"
  dra-attributes:
    - productName: "NVIDIA A100 40GB PCIe"
  label-key-model: "composable-a100-40G"
  driver-name: "gpu.nvidia.com"
  k8s-device-name: "nvidia-a100-40"
  cannot-coexist-with: [2, 3, 4]
            `,
				"label-prefix":    "composable.fsastech.com",
				"fabric-id-range": `[1, 2, 3]`,
			},
			createConfigMap: true,
			wantSpec: types.ComposableDRASpec{
				DeviceInfos: []types.DeviceInfo{
					{
						Index:        1,
						CDIModelName: "A100 40G",
						DRAttributes: []types.DRAttribute{
							{
								ProductName: "NVIDIA A100 40GB PCIe",
							},
						},
						LabelKeyModel:     "composable-a100-40G",
						DriverName:        "gpu.nvidia.com",
						K8sDeviceName:     "nvidia-a100-40",
						CannotCoexistWith: []int{2, 3, 4},
					},
				},
				LabelPrefix:   "composable.fsastech.com",
				FabricIDRange: []int{1, 2, 3},
			},
			wantErr: false,
		},
		{
			name:            "Configmap not found",
			createConfigMap: false,
			wantErr:         true,
			errMsg:          "failed to get ConfigMap",
		},
		{
			name: "Invalid device info",
			configMapData: map[string]string{
				"device-info":  "invalid yaml",
				"label-prefix": "test-",
			},
			createConfigMap: true,
			wantErr:         true,
			errMsg:          "failed to parse device-info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var clientSet *fake.Clientset
			if tt.createConfigMap {
				clientSet = fake.NewSimpleClientset(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "composable-dra-dds",
						Namespace: "composable-dra",
					},
					Data: tt.configMapData,
				})
			} else {
				clientSet = fake.NewSimpleClientset()
			}

			result, err := GetConfigMapInfo(context.Background(), clientSet)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(result, tt.wantSpec) {
					t.Errorf("got %+v, want %+v", result, tt.wantSpec)
				}
			}
		})
	}
}
