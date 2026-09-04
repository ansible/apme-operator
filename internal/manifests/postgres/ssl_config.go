package postgres

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ansible/apme-operator/internal/resolve"
)

// SSLConfigMap configures sclorg Postgres to enable TLS using mounted certs.
func SSLConfigMap(d resolve.Desired) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.Name + "-postgres-ssl",
			Namespace: d.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "apme",
				"app.kubernetes.io/instance":   d.Name,
				"app.kubernetes.io/managed-by": "apme-operator",
				"app.kubernetes.io/component":  "postgres",
			},
		},
		Data: map[string]string{
			"ssl.conf": `ssl = on
ssl_cert_file = '/opt/app-root/src/certs/tls.crt'
ssl_key_file = '/opt/app-root/src/certs/tls.key'
`,
		},
	}
}
