package manifests

import (
	networkingv1 "k8s.io/api/networking/v1"

	"github.com/ansible/apme-operator/internal/resolve"
)

// Ingress is optional vanilla-k8s exposure.
func Ingress(d resolve.Desired) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	var paths []networkingv1.HTTPIngressPath
	if d.UI {
		paths = []networkingv1.HTTPIngressPath{
			httpPath("/", pathType, d.Name+"-ui", 8081),
			httpPath("/api", pathType, d.Name+"-gateway", 8080),
		}
	} else {
		paths = []networkingv1.HTTPIngressPath{
			httpPath("/", pathType, d.Name+"-gateway", 8080),
		}
	}
	ing := &networkingv1.Ingress{
		TypeMeta:   typeMeta("Ingress", "networking.k8s.io/v1"),
		ObjectMeta: meta(d.Name, d.Namespace, componentEngine, d),
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: d.IngressHost,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{Paths: paths},
				},
			}},
		},
	}
	if d.IngressClassName != "" {
		ing.Spec.IngressClassName = &d.IngressClassName
	}
	if len(d.IngressAnnotations) > 0 {
		ing.ObjectMeta.Annotations = d.IngressAnnotations
	}
	if d.IngressTLSSecretName != "" && d.IngressHost != "" {
		ing.Spec.TLS = []networkingv1.IngressTLS{{
			Hosts:      []string{d.IngressHost},
			SecretName: d.IngressTLSSecretName,
		}}
	}
	return ing
}

func httpPath(path string, pt networkingv1.PathType, svc string, port int32) networkingv1.HTTPIngressPath {
	return networkingv1.HTTPIngressPath{
		Path:     path,
		PathType: &pt,
		Backend: networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: svc,
				Port: networkingv1.ServiceBackendPort{Number: port},
			},
		},
	}
}
