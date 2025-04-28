package utils

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"github.com/InfraDDS/dynamic-device-scaler/internal/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			} else if !reflect.DeepEqual(result, tc.expectedResult) {
				t.Errorf("Expected result of GetConfiguredDeviceCount. Got: %v, Want: %v", result, tc.expectedResult)
			}
		})
	}
}

func TestGetNextSize(t *testing.T) {
	now := time.Now()
	twoMinutesAgo := now.Add(-2 * time.Minute)
	thirtySecondsAgo := now.Add(-30 * time.Second)

	testResources := map[string][]client.Object{
		"no-qualified": {
			&cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{Name: "res1"},
				Status:     cdioperator.ComposableResourceStatus{State: "Online"},
			},
			&cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "res2",
					DeletionTimestamp: &metav1.Time{Time: thirtySecondsAgo},
					Finalizers:        []string{"dummy-finalizer"},
				},
				Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
			},
		},
		"some-qualified": {
			&cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "res1",
					DeletionTimestamp: &metav1.Time{Time: twoMinutesAgo.UTC()},
					Finalizers:        []string{"dummy-finalizer"},
				},
				Status: cdioperator.ComposableResourceStatus{State: "Online"},
			},
			&cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "res2",
					DeletionTimestamp: &metav1.Time{Time: twoMinutesAgo.UTC()},
					Finalizers:        []string{"dummy-finalizer"},
				},
				Status: cdioperator.ComposableResourceStatus{State: "Attaching"},
			},
			&cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "res3",
					DeletionTimestamp: &metav1.Time{Time: twoMinutesAgo.UTC()},
					Finalizers:        []string{"dummy-finalizer"},
				},
				Status: cdioperator.ComposableResourceStatus{State: "Offline"},
			},
		},
		"time-unqualified": {
			&cdioperator.ComposableResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "res1",
					DeletionTimestamp: &metav1.Time{Time: thirtySecondsAgo},
					Finalizers:        []string{"dummy-finalizer"},
				},
				Status: cdioperator.ComposableResourceStatus{State: "Online"},
			},
		},
	}

	tests := []struct {
		name       string
		listError  error
		objectsKey string
		count      int64
		wantSize   int64
		wantErr    string
	}{
		{
			name:      "List error",
			listError: errors.New("list error"),
			wantErr:   "failed to list ComposableResourceList: list error",
			wantSize:  0,
			count:     5,
		},
		{
			name:       "No qualified resources (count 3)",
			objectsKey: "no-qualified",
			count:      3,
			wantSize:   3,
		},
		{
			name:       "No qualified resources (count 0)",
			objectsKey: "no-qualified",
			count:      0,
			wantSize:   0,
		},
		{
			name:       "Some qualified (count < resourceCount)",
			objectsKey: "some-qualified",
			count:      1,
			wantSize:   1,
		},
		{
			name:       "Some qualified (count > resourceCount)",
			objectsKey: "some-qualified",
			count:      3,
			wantSize:   3,
		},
		{
			name:       "Time unqualified resources",
			objectsKey: "time-unqualified",
			count:      0,
			wantSize:   0,
		},
	}

	scheme := runtime.NewScheme()
	if err := cdioperator.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cl client.Client
			if tt.listError != nil {
				cl = &MockClient{listError: tt.listError}
			} else {
				cl = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(testResources[tt.objectsKey]...).
					Build()
			}

			gotSize, err := getNextSize(context.Background(), cl, tt.count)

			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("Error mismatch\nWant: %v\nGot:  %v", tt.wantErr, err)
				}
				return
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if gotSize != tt.wantSize {
					t.Errorf("Size mismatch\nWant: %d\nGot:  %d", tt.wantSize, gotSize)
				}
			}
		})
	}
}

type MockClient struct {
	client.Client
	listError error
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return m.listError
}
