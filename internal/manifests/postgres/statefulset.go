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
					Containers: []corev1.Container{{
						Name:  "postgres",
						Image: d.PostgresImage,
						Ports: []corev1.ContainerPort{{Name: "postgres", ContainerPort: 5432}},
						Env: []corev1.EnvVar{
							secretEnv("POSTGRESQL_USER", d.DatabaseSecretName, "username"),
							secretEnv("POSTGRESQL_PASSWORD", d.DatabaseSecretName, "password"),
							secretEnv("POSTGRESQL_DATABASE", d.DatabaseSecretName, "database"),
						},
						VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/pgsql/data"}},
						Resources:    d.PostgresResources,
						ReadinessProbe: &corev1.Probe{
							ProbeHandler:  corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: port}},
							PeriodSeconds: 10,
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data", Labels: labels},
				Spec:       pvcSpec,
			}},
		},
	}
}
