package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	myv1 "github.com/somaz94/kube-drift/api/v1alpha1"
)

func TestReconcile_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = myv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &DriftCheckReconciler{
		Client: cl,
		Scheme: scheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent",
			Namespace: "default",
		},
	}

	_, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("Reconcile() error = %v, want nil for not found", err)
	}
}

func TestReconcile_Found(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = myv1.AddToScheme(scheme)

	resource := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type: myv1.SourceTypeConfigMap,
				ConfigMap: &myv1.ConfigMapSource{
					Name: "desired-manifests",
				},
			},
			Interval: metav1.Duration{Duration: 5 * time.Minute},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(resource).Build()

	r := &DriftCheckReconciler{
		Client: cl,
		Scheme: scheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-resource",
			Namespace: "default",
		},
	}

	_, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("Reconcile() error = %v, want nil", err)
	}
}
