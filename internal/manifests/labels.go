package manifests

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ansible/apme-operator/internal/resolve"
)

const (
	appNameLabel      = "app.kubernetes.io/name"
	appInstanceLabel  = "app.kubernetes.io/instance"
	appManagedByLabel = "app.kubernetes.io/managed-by"
	appComponentLabel = "app.kubernetes.io/component"
	managedBy         = "apme-operator"
	componentEngine   = "engine"
	componentGateway  = "gateway"
	componentUI       = "ui"
	componentPostgres = "postgres"
	componentAbbenay  = "abbenay"
)

func meta(name, ns, component string, d resolve.Desired) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: ns,
		Labels:    labels(d, component),
	}
}

func labels(d resolve.Desired, component string) map[string]string {
	return map[string]string{
		appNameLabel:      "apme",
		appInstanceLabel:  d.Name,
		appManagedByLabel: managedBy,
		appComponentLabel: component,
	}
}

func selectorLabels(d resolve.Desired, component string) map[string]string {
	return map[string]string{
		appNameLabel:      "apme",
		appInstanceLabel:  d.Name,
		appComponentLabel: component,
	}
}

// Checksum is a stable annotation value so the Deployment rolls when inputs change.
// Uses FNV-1a (not a password hash): inputs are names/keys/version strings for
// change detection, not secret material.
func Checksum(parts ...string) string {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = fmt.Fprintln(h, p)
	}
	return hex.EncodeToString(h.Sum(nil))
}
