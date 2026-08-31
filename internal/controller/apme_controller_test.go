package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apmev1alpha1 "github.com/ansible/apme-operator/api/v1alpha1"
)

func boolPtr(v bool) *bool { return &v }

func reconcileN(ctx context.Context, r *ApmeReconciler, nn types.NamespacedName, n int) {
	GinkgoHelper()
	for i := 0; i < n; i++ {
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	}
}

func newReconciler() *ApmeReconciler {
	return &ApmeReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), HasRouteAPI: false}
}

func gatewayEnv(dep *appsv1.Deployment) []corev1.EnvVar {
	for i := range dep.Spec.Template.Spec.Containers {
		c := dep.Spec.Template.Spec.Containers[i]
		if c.Name == "gateway" {
			return c.Env
		}
	}
	return nil
}

func hasContainer(dep *appsv1.Deployment, name string) bool {
	for _, c := range dep.Spec.Template.Spec.Containers {
		if c.Name == name {
			return true
		}
	}
	return false
}

var _ = Describe("Apme Controller", func() {
	ctx := context.Background()

	AfterEach(func() {
		By("removing leftover Apme CRs in default")
		list := &apmev1alpha1.ApmeList{}
		Expect(k8sClient.List(ctx, list, client.InNamespace("default"))).To(Succeed())
		for i := range list.Items {
			_ = k8sClient.Delete(ctx, &list.Items[i])
		}
	})

	It("creates managed Postgres and wires APME_DATABASE_URL", func() {
		nn := types.NamespacedName{Name: "apme-managed", Namespace: "default"}
		cr := &apmev1alpha1.Apme{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: apmev1alpha1.ApmeSpec{
				Exposure: apmev1alpha1.ExposureSpec{
					Route: apmev1alpha1.RouteSpec{Enabled: boolPtr(false)},
				},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		r := newReconciler()
		reconcileN(ctx, r, nn, 2)

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-postgres", Namespace: nn.Namespace}, &appsv1.StatefulSet{})).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-postgres", Namespace: nn.Namespace}, &corev1.Service{})).To(Succeed())
		sec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-postgres", Namespace: nn.Namespace}, sec)).To(Succeed())
		Expect(sec.Data["database-url"]).NotTo(BeEmpty())

		pvc := &corev1.PersistentVolumeClaim{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-gateway-data", Namespace: nn.Namespace}, pvc)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(*dep.Spec.Replicas).To(Equal(int32(1)))
		Expect(dep.Spec.Strategy.Type).To(Equal(appsv1.RecreateDeploymentStrategyType))
		found := false
		for _, e := range gatewayEnv(dep) {
			if e.Name == "APME_DATABASE_URL" {
				found = true
				Expect(e.ValueFrom.SecretKeyRef.Name).To(Equal(nn.Name + "-postgres"))
				Expect(e.ValueFrom.SecretKeyRef.Key).To(Equal("database-url"))
			}
			Expect(e.Name).NotTo(Equal("APME_DB_PATH"))
		}
		Expect(found).To(BeTrue())
		Expect(hasContainer(dep, "abbenay")).To(BeFalse())

		got := &apmev1alpha1.Apme{}
		Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
		Expect(got.Status.Topology).To(Equal(apmev1alpha1.TopologySimple))
		Expect(got.Status.Database).To(Equal(apmev1alpha1.DatabaseManaged))
		Expect(got.Status.ObservedGeneration).To(Equal(got.Generation))
	})

	It("adds Abbenay when enabled and skips it when disabled", func() {
		nn := types.NamespacedName{Name: "apme-ai", Namespace: "default"}
		cr := &apmev1alpha1.Apme{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: apmev1alpha1.ApmeSpec{
				Abbenay:  apmev1alpha1.AbbenaySpec{Enabled: true},
				Exposure: apmev1alpha1.ExposureSpec{Route: apmev1alpha1.RouteSpec{Enabled: boolPtr(false)}},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		r := newReconciler()
		reconcileN(ctx, r, nn, 1)

		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(hasContainer(dep, "abbenay")).To(BeTrue())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-abbenay", Namespace: nn.Namespace}, &corev1.Secret{})).To(Succeed())

		off := types.NamespacedName{Name: "apme-noai", Namespace: "default"}
		cr2 := &apmev1alpha1.Apme{
			ObjectMeta: metav1.ObjectMeta{Name: off.Name, Namespace: off.Namespace},
			Spec: apmev1alpha1.ApmeSpec{
				Exposure: apmev1alpha1.ExposureSpec{Route: apmev1alpha1.RouteSpec{Enabled: boolPtr(false)}},
			},
		}
		Expect(k8sClient.Create(ctx, cr2)).To(Succeed())
		reconcileN(ctx, r, off, 1)
		dep2 := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, off, dep2)).To(Succeed())
		Expect(hasContainer(dep2, "abbenay")).To(BeFalse())
		err := k8sClient.Get(ctx, types.NamespacedName{Name: off.Name + "-abbenay", Namespace: off.Namespace}, &corev1.Secret{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("creates no Postgres objects in external mode", func() {
		nn := types.NamespacedName{Name: "apme-ext", Namespace: "default"}
		userSec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "ext-db", Namespace: nn.Namespace},
			StringData: map[string]string{"database-url": "postgresql+asyncpg://apme:x@db:5432/apme"},
		}
		Expect(k8sClient.Create(ctx, userSec)).To(Succeed())
		cr := &apmev1alpha1.Apme{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: apmev1alpha1.ApmeSpec{
				Database: apmev1alpha1.DatabaseSpec{
					ConnectionSecretRef: apmev1alpha1.SecretKeyRef{Name: "ext-db", Key: "database-url"},
				},
				Exposure: apmev1alpha1.ExposureSpec{Route: apmev1alpha1.RouteSpec{Enabled: boolPtr(false)}},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		r := newReconciler()
		reconcileN(ctx, r, nn, 2)

		err := k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-postgres", Namespace: nn.Namespace}, &appsv1.StatefulSet{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-postgres", Namespace: nn.Namespace}, &corev1.Service{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		found := false
		for _, e := range gatewayEnv(dep) {
			if e.Name == "APME_DATABASE_URL" {
				found = true
				Expect(e.ValueFrom.SecretKeyRef.Name).To(Equal("ext-db"))
			}
		}
		Expect(found).To(BeTrue())

		got := &apmev1alpha1.Apme{}
		Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
		Expect(got.Status.Database).To(Equal(apmev1alpha1.DatabaseExternal))
	})

	It("does not delete managed Postgres on mode switch", func() {
		nn := types.NamespacedName{Name: "apme-switch", Namespace: "default"}
		cr := &apmev1alpha1.Apme{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: apmev1alpha1.ApmeSpec{
				Exposure: apmev1alpha1.ExposureSpec{Route: apmev1alpha1.RouteSpec{Enabled: boolPtr(false)}},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		r := newReconciler()
		reconcileN(ctx, r, nn, 1)
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-postgres", Namespace: nn.Namespace}, &appsv1.StatefulSet{})).To(Succeed())

		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "switch-db", Namespace: nn.Namespace},
			StringData: map[string]string{"database-url": "postgresql+asyncpg://apme:x@db:5432/apme"},
		})).To(Succeed())

		Expect(k8sClient.Get(ctx, nn, cr)).To(Succeed())
		cr.Spec.Database.ConnectionSecretRef = apmev1alpha1.SecretKeyRef{Name: "switch-db", Key: "database-url"}
		Expect(k8sClient.Update(ctx, cr)).To(Succeed())
		reconcileN(ctx, r, nn, 1)

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nn.Name + "-postgres", Namespace: nn.Namespace}, &appsv1.StatefulSet{})).To(Succeed())
		got := &apmev1alpha1.Apme{}
		Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
		Expect(got.Status.Database).To(Equal(apmev1alpha1.DatabaseExternal))
	})
})
