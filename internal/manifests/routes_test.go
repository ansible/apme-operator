package manifests

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	apmev1alpha1 "github.com/ansible/apme-operator/api/v1alpha1"
	"github.com/ansible/apme-operator/internal/resolve"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRouteOmitsEmptyHost(t *testing.T) {
	ui := true
	enabled := true
	d := resolve.From(&apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "default"},
		Spec: apmev1alpha1.ApmeSpec{
			Components: apmev1alpha1.ComponentsSpec{UI: &ui},
			Exposure:   apmev1alpha1.ExposureSpec{Route: apmev1alpha1.RouteSpec{Enabled: &enabled}},
		},
	})
	rt := UIRoute(d)
	_, found, err := unstructured.NestedString(rt.Object, "spec", "host")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatalf("expected empty host to be omitted from Route spec")
	}
	intent, _, _ := unstructured.NestedString(rt.Object, "metadata", "annotations", RouteDesiredHostAnnotation)
	if intent != "" {
		t.Fatalf("desired-host annotation=%q, want empty", intent)
	}
}

func TestRouteSetsCustomHost(t *testing.T) {
	ui := true
	enabled := true
	d := resolve.From(&apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "default"},
		Spec: apmev1alpha1.ApmeSpec{
			Components: apmev1alpha1.ComponentsSpec{UI: &ui},
			Exposure: apmev1alpha1.ExposureSpec{
				Route: apmev1alpha1.RouteSpec{Enabled: &enabled, Host: "apme.apps.example.com"},
			},
		},
	})
	rt := APIRoute(d)
	host, found, err := unstructured.NestedString(rt.Object, "spec", "host")
	if err != nil || !found {
		t.Fatalf("host found=%v err=%v", found, err)
	}
	if host != "apme.apps.example.com" {
		t.Fatalf("host=%s", host)
	}
	path, _, _ := unstructured.NestedString(rt.Object, "spec", "path")
	if path != "/api" {
		t.Fatalf("path=%s", path)
	}
}
