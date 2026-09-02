package controller

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apmev1alpha1 "github.com/ansible/apme-operator/api/v1alpha1"
	"github.com/ansible/apme-operator/internal/manifests"
	"github.com/ansible/apme-operator/internal/resolve"
)

func routeScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(apmev1alpha1.AddToScheme(s)).To(Succeed())
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(appsv1.AddToScheme(s)).To(Succeed())
	return s
}

func testDesired() resolve.Desired {
	ui := true
	route := true
	cr := &apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "default"},
		Spec: apmev1alpha1.ApmeSpec{
			Components: apmev1alpha1.ComponentsSpec{UI: &ui},
			Exposure: apmev1alpha1.ExposureSpec{
				Route: apmev1alpha1.RouteSpec{Enabled: &route, Host: "apme.apps.example.com"},
			},
		},
	}
	return resolve.From(cr)
}

func TestEnsureRoutesGarbageCollects(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	scheme := routeScheme(t)

	cr := &apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "default", UID: "uid-1"},
	}
	d := testDesired()
	uiRoute := manifests.UIRoute(d)
	apiRoute := manifests.APIRoute(d)
	g.Expect(controllerutil.SetControllerReference(cr, uiRoute, scheme)).To(Succeed())
	g.Expect(controllerutil.SetControllerReference(cr, apiRoute, scheme)).To(Succeed())

	foreign := manifests.UIRoute(d)
	foreign.SetName("unrelated-route")
	foreign.SetLabels(map[string]string{"app": "other"})

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr, uiRoute, apiRoute, foreign).Build()
	r := &ApmeReconciler{Client: cl, Scheme: scheme, HasRouteAPI: true}

	d.RouteEnabled = false
	d.UI = false
	g.Expect(r.ensureRoutes(ctx, cr, d)).To(Succeed())

	got := newRoute()
	err := cl.Get(ctx, client.ObjectKey{Name: "apme-ui", Namespace: "default"}, got)
	g.Expect(err).To(HaveOccurred())
	g.Expect(client.IgnoreNotFound(err)).To(Succeed())

	err = cl.Get(ctx, client.ObjectKey{Name: "apme-api", Namespace: "default"}, got)
	g.Expect(err).To(HaveOccurred())
	g.Expect(client.IgnoreNotFound(err)).To(Succeed())

	err = cl.Get(ctx, client.ObjectKey{Name: "unrelated-route", Namespace: "default"}, got)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestEnsureRoutesRemovesUIOnly(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	scheme := routeScheme(t)

	cr := &apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "default", UID: "uid-1"},
	}
	d := testDesired()
	uiRoute := manifests.UIRoute(d)
	apiRoute := manifests.APIRoute(d)
	g.Expect(controllerutil.SetControllerReference(cr, uiRoute, scheme)).To(Succeed())
	g.Expect(controllerutil.SetControllerReference(cr, apiRoute, scheme)).To(Succeed())

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr, uiRoute, apiRoute).Build()
	r := &ApmeReconciler{Client: cl, Scheme: scheme, HasRouteAPI: true}

	d.UI = false
	g.Expect(r.ensureRoutes(ctx, cr, d)).To(Succeed())

	got := newRoute()
	err := cl.Get(ctx, client.ObjectKey{Name: "apme-ui", Namespace: "default"}, got)
	g.Expect(err).To(HaveOccurred())
	g.Expect(client.IgnoreNotFound(err)).To(Succeed())

	// Path changed (/api → ""); first pass deletes, second recreates.
	g.Expect(r.ensureRoutes(ctx, cr, d)).To(Succeed())
	err = cl.Get(ctx, client.ObjectKey{Name: "apme-api", Namespace: "default"}, got)
	g.Expect(err).NotTo(HaveOccurred())
	path, _, _ := unstructured.NestedString(got.Object, "spec", "path")
	g.Expect(path).To(Equal(""))
}

func TestEnsureRoutesRecreatesOnHostChange(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	scheme := routeScheme(t)

	cr := &apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "default", UID: "uid-1"},
	}
	d := testDesired()
	apiRoute := manifests.APIRoute(d)
	g.Expect(controllerutil.SetControllerReference(cr, apiRoute, scheme)).To(Succeed())

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr, apiRoute).Build()
	r := &ApmeReconciler{Client: cl, Scheme: scheme, HasRouteAPI: true}

	d.RouteHost = "new.apps.example.com"
	g.Expect(r.ensureRoutes(ctx, cr, d)).To(Succeed())
	got := newRoute()
	err := cl.Get(ctx, client.ObjectKey{Name: "apme-api", Namespace: "default"}, got)
	g.Expect(err).To(HaveOccurred())

	g.Expect(r.ensureRoutes(ctx, cr, d)).To(Succeed())
	err = cl.Get(ctx, client.ObjectKey{Name: "apme-api", Namespace: "default"}, got)
	g.Expect(err).NotTo(HaveOccurred())
	host, _, _ := unstructured.NestedString(got.Object, "spec", "host")
	g.Expect(host).To(Equal("new.apps.example.com"))
	intent, _, _ := unstructured.NestedString(got.Object, "metadata", "annotations", manifests.RouteDesiredHostAnnotation)
	g.Expect(intent).To(Equal("new.apps.example.com"))
}

func TestRouteNeedsRecreate(t *testing.T) {
	g := NewWithT(t)
	d := testDesired()
	desired := manifests.APIRoute(d)

	existing := desired.DeepCopy()
	g.Expect(routeNeedsRecreate(existing, desired)).To(BeFalse())

	_ = unstructured.SetNestedField(existing.Object, "other.example.com", "spec", "host")
	_ = unstructured.SetNestedField(existing.Object, "other.example.com", "metadata", "annotations", manifests.RouteDesiredHostAnnotation)
	g.Expect(routeNeedsRecreate(existing, desired)).To(BeTrue())

	existing = desired.DeepCopy()
	_ = unstructured.SetNestedField(existing.Object, "", "spec", "path")
	g.Expect(routeNeedsRecreate(existing, desired)).To(BeTrue())
}

func TestImagePullMessage(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	scheme := routeScheme(t)

	labels := map[string]string{
		"app.kubernetes.io/name":      "apme",
		"app.kubernetes.io/instance":  "apme",
		"app.kubernetes.io/component": "engine",
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "apme-abc",
			Namespace: "default",
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "engine",
				Image: "quay.io/ansible/apme-engine:missing",
			}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "engine",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "ImagePullBackOff",
						Message: "Back-off pulling image",
					},
				},
			}},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, pod).Build()
	r := &ApmeReconciler{Client: cl, Scheme: scheme}
	msg := r.imagePullMessage(ctx, dep)
	g.Expect(msg).To(ContainSubstring("ImagePullBackOff"))
	g.Expect(msg).To(ContainSubstring("quay.io/ansible/apme-engine:missing"))
}

func TestWrapRouteApplyErrForbidden(t *testing.T) {
	g := NewWithT(t)
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: "route.openshift.io", Resource: "routes/custom-host"},
		"",
		nil,
	)
	err := wrapRouteApplyErr(forbidden, "apme.apps.example.com")
	g.Expect(isRoutePermissionErr(err)).To(BeTrue())
	g.Expect(err.Error()).To(ContainSubstring("routes/custom-host"))

	plain := wrapRouteApplyErr(apierrors.NewForbidden(
		schema.GroupResource{Group: "route.openshift.io", Resource: "routes"},
		"",
		nil,
	), "")
	g.Expect(isRoutePermissionErr(plain)).To(BeFalse())
	g.Expect(plain.Error()).To(ContainSubstring("route.openshift.io/routes"))
}

func TestApplyOrRecreateRoutePreservesAssignedHost(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	scheme := routeScheme(t)

	cr := &apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "default", UID: "uid-1"},
	}
	d := testDesired()
	d.RouteHost = ""
	existing := manifests.APIRoute(d)
	g.Expect(controllerutil.SetControllerReference(cr, existing, scheme)).To(Succeed())
	_ = unstructured.SetNestedField(existing.Object, "apme-api-default.apps.cluster", "spec", "host")

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr, existing).Build()
	r := &ApmeReconciler{Client: cl, Scheme: scheme, HasRouteAPI: true}

	desired := manifests.APIRoute(d)
	g.Expect(r.applyOrRecreateRoute(ctx, cr, desired)).To(Succeed())

	got := newRoute()
	g.Expect(cl.Get(ctx, client.ObjectKey{Name: "apme-api", Namespace: "default"}, got)).To(Succeed())
	host, _, _ := unstructured.NestedString(got.Object, "spec", "host")
	g.Expect(host).To(Equal("apme-api-default.apps.cluster"))
}
