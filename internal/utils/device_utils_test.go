package utils

import (
	"context"
	"reflect"
	"testing"
	"time"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetConfiguredDeviceCount(t *testing.T) {
	testCases := []struct {
		name                 string
		existingResourceList *cdioperator.ComposableResourceList
		resourceClaims       []types.ResourceClaimInfo
		expectedResult       map[string]int
		wantErr              bool
		expectedErrMsg       string
	}{
		{
			name:           "empty resource claim info",
			expectedResult: map[string]int{},
		},
		{
			name: "resource cliam with preparing state devices",
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name: "test",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "GPU2",
							Model: "A100 40G",
							State: "Preparing",
						},
					},
				},
			},
			expectedResult: map[string]int{
				"A100 40G": 2,
			},
		},
		{
			name: "resource cliam with other state devices",
			resourceClaims: []types.ResourceClaimInfo{
				{
					Name: "test",
					Devices: []types.ResourceClaimDevice{
						{
							Name:  "GPU1",
							Model: "A100 40G",
							State: "Preparing",
						},
						{
							Name:  "GPU2",
							Model: "A100 40G",
							State: "Failed",
						},
					},
				},
			},
			expectedResult: map[string]int{
				"A100 40G": 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			if tc.existingResourceList != nil {
				for i := range tc.existingResourceList.Items {
					clientObjects = append(clientObjects, &tc.existingResourceList.Items[i])
				}
			}

			s := scheme.Scheme
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			result, err := GetConfiguredDeviceCount(context.Background(), fakeClient, tc.resourceClaims)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				if err.Error() != tc.expectedErrMsg {
					t.Errorf("Error message is incorrect. Got: %q, Want: %q", err.Error(), tc.expectedErrMsg)
				}
				return
			}
			if !reflect.DeepEqual(result, tc.expectedResult) {
				t.Errorf("Expected result of GetConfiguredDeviceCount. Got: %v, Want: %v", result, tc.expectedResult)
			}
		})
	}
}

func TestDynamicAttach(t *testing.T) {
	testCases := []struct {
		name                         string
		existingComposabilityRequest *cdioperator.ComposabilityRequestList
		updateComposabilityRequest   *cdioperator.ComposabilityRequest
		count                        int64
		model                        string
		nodeName                     string
		wantErr                      bool
		expectedErrMsg               string
	}{
		{
			name:                       "empty update ComposabilityRequest",
			updateComposabilityRequest: nil,
			model:                      "A100 40G",
			count:                      2,
			nodeName:                   "node1",
		},
		{
			name: "empty existing ComposabilityRequest",
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       2,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			count:          2,
			wantErr:        true,
			expectedErrMsg: "failed to get ComposabilityRequest: composabilityrequests.meta.k8s.io \"test\" not found",
		},
		{
			name: "normal case",
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       2,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			existingComposabilityRequest: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Size:       2,
								Model:      "A100 40G",
								TargetNode: "node1",
							},
						},
					},
				},
			},
			count: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			if tc.existingComposabilityRequest != nil {
				for i := range tc.existingComposabilityRequest.Items {
					clientObjects = append(clientObjects, &tc.existingComposabilityRequest.Items[i])
				}
			}

			s := scheme.Scheme
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposabilityRequest{}, &cdioperator.ComposabilityRequestList{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			err := DynamicAttach(context.Background(), fakeClient, tc.updateComposabilityRequest, tc.count, tc.model, tc.nodeName)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				if err.Error() != tc.expectedErrMsg {
					t.Errorf("Error message is incorrect. Got: %q, Want: %q", err.Error(), tc.expectedErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.updateComposabilityRequest == nil {
				crList := &cdioperator.ComposabilityRequestList{}
				err = fakeClient.List(context.Background(), crList)
				if err != nil {
					t.Fatalf("Failed to list ComposabilityRequests: %v", err)
				}

				if len(crList.Items) != 1 {
					t.Fatalf("Expected 1 ComposabilityRequest, got %d", len(crList.Items))
				}

				if crList.Items[0].Spec.Resource.Model != tc.model {
					t.Errorf("Expected Model %q, got %q", tc.model, crList.Items[0].Spec.Resource.Model)
				}
				if crList.Items[0].Spec.Resource.Size != tc.count {
					t.Errorf("Expected Size %d, got %d", tc.count, crList.Items[0].Spec.Resource.Size)
				}
				if crList.Items[0].Spec.Resource.TargetNode != tc.nodeName {
					t.Errorf("Expected TargetNode %q, got %q", tc.nodeName, crList.Items[0].Spec.Resource.TargetNode)
				}
			} else {
				existingCR := &cdioperator.ComposabilityRequest{}
				err := fakeClient.Get(context.Background(), k8stypes.NamespacedName{Name: tc.updateComposabilityRequest.Name}, existingCR)
				if err != nil {
					t.Errorf("failed to get ComposabilityRequest: %v", err)
				}

				if existingCR.Spec.Resource.Size != tc.count {
					t.Errorf("Expected Size %d, got %d", tc.count, existingCR.Spec.Resource.Size)
				}
			}
		})
	}
}

func TestDynamicDetach(t *testing.T) {
	now := time.Now()
	thirtySecondsAgo := now.Add(-30 * time.Second)

	testCases := []struct {
		name                         string
		existingComposabilityRequest *cdioperator.ComposabilityRequestList
		existingComposableResource   *cdioperator.ComposableResourceList
		updateComposabilityRequest   *cdioperator.ComposabilityRequest
		count                        int64
		wantErr                      bool
		expectedErrMsg               string
		expectedSize                 int64
	}{
		{
			name: "nextSize less than composabilityRequest size",
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       4,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			count: 3,
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "res1"},
						Status:     cdioperator.ComposableResourceStatus{State: "Online"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res2",
							DeletionTimestamp: &metav1.Time{Time: thirtySecondsAgo},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
					},
				},
			},
			existingComposabilityRequest: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Size:       2,
								Model:      "A100 40G",
								TargetNode: "node1",
							},
						},
					},
				},
			},
			expectedSize: 3,
		},
		{
			name: "nextSize less than composabilityRequest size",
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       4,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			count: 3,
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "res1"},
						Status:     cdioperator.ComposableResourceStatus{State: "Online"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res2",
							DeletionTimestamp: &metav1.Time{Time: thirtySecondsAgo},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
					},
				},
			},
			existingComposabilityRequest: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Size:       2,
								Model:      "A100 40G",
								TargetNode: "node1",
							},
						},
					},
				},
			},
			expectedSize: 3,
		},
		{
			name: "count greater than composabilityRequest size",
			updateComposabilityRequest: &cdioperator.ComposabilityRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: cdioperator.ComposabilityRequestSpec{
					Resource: cdioperator.ScalarResourceDetails{
						Type:       "gpu",
						Size:       2,
						Model:      "A100 40G",
						TargetNode: "node1",
					},
				},
			},
			count: 4,
			existingComposabilityRequest: &cdioperator.ComposabilityRequestList{
				Items: []cdioperator.ComposabilityRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: cdioperator.ComposabilityRequestSpec{
							Resource: cdioperator.ScalarResourceDetails{
								Type:       "gpu",
								Size:       2,
								Model:      "A100 40G",
								TargetNode: "node1",
							},
						},
					},
				},
			},
			expectedSize: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			if tc.existingComposabilityRequest != nil {
				for i := range tc.existingComposabilityRequest.Items {
					clientObjects = append(clientObjects, &tc.existingComposabilityRequest.Items[i])
				}
			}
			if tc.existingComposableResource != nil {
				for i := range tc.existingComposableResource.Items {
					clientObjects = append(clientObjects, &tc.existingComposableResource.Items[i])
				}
			}

			s := scheme.Scheme
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposabilityRequest{}, &cdioperator.ComposabilityRequestList{})
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			err := DynamicDetach(context.Background(), fakeClient, tc.updateComposabilityRequest, tc.count)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				if err.Error() != tc.expectedErrMsg {
					t.Errorf("Error message is incorrect. Got: %q, Want: %q", err.Error(), tc.expectedErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			existingCR := &cdioperator.ComposabilityRequest{}
			err = fakeClient.Get(context.Background(), k8stypes.NamespacedName{Name: tc.updateComposabilityRequest.Name}, existingCR)
			if err != nil {
				t.Errorf("failed to get ComposabilityRequest: %v", err)
			}

			if existingCR.Spec.Resource.Size != tc.expectedSize {
				t.Errorf("Expected Size %d, got %d", tc.expectedSize, existingCR.Spec.Resource.Size)
			}

		})
	}
}

func TestGetNextSize(t *testing.T) {
	now := time.Now()
	twoMinutesAgo := now.Add(-2 * time.Minute)
	thirtySecondsAgo := now.Add(-30 * time.Second)

	tests := []struct {
		name                       string
		existingComposableResource *cdioperator.ComposableResourceList
		count                      int64
		wantErr                    bool
		expectedErrMsg             string
		expectedSize               int64
	}{
		{
			name: "No qualified resources",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "res1"},
						Status:     cdioperator.ComposableResourceStatus{State: "Online"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res2",
							DeletionTimestamp: &metav1.Time{Time: thirtySecondsAgo},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
					},
				},
			},
			count:        3,
			expectedSize: 3,
		},
		{
			name: "Some qualified (count less than resourceCount)",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res1",
							DeletionTimestamp: &metav1.Time{Time: twoMinutesAgo.UTC()},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Online"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res2",
							DeletionTimestamp: &metav1.Time{Time: twoMinutesAgo.UTC()},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res3",
							DeletionTimestamp: &metav1.Time{Time: twoMinutesAgo.UTC()},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Offline"},
					},
				},
			},
			count:        1,
			expectedSize: 1,
		},
		{
			name: "Some qualified (count greater than resourceCount)",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res1",
							DeletionTimestamp: &metav1.Time{Time: twoMinutesAgo.UTC()},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Online"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res2",
							DeletionTimestamp: &metav1.Time{Time: twoMinutesAgo.UTC()},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res3",
							DeletionTimestamp: &metav1.Time{Time: twoMinutesAgo.UTC()},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Offline"},
					},
				},
			},
			count:        3,
			expectedSize: 3,
		},
		{
			name: "Time unqualified resources",
			existingComposableResource: &cdioperator.ComposableResourceList{
				Items: []cdioperator.ComposableResource{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "res1",
							DeletionTimestamp: &metav1.Time{Time: thirtySecondsAgo},
							Finalizers:        []string{"dummy-finalizer"},
						},
						Status: cdioperator.ComposableResourceStatus{State: "Online"},
					},
				},
			},
			count:        0,
			expectedSize: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientObjects := []runtime.Object{}
			if tc.existingComposableResource != nil {
				for i := range tc.existingComposableResource.Items {
					clientObjects = append(clientObjects, &tc.existingComposableResource.Items[i])
				}
			}

			s := scheme.Scheme
			s.AddKnownTypes(metav1.SchemeGroupVersion, &cdioperator.ComposableResource{}, &cdioperator.ComposableResourceList{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clientObjects...).Build()

			gotSize, err := getNextSize(context.Background(), fakeClient, tc.count)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				if err.Error() != tc.expectedErrMsg {
					t.Errorf("Error message is incorrect. Got: %q, Want: %q", err.Error(), tc.expectedErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if gotSize != tc.expectedSize {
				t.Errorf("Size mismatch\nWant: %d\nGot:  %d", tc.expectedSize, gotSize)
			}
		})
	}
}
