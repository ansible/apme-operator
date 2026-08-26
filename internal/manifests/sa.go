package manifests

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ansible/apme-operator/internal/resolve"
)

func typeMeta(kind, apiVersion string) metav1.TypeMeta {
	return metav1.TypeMeta{APIVersion: apiVersion, Kind: kind}
}

// ServiceAccount is the dedicated operand SA (never default).
func ServiceAccount(d resolve.Desired) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta:   typeMeta("ServiceAccount", "v1"),
		ObjectMeta: meta(d.Name, d.Namespace, componentEngine, d),
	}
}
