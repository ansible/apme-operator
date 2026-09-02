package manifests

import (
	"github.com/ansible/apme-operator/internal/resolve"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RouteDesiredHostAnnotation records the CR-desired Route host (may be empty).
// Used to detect host intent changes without confusing OpenShift-assigned hosts.
const RouteDesiredHostAnnotation = "apme.ansible.com/desired-host"

func routeUnstructured(name, component, host, svc, path string, d resolve.Desired) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"to": map[string]interface{}{
			"kind":   "Service",
			"name":   svc,
			"weight": int64(100),
		},
		"port": map[string]interface{}{
			"targetPort": "http",
		},
		"tls": map[string]interface{}{
			"termination":                   d.RouteTermination,
			"insecureEdgeTerminationPolicy": d.RouteInsecureEdgeTerminationPolicy,
		},
	}
	// Omit empty host so OpenShift can assign *.apps.<domain>.
	if host != "" {
		spec["host"] = host
	}
	if path != "" {
		spec["path"] = path
	}
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "route.openshift.io/v1",
		"kind":       "Route",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": d.Namespace,
			"labels":    toUnstructuredStringMap(labels(d, component)),
			"annotations": map[string]interface{}{
				RouteDesiredHostAnnotation: host,
			},
		},
		"spec": spec,
	}}
	return u
}

func toUnstructuredStringMap(in map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// UIRoute is the UI Route at / when UI is on.
func UIRoute(d resolve.Desired) *unstructured.Unstructured {
	return routeUnstructured(d.Name+"-ui", componentUI, d.RouteHost, d.Name+"-ui", "", d)
}

// APIRoute is Gateway. Path /api when UI is on so both share one host.
func APIRoute(d resolve.Desired) *unstructured.Unstructured {
	path := ""
	if d.UI {
		path = "/api"
	}
	return routeUnstructured(d.Name+"-api", componentGateway, d.RouteHost, d.Name+"-gateway", path, d)
}
