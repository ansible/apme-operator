package manifests

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ansible/apme-operator/internal/resolve"
)

func clusterIP(name, component string, d resolve.Desired, ports []corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		TypeMeta:   typeMeta("Service", "v1"),
		ObjectMeta: meta(name, d.Namespace, component, d),
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selectorLabels(d, componentEngine),
			Ports:    ports,
		},
	}
}

// GatewayService exposes HTTP :8080. gRPC 50060 stays pod-local.
func GatewayService(d resolve.Desired) *corev1.Service {
	return clusterIP(d.Name+"-gateway", componentGateway, d, []corev1.ServicePort{{
		Name: "http", Port: 8080, TargetPort: intstr.FromInt(8080), Protocol: corev1.ProtocolTCP,
	}})
}

// EngineService exposes engine gRPC and optional validator ports.
func EngineService(d resolve.Desired) *corev1.Service {
	ports := []corev1.ServicePort{{
		Name: "grpc-engine", Port: 50051, TargetPort: intstr.FromInt(50051), Protocol: corev1.ProtocolTCP,
	}}
	if d.CollectionHealth {
		ports = append(ports, corev1.ServicePort{
			Name: "grpc-collection-health", Port: 50058, TargetPort: intstr.FromInt(50058), Protocol: corev1.ProtocolTCP,
		})
	}
	if d.DepAudit {
		ports = append(ports, corev1.ServicePort{
			Name: "grpc-dep-audit", Port: 50059, TargetPort: intstr.FromInt(50059), Protocol: corev1.ProtocolTCP,
		})
	}
	return clusterIP(d.Name+"-engine", componentEngine, d, ports)
}

// UIService exposes UI :8081. Selector is the engine pod (sidecar).
func UIService(d resolve.Desired) *corev1.Service {
	return clusterIP(d.Name+"-ui", componentUI, d, []corev1.ServicePort{{
		Name: "http", Port: 8081, TargetPort: intstr.FromInt(8081), Protocol: corev1.ProtocolTCP,
	}})
}

// PostgresService is ClusterIP :5432, selector component=postgres.
func PostgresService(d resolve.Desired) *corev1.Service {
	svc := clusterIP(d.Name+"-postgres", componentPostgres, d, []corev1.ServicePort{{
		Name: "postgres", Port: 5432, TargetPort: intstr.FromInt(5432), Protocol: corev1.ProtocolTCP,
	}})
	svc.Spec.Selector = selectorLabels(d, componentPostgres)
	return svc
}
