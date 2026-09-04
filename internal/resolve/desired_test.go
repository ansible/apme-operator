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
