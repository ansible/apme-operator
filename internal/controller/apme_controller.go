/*
Copyright 2026 Ansible.
*/

package controller

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	apmev1alpha1 "github.com/ansible/apme-operator/api/v1alpha1"
	"github.com/ansible/apme-operator/internal/manifests"
	"github.com/ansible/apme-operator/internal/manifests/postgres"
	"github.com/ansible/apme-operator/internal/resolve"
)

const (
	condReady         = "Ready"
	condProgressing   = "Progressing"
	condDegraded      = "Degraded"
	condDatabaseReady = "DatabaseReady"
	requeueStatus     = 20 * time.Second
)

// ApmeReconciler reconciles an Apme object.
type ApmeReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	HasRouteAPI bool
}

// +kubebuilder:rbac:groups=apme.ansible.com,resources=apmes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apme.ansible.com,resources=apmes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apme.ansible.com,resources=apmes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=serviceaccounts;services;configmaps;secrets;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies;ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes/custom-host,verbs=create

func (r *ApmeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	cr := &apmev1alpha1.Apme{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	d := resolve.From(cr)
	leftoverPG, err := r.ensureAll(ctx, cr, d)
	if err != nil {
		return r.fail(ctx, cr, d, err)
	}

	if err := r.patchStatus(ctx, cr, d, leftoverPG, nil); err != nil {
		log.Error(err, "unable to patch status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueStatus}, nil
}

func (r *ApmeReconciler) ensureAll(ctx context.Context, cr *apmev1alpha1.Apme, d resolve.Desired) (bool, error) {
	if err := r.applyCore(ctx, cr, d); err != nil {
		return false, err
	}
	leftover, err := r.ensureDatabase(ctx, cr, d)
	if err != nil {
		return leftover, err
	}
	if err := r.ensureAbbenay(ctx, cr, d); err != nil {
		return leftover, err
	}
	return leftover, r.applyWorkload(ctx, cr, d)
}

func (r *ApmeReconciler) applyCore(ctx context.Context, cr *apmev1alpha1.Apme, d resolve.Desired) error {
	for _, obj := range []client.Object{
		manifests.ServiceAccount(d),
		manifests.SessionsPVC(d),
		manifests.ProxyCachePVC(d),
	} {
		if err := r.apply(ctx, cr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (r *ApmeReconciler) ensureDatabase(ctx context.Context, cr *apmev1alpha1.Apme, d resolve.Desired) (bool, error) {
	if d.DatabaseMode != apmev1alpha1.DatabaseManaged {
		return r.managedPostgresExists(ctx, d), nil
	}
	if err := r.ensurePostgresSecret(ctx, cr, d); err != nil {
		return false, err
	}
	if err := r.apply(ctx, cr, postgres.StatefulSet(d)); err != nil {
		return false, err
	}
	if err := r.apply(ctx, cr, manifests.PostgresService(d)); err != nil {
		return false, err
	}
	if d.NetworkPolicy {
		return false, r.apply(ctx, cr, manifests.PostgresNetworkPolicy(d))
	}
	return false, nil
}

func (r *ApmeReconciler) ensureAbbenay(ctx context.Context, cr *apmev1alpha1.Apme, d resolve.Desired) error {
	if !d.Abbenay {
		return nil
	}
	if d.GenerateAbbenayToken {
		if err := r.ensureAbbenayToken(ctx, cr, d); err != nil {
			return err
		}
	}
	if d.AbbenayConfigMap == "" {
		if err := r.apply(ctx, cr, manifests.DefaultAbbenayConfigMap(d)); err != nil {
			return err
		}
	}
	if d.AbbenayPersist {
		return r.apply(ctx, cr, manifests.AbbenayPVC(d))
	}
	return nil
}

func (r *ApmeReconciler) applyWorkload(ctx context.Context, cr *apmev1alpha1.Apme, d resolve.Desired) error {
	sum := manifests.Checksum(d.DatabaseSecretName, d.DatabaseSecretKey, d.AbbenayTokenName, d.Version)
	objs := []client.Object{
		manifests.Deployment(d, sum),
		manifests.EngineService(d),
		manifests.GatewayService(d),
	}
	if d.UI {
		objs = append(objs, manifests.UIService(d))
	}
	if d.RouteEnabled && r.hasRouteAPI() {
		if d.UI {
			objs = append(objs, manifests.UIRoute(d))
		}
		objs = append(objs, manifests.APIRoute(d))
	}
	if d.IngressEnabled {
		objs = append(objs, manifests.Ingress(d))
	}
	if d.NetworkPolicy {
		objs = append(objs, manifests.APMENetworkPolicy(d))
	}
	for _, obj := range objs {
		if err := r.apply(ctx, cr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (r *ApmeReconciler) fail(ctx context.Context, cr *apmev1alpha1.Apme, d resolve.Desired, err error) (ctrl.Result, error) {
	_ = r.patchStatus(ctx, cr, d, false, err)
	return ctrl.Result{}, err
}

// applyPatch is server-side apply without using the deprecated client.Apply
// patch constant (SA1019 in controller-runtime >=0.23). Prefer this over
// ApplyConfigurationFromUnstructured for typed objects: converting API
// structs to unstructured cannot distinguish unset fields from explicit zeros.
type applyPatch struct{}

func (applyPatch) Type() types.PatchType { return types.ApplyPatchType }

func (applyPatch) Data(obj client.Object) ([]byte, error) {
	return json.Marshal(obj)
}

func (r *ApmeReconciler) apply(ctx context.Context, owner *apmev1alpha1.Apme, obj client.Object) error {
	if err := controllerutil.SetControllerReference(owner, obj, r.Scheme); err != nil {
		return err
	}
	return r.Patch(ctx, obj, applyPatch{}, client.ForceOwnership, client.FieldOwner(apmev1alpha1.FieldManager))
}

func (r *ApmeReconciler) ensurePostgresSecret(ctx context.Context, owner *apmev1alpha1.Apme, d resolve.Desired) error {
	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: d.DatabaseSecretName, Namespace: d.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	sec, err := postgres.NewSecret(d)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(owner, sec, r.Scheme); err != nil {
		return err
	}
	return r.Create(ctx, sec)
}

func (r *ApmeReconciler) ensureAbbenayToken(ctx context.Context, owner *apmev1alpha1.Apme, d resolve.Desired) error {
	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: d.AbbenayTokenName, Namespace: d.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	tok, err := randomToken(32)
	if err != nil {
		return err
	}
	sec := manifests.NewAbbenayTokenSecret(d, tok)
	if err := controllerutil.SetControllerReference(owner, sec, r.Scheme); err != nil {
		return err
	}
	return r.Create(ctx, sec)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

func (r *ApmeReconciler) managedPostgresExists(ctx context.Context, d resolve.Desired) bool {
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: d.Name + "-postgres", Namespace: d.Namespace}, sts)
	return err == nil
}

func (r *ApmeReconciler) hasRouteAPI() bool {
	return r.HasRouteAPI
}

func (r *ApmeReconciler) patchStatus(ctx context.Context, cr *apmev1alpha1.Apme, d resolve.Desired, leftoverPG bool, recErr error) error {
	orig := cr.DeepCopy()
	now := metav1.Now()
	st := cr.Status.DeepCopy()
	st.ObservedGeneration = cr.Generation
	st.Topology = apmev1alpha1.TopologySimple
	st.Database = d.DatabaseMode
	st.URL = r.publicURL(ctx, d)

	set := func(typ string, status metav1.ConditionStatus, reason, msg string) {
		apimeta.SetStatusCondition(&st.Conditions, metav1.Condition{
			Type:               typ,
			Status:             status,
			Reason:             reason,
			Message:            msg,
			LastTransitionTime: now,
			ObservedGeneration: cr.Generation,
		})
	}

	dbReady, dbMsg := r.databaseReady(ctx, d)
	if dbReady {
		set(condDatabaseReady, metav1.ConditionTrue, "Available", dbMsg)
	} else {
		set(condDatabaseReady, metav1.ConditionFalse, "Pending", dbMsg)
	}

	if leftoverPG {
		set(condDegraded, metav1.ConditionTrue, "LeftoverManagedPostgres",
			"connectionSecretRef is set but managed Postgres objects still exist; delete them manually")
	} else if recErr != nil {
		set(condDegraded, metav1.ConditionTrue, "ReconcileError", recErr.Error())
	} else {
		set(condDegraded, metav1.ConditionFalse, "AsExpected", "no errors")
	}

	depReady, depMsg := r.deploymentReady(ctx, d)
	if recErr != nil {
		set(condProgressing, metav1.ConditionTrue, "Retrying", recErr.Error())
		set(condReady, metav1.ConditionFalse, "Error", recErr.Error())
	} else if depReady && dbReady && !leftoverPG {
		set(condProgressing, metav1.ConditionFalse, "Stable", "all owned objects ready")
		set(condReady, metav1.ConditionTrue, "Available", "APME Simple topology is ready")
	} else {
		set(condProgressing, metav1.ConditionTrue, "Waiting", depMsg)
		set(condReady, metav1.ConditionFalse, "Waiting", depMsg)
	}

	cr.Status = *st
	return r.Status().Patch(ctx, cr, client.MergeFrom(orig))
}

func (r *ApmeReconciler) databaseReady(ctx context.Context, d resolve.Desired) (bool, string) {
	if d.DatabaseMode == apmev1alpha1.DatabaseExternal {
		sec := &corev1.Secret{}
		err := r.Get(ctx, types.NamespacedName{Name: d.DatabaseSecretName, Namespace: d.Namespace}, sec)
		if err != nil {
			return false, fmt.Sprintf("external database secret %q not found", d.DatabaseSecretName)
		}
		return true, "external secret present"
	}
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: d.Name + "-postgres", Namespace: d.Namespace}, sts)
	if err != nil {
		return false, "postgres StatefulSet not found"
	}
	if sts.Status.ReadyReplicas >= 1 {
		return true, "postgres ready"
	}
	return false, "waiting for postgres replica"
}

func (r *ApmeReconciler) deploymentReady(ctx context.Context, d resolve.Desired) (bool, string) {
	dep := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: d.Name, Namespace: d.Namespace}, dep)
	if err != nil {
		return false, "deployment not found"
	}
	if dep.Status.ReadyReplicas >= 1 {
		return true, "deployment ready"
	}
	return false, "waiting for APME deployment"
}

func (r *ApmeReconciler) publicURL(ctx context.Context, d resolve.Desired) string {
	if d.RouteEnabled && r.hasRouteAPI() {
		name := d.Name + "-ui"
		if !d.UI {
			name = d.Name + "-api"
		}
		rt := &unstructured.Unstructured{}
		rt.SetAPIVersion("route.openshift.io/v1")
		rt.SetKind("Route")
		if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: d.Namespace}, rt); err == nil {
			host, _, _ := unstructured.NestedString(rt.Object, "spec", "host")
			if ingress, found, _ := unstructured.NestedSlice(rt.Object, "status", "ingress"); found && len(ingress) > 0 {
				if m, ok := ingress[0].(map[string]interface{}); ok {
					if h, _ := m["host"].(string); h != "" {
						host = h
					}
				}
			}
			if host != "" {
				return "https://" + host
			}
		}
	}
	if d.IngressEnabled && d.IngressHost != "" {
		return "https://" + d.IngressHost
	}
	return ""
}

func (r *ApmeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&apmev1alpha1.Apme{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&networkingv1.Ingress{}).
		Named("apme")
	return b.Complete(r)
}
