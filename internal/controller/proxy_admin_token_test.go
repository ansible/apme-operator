package controller

import (
	"bytes"
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apmev1alpha1 "github.com/ansible/apme-operator/api/v1alpha1"
	"github.com/ansible/apme-operator/internal/resolve"
)

func TestProxyAdminTokenLifecycle(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := apmev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cr := &apmev1alpha1.Apme{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns", UID: "test-owner"}}
	d := resolve.From(cr)
	r := &ApmeReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Scheme: scheme}
	ctx := context.Background()
	key := types.NamespacedName{Name: "test-proxy-admin", Namespace: cr.Namespace}
	if err := r.ensureProxyAdminToken(ctx, cr, d); err != nil {
		t.Fatal(err)
	}
	first := &corev1.Secret{}
	if err := r.Get(ctx, key, first); err != nil {
		t.Fatal(err)
	}
	if len(first.Data["token"]) != 32 {
		t.Fatal("expected a generated 32-character random token")
	}
	if len(first.OwnerReferences) != 1 || first.OwnerReferences[0].UID != cr.UID {
		t.Fatal("generated token must be owned by the Apme instance")
	}
	if err := r.ensureProxyAdminToken(ctx, cr, d); err != nil {
		t.Fatal(err)
	}
	second := &corev1.Secret{}
	if err := r.Get(ctx, key, second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Data["token"], second.Data["token"]) {
		t.Fatal("reconciliation rotated the token")
	}
	second.Data["token"] = nil
	if err := r.Update(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := r.ensureProxyAdminToken(ctx, cr, d); err == nil {
		t.Fatal("an existing secret with no token must fail visibly")
	}
}
