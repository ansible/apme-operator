package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apmev1alpha1 "github.com/ansible/apme-operator/api/v1alpha1"
	"github.com/ansible/apme-operator/internal/manifests"
	"github.com/ansible/apme-operator/internal/resolve"
)

// routePermissionError is returned when Route apply fails with Forbidden while
// setting a custom host (typically missing routes/custom-host).
type routePermissionError struct {
	err error
}

func (e *routePermissionError) Error() string {
	return fmt.Sprintf(
		"missing permission to manage Route hosts; grant create on route.openshift.io/routes/custom-host to the operator ServiceAccount: %v",
		e.err,
	)
}

func (e *routePermissionError) Unwrap() error { return e.err }

func isRoutePermissionErr(err error) bool {
	var pe *routePermissionError
	return errors.As(err, &pe)
}

func newRoute() *unstructured.Unstructured {
	rt := &unstructured.Unstructured{}
	rt.SetAPIVersion("route.openshift.io/v1")
	rt.SetKind("Route")
	return rt
}

func (r *ApmeReconciler) ensureRoutes(ctx context.Context, cr *apmev1alpha1.Apme, d resolve.Desired) error {
	if !r.hasRouteAPI() {
		return nil
	}

	wantUI := d.RouteEnabled && d.UI
	wantAPI := d.RouteEnabled

	if wantUI {
		if err := r.applyOrRecreateRoute(ctx, cr, manifests.UIRoute(d)); err != nil {
			return wrapRouteApplyErr(err, d.RouteHost)
		}
	} else if err := r.deleteOwnedRoute(ctx, cr, d.Name+"-ui", d.Namespace); err != nil {
		return err
	}

	if wantAPI {
		if err := r.applyOrRecreateRoute(ctx, cr, manifests.APIRoute(d)); err != nil {
			return wrapRouteApplyErr(err, d.RouteHost)
		}
	} else if err := r.deleteOwnedRoute(ctx, cr, d.Name+"-api", d.Namespace); err != nil {
		return err
	}

	return nil
}

func wrapRouteApplyErr(err error, customHost string) error {
	if err == nil {
		return nil
	}
	if !apierrors.IsForbidden(err) {
		return err
	}
	msg := strings.ToLower(err.Error())
	if customHost != "" || strings.Contains(msg, "custom-host") {
		return &routePermissionError{err: err}
	}
	return fmt.Errorf("forbidden managing OpenShift Route (check ClusterRole for route.openshift.io/routes): %w", err)
}

func (r *ApmeReconciler) applyOrRecreateRoute(ctx context.Context, owner *apmev1alpha1.Apme, desired *unstructured.Unstructured) error {
	existing := newRoute()
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if err == nil {
		if !existing.GetDeletionTimestamp().IsZero() {
			// Wait for GC before recreating.
			return nil
		}
		if metav1.IsControlledBy(existing, owner) && routeNeedsRecreate(existing, desired) {
			return r.Delete(ctx, existing)
		}
		// When the CR omits host, preserve the live host in the apply payload so
		// SSA ForceOwnership does not clear an OpenShift-assigned hostname.
		desiredHost, _, _ := unstructured.NestedString(desired.Object, "spec", "host")
		if desiredHost == "" {
			if existingHost, found, _ := unstructured.NestedString(existing.Object, "spec", "host"); found && existingHost != "" {
				_ = unstructured.SetNestedField(desired.Object, existingHost, "spec", "host")
			}
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	return r.apply(ctx, owner, desired)
}

func routeNeedsRecreate(existing, desired *unstructured.Unstructured) bool {
	existingPath, _, _ := unstructured.NestedString(existing.Object, "spec", "path")
	desiredPath, _, _ := unstructured.NestedString(desired.Object, "spec", "path")
	if existingPath != desiredPath {
		return true
	}

	desiredIntent, _, _ := unstructured.NestedString(desired.Object, "metadata", "annotations", manifests.RouteDesiredHostAnnotation)
	existingIntent, hasIntent, _ := unstructured.NestedString(existing.Object, "metadata", "annotations", manifests.RouteDesiredHostAnnotation)
	if hasIntent {
		return existingIntent != desiredIntent
	}

	// Legacy Routes without the annotation: recreate only when a custom host is
	// desired and differs from the live spec.host. Clearing host on legacy
	// Routes (no annotation yet) needs a one-time delete or waits until the
	// annotation is stamped by a subsequent apply.
	desiredHost, _, _ := unstructured.NestedString(desired.Object, "spec", "host")
	if desiredHost == "" {
		return false
	}
	existingHost, _, _ := unstructured.NestedString(existing.Object, "spec", "host")
	return existingHost != desiredHost
}

func (r *ApmeReconciler) deleteOwnedRoute(ctx context.Context, owner *apmev1alpha1.Apme, name, namespace string) error {
	rt := newRoute()
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, rt)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !metav1.IsControlledBy(rt, owner) {
		return nil
	}
	return client.IgnoreNotFound(r.Delete(ctx, rt))
}
