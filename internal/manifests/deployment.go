package manifests

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ansible/apme-operator/internal/manifests/containers"
	"github.com/ansible/apme-operator/internal/resolve"
)

const checksumAnnotation = "apme.ansible.com/config-checksum"

// Deployment is the Simple all-in-one APME pod (replicas=1, Recreate).
func Deployment(d resolve.Desired, checksum string) *appsv1.Deployment {
	replicas := d.Replicas
	if replicas == 0 {
		replicas = 1
	}
	labels := selectorLabels(d, componentEngine)
	ann := map[string]string{checksumAnnotation: checksum}

	cs := []corev1.Container{
		containers.Engine(d),
		containers.Native(d),
		containers.OPA(d),
		containers.Ansible(d),
	}
	if d.Gitleaks {
		cs = append(cs, containers.Gitleaks(d))
	}
	if d.CollectionHealth {
		cs = append(cs, containers.CollectionHealth(d))
	}
	if d.DepAudit {
		cs = append(cs, containers.DepAudit(d))
	}
	cs = append(cs, containers.GalaxyProxy(d), containers.Gateway(d))
	if d.UI {
		cs = append(cs, containers.UI(d))
	}
	if d.Abbenay {
		cs = append(cs, containers.Abbenay(d))
	}

	var inits []corev1.Container
	if d.GeneratePostgres {
		inits = append(inits, containers.InitDBCABundle(d))
	}
	if d.UI {
		inits = append(inits, containers.InitNginx(d))
	}
	if d.Abbenay {
		inits = append(inits, containers.InitAbbenayConfig(d))
	}

	vols := []corev1.Volume{
		pvcVol("sessions", d.Name+"-sessions"),
		pvcVol("proxy-cache", d.Name+"-proxy-cache"),
	}
	if d.GeneratePostgres {
		vols = append(vols,
			corev1.Volume{
				Name: "db-ca",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: d.PostgresTLSSecretName,
						Items:      []corev1.KeyToPath{{Key: "ca.crt", Path: "ca.crt"}},
					},
				},
			},
			emptyVol("db-ca-bundle"),
		)
	}
	if d.UI {
		vols = append(vols,
			emptyVol("nginx-cache"),
			emptyVol("nginx-run"),
			emptyVol("nginx-conf"),
		)
	}
	if d.Abbenay {
		seed := d.Name + "-abbenay-config"
		if d.AbbenayConfigMap != "" {
			seed = d.AbbenayConfigMap
		}
		vols = append(vols,
			corev1.Volume{
				Name: "abbenay-config-seed",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: seed},
					},
				},
			},
			emptyVol("abbenay-run"),
		)
		if d.AbbenayPersist {
			vols = append(vols, pvcVol("abbenay-config", d.Name+"-abbenay-config"))
		} else {
			vols = append(vols, emptyVol("abbenay-config"))
		}
	}

	return &appsv1.Deployment{
		TypeMeta:   typeMeta("Deployment", "apps/v1"),
		ObjectMeta: meta(d.Name, d.Namespace, componentEngine, d),
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels, Annotations: ann},
				Spec: corev1.PodSpec{
					ServiceAccountName: d.Name,
					SecurityContext:    &corev1.PodSecurityContext{},
					ImagePullSecrets:   d.PullSecrets,
					InitContainers:     inits,
					Containers:         cs,
					Volumes:            vols,
				},
			},
		},
	}
}

func pvcVol(name, claim string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim},
		},
	}
}

func emptyVol(name string) corev1.Volume {
	return corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}
}

// DefaultAbbenayConfigMap is a minimal seed when the user did not supply configMapRef.
func DefaultAbbenayConfigMap(d resolve.Desired) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   typeMeta("ConfigMap", "v1"),
		ObjectMeta: meta(d.Name+"-abbenay-config", d.Namespace, componentAbbenay, d),
		Data: map[string]string{
			"config.yaml": "providers: []\n",
		},
	}
}

// NewAbbenayTokenSecret generates the once-created Abbenay token.
func NewAbbenayTokenSecret(d resolve.Desired, token string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta:   typeMeta("Secret", "v1"),
		ObjectMeta: meta(d.Name+"-abbenay", d.Namespace, componentAbbenay, d),
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{d.AbbenayTokenKey: token},
	}
}
