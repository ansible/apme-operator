package manifests

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/ansible/apme-operator/internal/resolve"
)

func claim(name, component string, d resolve.Desired, size resource.Quantity, storageClass string) *corev1.PersistentVolumeClaim {
	spec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceStorage: size},
		},
	}
	if storageClass != "" {
		sc := storageClass
		spec.StorageClassName = &sc
	}
	return &corev1.PersistentVolumeClaim{
		TypeMeta:   typeMeta("PersistentVolumeClaim", "v1"),
		ObjectMeta: meta(name, d.Namespace, component, d),
		Spec:       spec,
	}
}

// SessionsPVC is the Engine session venv volume.
func SessionsPVC(d resolve.Desired) *corev1.PersistentVolumeClaim {
	return claim(d.Name+"-sessions", componentEngine, d, d.SessionsSize, d.SessionsStorageClass)
}

// ProxyCachePVC is the Galaxy Proxy wheel cache.
func ProxyCachePVC(d resolve.Desired) *corev1.PersistentVolumeClaim {
	return claim(d.Name+"-proxy-cache", componentEngine, d, d.ProxyCacheSize, d.ProxyCacheStorageClass)
}

// AbbenayPVC is the optional Abbenay config volume.
func AbbenayPVC(d resolve.Desired) *corev1.PersistentVolumeClaim {
	return claim(d.Name+"-abbenay-config", componentAbbenay, d, d.AbbenayPVCSize, d.AbbenayStorageClass)
}
