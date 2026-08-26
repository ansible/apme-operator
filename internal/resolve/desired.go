package resolve

import (
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	apmev1alpha1 "github.com/ansible/apme-operator/api/v1alpha1"
)

// Desired is the fully defaulted intent for one Apme instance.
type Desired struct {
	Name      string
	Namespace string

	Version     string
	Registry    string
	PullPolicy  corev1.PullPolicy
	PullSecrets []corev1.LocalObjectReference
	Replicas    int32

	Gitleaks         bool
	CollectionHealth bool
	DepAudit         bool
	UI               bool

	Abbenay              bool
	AbbenayImage         string
	AbbenayTokenName     string
	AbbenayTokenKey      string
	GenerateAbbenayToken bool
	AbbenayConfigMap     string
	AbbenayPersist       bool
	AbbenayPVCSize       resource.Quantity
	AbbenayStorageClass  string

	DatabaseMode         apmev1alpha1.DatabaseMode
	DatabaseSecretName   string
	DatabaseSecretKey    string
	GeneratePostgres     bool
	PostgresImage        string
	PostgresSize         resource.Quantity
	PostgresStorageClass string
	PostgresResources    corev1.ResourceRequirements

	SessionsSize           resource.Quantity
	SessionsStorageClass   string
	ProxyCacheSize         resource.Quantity
	ProxyCacheStorageClass string

	RouteEnabled                       bool
	RouteHost                          string
	RouteTermination                   string
	RouteInsecureEdgeTerminationPolicy string
	IngressEnabled                     bool
	IngressClassName                   string
	IngressHost                        string
	IngressAnnotations                 map[string]string
	IngressTLSSecretName               string

	NetworkPolicy bool
	Resources     corev1.ResourceRequirements
	ProxyEnv      []corev1.EnvVar
}

func ptrBool(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func qtyOr(q resource.Quantity, def string) resource.Quantity {
	if q.IsZero() {
		return resource.MustParse(def)
	}
	return q
}

// From applies v1 defaults to an Apme CR.
func From(cr *apmev1alpha1.Apme) Desired {
	d := Desired{
		Name:                 cr.Name,
		Namespace:            cr.Namespace,
		Version:              cr.Spec.Version,
		Registry:             cr.Spec.Image.Registry,
		PullPolicy:           cr.Spec.Image.PullPolicy,
		PullSecrets:          cr.Spec.Image.PullSecrets,
		Replicas:             cr.Spec.Replicas,
		Gitleaks:             ptrBool(cr.Spec.Components.Gitleaks, true),
		CollectionHealth:     ptrBool(cr.Spec.Components.CollectionHealth, true),
		DepAudit:             ptrBool(cr.Spec.Components.DepAudit, true),
		UI:                   ptrBool(cr.Spec.Components.UI, true),
		Abbenay:              cr.Spec.Abbenay.Enabled,
		AbbenayImage:         cr.Spec.Abbenay.Image,
		AbbenayTokenName:     cr.Spec.Abbenay.TokenSecretRef.Name,
		AbbenayTokenKey:      cr.Spec.Abbenay.TokenSecretRef.Key,
		AbbenayConfigMap:     cr.Spec.Abbenay.ConfigMapRef.Name,
		NetworkPolicy:        ptrBool(cr.Spec.NetworkPolicy.Enabled, true),
		Resources:            cr.Spec.Resources,
		RouteHost:            cr.Spec.Exposure.Route.Host,
		IngressEnabled:       cr.Spec.Exposure.Ingress.Enabled,
		IngressClassName:     cr.Spec.Exposure.Ingress.ClassName,
		IngressHost:          cr.Spec.Exposure.Ingress.Host,
		IngressAnnotations:   cr.Spec.Exposure.Ingress.Annotations,
		IngressTLSSecretName: cr.Spec.Exposure.Ingress.TLSSecretName,
	}

	if d.Version == "" {
		d.Version = apmev1alpha1.DefaultVersion
	}
	if d.Registry == "" {
		d.Registry = apmev1alpha1.DefaultRegistry
	}
	if d.PullPolicy == "" {
		d.PullPolicy = corev1.PullIfNotPresent
	}
	if d.Replicas == 0 {
		d.Replicas = 1
	}

	d.RouteEnabled = ptrBool(cr.Spec.Exposure.Route.Enabled, true)
	d.RouteTermination = cr.Spec.Exposure.Route.TLS.Termination
	if d.RouteTermination == "" {
		d.RouteTermination = "edge"
	}
	d.RouteInsecureEdgeTerminationPolicy = cr.Spec.Exposure.Route.TLS.InsecureEdgeTerminationPolicy
	if d.RouteInsecureEdgeTerminationPolicy == "" {
		d.RouteInsecureEdgeTerminationPolicy = "Redirect"
	}

	d.SessionsSize = qtyOr(cr.Spec.Storage.Sessions.Size, "10Gi")
	d.SessionsStorageClass = cr.Spec.Storage.Sessions.StorageClass
	d.ProxyCacheSize = qtyOr(cr.Spec.Storage.ProxyCache.Size, "10Gi")
	d.ProxyCacheStorageClass = cr.Spec.Storage.ProxyCache.StorageClass

	if cr.Spec.Database.ConnectionSecretRef.Name != "" {
		d.DatabaseMode = apmev1alpha1.DatabaseExternal
		d.DatabaseSecretName = cr.Spec.Database.ConnectionSecretRef.Name
		d.DatabaseSecretKey = cr.Spec.Database.ConnectionSecretRef.Key
		if d.DatabaseSecretKey == "" {
			d.DatabaseSecretKey = "database-url"
		}
	} else {
		d.DatabaseMode = apmev1alpha1.DatabaseManaged
		d.GeneratePostgres = true
		d.DatabaseSecretName = cr.Name + "-postgres"
		d.DatabaseSecretKey = "database-url"
		d.PostgresImage = cr.Spec.Database.Postgres.Image
		if d.PostgresImage == "" {
			d.PostgresImage = apmev1alpha1.DefaultPostgresImage
		}
		d.PostgresSize = qtyOr(cr.Spec.Database.Postgres.Storage.Size, "10Gi")
		d.PostgresStorageClass = cr.Spec.Database.Postgres.Storage.StorageClass
		d.PostgresResources = cr.Spec.Database.Postgres.Resources
	}

	if d.Abbenay {
		if d.AbbenayImage == "" {
			d.AbbenayImage = apmev1alpha1.DefaultAbbenayImage
		}
		if d.AbbenayTokenKey == "" {
			d.AbbenayTokenKey = "token"
		}
		if d.AbbenayTokenName == "" {
			d.GenerateAbbenayToken = true
			d.AbbenayTokenName = cr.Name + "-abbenay"
		}
		d.AbbenayPersist = ptrBool(cr.Spec.Abbenay.Persistence.Enabled, true)
		d.AbbenayPVCSize = qtyOr(cr.Spec.Abbenay.Persistence.Size, "1Gi")
		d.AbbenayStorageClass = cr.Spec.Abbenay.Persistence.StorageClass
	}

	d.ProxyEnv = clusterProxyEnv()
	return d
}

func clusterProxyEnv() []corev1.EnvVar {
	var out []corev1.EnvVar
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"} {
		if v := os.Getenv(k); strings.TrimSpace(v) != "" {
			out = append(out, corev1.EnvVar{Name: k, Value: v})
		}
	}
	return out
}

// Image returns registry/apme-{component}:version.
func (d Desired) Image(component string) string {
	return d.Registry + "/apme-" + component + ":" + d.Version
}
