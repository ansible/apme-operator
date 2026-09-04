package postgres

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ansible/apme-operator/internal/resolve"
)

const (
	pgUser = "apme"
	pgDB   = "apme"
)

// NewSecret generates the once-created Postgres credentials Secret.
func NewSecret(d resolve.Desired) (*corev1.Secret, error) {
	pw, err := randomPassword(24)
	if err != nil {
		return nil, err
	}
	host := fmt.Sprintf("%s-postgres.%s.svc", d.Name, d.Namespace)
	u := &url.URL{
		Scheme: "postgresql+asyncpg",
		User:   url.UserPassword(pgUser, pw),
		Host:   fmt.Sprintf("%s:5432", host),
		Path:   "/" + pgDB,
		// Use asyncpg's ssl= query param (not libpq sslmode=). The APME gateway
		// remaps sslmode→ssl via SQLAlchemy URL mutation; str(URL) redacts the
		// password to "***" and breaks authentication. ssl=verify-full needs no remap.
		RawQuery: "ssl=verify-full",
	}
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.DatabaseSecretName,
			Namespace: d.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "apme",
				"app.kubernetes.io/instance":   d.Name,
				"app.kubernetes.io/managed-by": "apme-operator",
				"app.kubernetes.io/component":  "postgres",
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"username":     pgUser,
			"password":     pw,
			"database":     pgDB,
			"database-url": u.String(),
		},
	}, nil
}

func randomPassword(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

// WithVerifyFullTLS returns databaseURL with ssl=verify-full set (preserving other query params).
// Prefer asyncpg's ssl= over libpq sslmode= so APME gateway URL normalization is a no-op
// (that path uses str(URL), which redacts passwords under SQLAlchemy 2).
func WithVerifyFullTLS(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Del("sslmode")
	q.Set("ssl", "verify-full")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func secretEnv(name, secret, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secret},
				Key:                  key,
			},
		},
	}
}

// StatefulSet is a single-replica Postgres with a volumeClaimTemplate.
func StatefulSet(d resolve.Desired) *appsv1.StatefulSet {
	replicas := int32(1)
	labels := map[string]string{
		"app.kubernetes.io/name":       "apme",
		"app.kubernetes.io/instance":   d.Name,
		"app.kubernetes.io/managed-by": "apme-operator",
		"app.kubernetes.io/component":  "postgres",
	}
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceStorage: d.PostgresSize},
		},
	}
	if d.PostgresStorageClass != "" {
		sc := d.PostgresStorageClass
		pvcSpec.StorageClassName = &sc
	}
	port := intstr.FromInt32(5432)
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.Name + "-postgres",
			Namespace: d.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: d.Name + "-postgres",
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					SecurityContext:  &corev1.PodSecurityContext{},
					ImagePullSecrets: d.PullSecrets,
					InitContainers: []corev1.Container{{
						Name:            "postgres-tls-perms",
						Image:           d.PostgresImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command: []string{"/bin/sh", "-c",
							"cp /tls-src/tls.crt /tls-src/tls.key /certs/ && chmod 600 /certs/tls.key && chmod 644 /certs/tls.crt",
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "postgres-tls-src", MountPath: "/tls-src", ReadOnly: true},
							{Name: "postgres-certs", MountPath: "/certs"},
						},
					}},
					Containers: []corev1.Container{{
						Name:  "postgres",
						Image: d.PostgresImage,
						Ports: []corev1.ContainerPort{{Name: "postgres", ContainerPort: 5432}},
						Env: []corev1.EnvVar{
							secretEnv("POSTGRESQL_USER", d.DatabaseSecretName, "username"),
							secretEnv("POSTGRESQL_PASSWORD", d.DatabaseSecretName, "password"),
							secretEnv("POSTGRESQL_DATABASE", d.DatabaseSecretName, "database"),
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: "/var/lib/pgsql/data"},
							{Name: "postgres-certs", MountPath: "/opt/app-root/src/certs", ReadOnly: true},
							{Name: "postgres-ssl-cfg", MountPath: "/opt/app-root/src/postgresql-cfg", ReadOnly: true},
						},
						Resources: d.PostgresResources,
						ReadinessProbe: &corev1.Probe{
							ProbeHandler:  corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: port}},
							PeriodSeconds: 10,
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "postgres-tls-src",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: d.PostgresTLSSecretName},
							},
						},
						{Name: "postgres-certs", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{
							Name: "postgres-ssl-cfg",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: d.Name + "-postgres-ssl"},
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data", Labels: labels},
				Spec:       pvcSpec,
			}},
		},
	}
}
