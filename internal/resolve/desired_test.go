package resolve

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apmev1alpha1 "github.com/ansible/apme-operator/api/v1alpha1"
)

func TestFromManagedDefaults(t *testing.T) {
	d := From(&apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "ns"},
	})
	if d.DatabaseMode != apmev1alpha1.DatabaseManaged {
		t.Fatalf("mode=%s", d.DatabaseMode)
	}
	if d.Image("engine") != "quay.io/ansible/apme-engine:2026.8.10" {
		t.Fatalf("image=%s", d.Image("engine"))
	}
	if !d.UI || !d.Gitleaks || d.Abbenay {
		t.Fatalf("components ui=%v gitleaks=%v abbenay=%v", d.UI, d.Gitleaks, d.Abbenay)
	}
	if d.DatabaseSecretName != "apme-postgres" {
		t.Fatalf("secret=%s", d.DatabaseSecretName)
	}
	if !d.GeneratePostgresTLS || d.PostgresTLSSecretName != "apme-postgres-tls" {
		t.Fatalf("tls generate=%v secret=%s", d.GeneratePostgresTLS, d.PostgresTLSSecretName)
	}
}

func TestFromExternal(t *testing.T) {
	d := From(&apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "ns"},
		Spec: apmev1alpha1.ApmeSpec{
			Database: apmev1alpha1.DatabaseSpec{
				ConnectionSecretRef: apmev1alpha1.SecretKeyRef{Name: "ext", Key: "database-url"},
			},
		},
	})
	if d.DatabaseMode != apmev1alpha1.DatabaseExternal || d.GeneratePostgres {
		t.Fatalf("mode=%s generate=%v", d.DatabaseMode, d.GeneratePostgres)
	}
	if d.DatabaseSecretName != "ext" {
		t.Fatalf("secret=%s", d.DatabaseSecretName)
	}
}

func TestFromPluginsEmpty(t *testing.T) {
	d := From(&apmev1alpha1.Apme{
		ObjectMeta: metav1.ObjectMeta{Name: "apme", Namespace: "ns"},
	})
	if len(d.Plugins) != 0 {
		t.Fatalf("plugins=%v", d.Plugins)
	}
}

func TestResolvePluginsPorts(t *testing.T) {
	got := resolvePlugins([]apmev1alpha1.PluginSpec{
		{Name: "secscan", Image: "example/secscan:0.1"},
		{Name: "orgpolicy", Image: "example/orgpolicy:1", Port: 50110},
		{Name: "alpha", Image: "example/alpha:1"},
	})
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	// Sorted by name: alpha, orgpolicy, secscan
	if got[0].Name != "alpha" || got[0].Port != 50100 {
		t.Fatalf("alpha=%+v", got[0])
	}
	if got[1].Name != "orgpolicy" || got[1].Port != 50110 {
		t.Fatalf("orgpolicy=%+v", got[1])
	}
	if got[2].Name != "secscan" || got[2].Port != 50101 {
		t.Fatalf("secscan=%+v want port 50101 (skip 50110)", got[2])
	}
}

func TestResolvePluginsStable(t *testing.T) {
	in := []apmev1alpha1.PluginSpec{
		{Name: "b", Image: "b:1"},
		{Name: "a", Image: "a:1"},
	}
	first := resolvePlugins(in)
	second := resolvePlugins(in)
	if first[0].Port != second[0].Port || first[1].Port != second[1].Port {
		t.Fatalf("unstable: %+v vs %+v", first, second)
	}
	if first[0].Name != "a" || first[0].Port != 50100 || first[1].Port != 50101 {
		t.Fatalf("got=%+v", first)
	}
}

func TestResolvePluginsConfigMap(t *testing.T) {
	got := resolvePlugins([]apmev1alpha1.PluginSpec{
		{
			Name:         "orgpolicy",
			Image:        "example/p:1",
			ConfigMapRef: apmev1alpha1.LocalObjectRef{Name: "orgpolicy-data"},
		},
	})
	if got[0].ConfigMapName != "orgpolicy-data" || got[0].Port != 50100 {
		t.Fatalf("got=%+v", got[0])
	}
}
