package manifests

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ansible/apme-operator/internal/resolve"
)

func tcpNPPort(p int32) networkingv1.NetworkPolicyPort {
	proto := corev1.ProtocolTCP
	port := intstr.FromInt32(p)
	return networkingv1.NetworkPolicyPort{Protocol: &proto, Port: &port}
}

func labelSelector(m map[string]string) metav1.LabelSelector {
	return metav1.LabelSelector{MatchLabels: m}
}

// APMENetworkPolicy allows ingress only to Gateway :8080 and UI :8081.
func APMENetworkPolicy(d resolve.Desired) *networkingv1.NetworkPolicy {
	ingress := []networkingv1.NetworkPolicyIngressRule{
		{Ports: []networkingv1.NetworkPolicyPort{tcpNPPort(8080)}},
	}
	if d.UI {
		ingress = append(ingress, networkingv1.NetworkPolicyIngressRule{
			Ports: []networkingv1.NetworkPolicyPort{tcpNPPort(8081)},
		})
	}
	return &networkingv1.NetworkPolicy{
		TypeMeta:   typeMeta("NetworkPolicy", "networking.k8s.io/v1"),
		ObjectMeta: meta(d.Name+"-engine", d.Namespace, componentEngine, d),
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: labelSelector(selectorLabels(d, componentEngine)),
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     ingress,
		},
	}
}

// PostgresNetworkPolicy allows :5432 only from APME engine pods.
func PostgresNetworkPolicy(d resolve.Desired) *networkingv1.NetworkPolicy {
	from := labelSelector(selectorLabels(d, componentEngine))
	return &networkingv1.NetworkPolicy{
		TypeMeta:   typeMeta("NetworkPolicy", "networking.k8s.io/v1"),
		ObjectMeta: meta(d.Name+"-postgres", d.Namespace, componentPostgres, d),
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: labelSelector(selectorLabels(d, componentPostgres)),
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &from}},
				Ports: []networkingv1.NetworkPolicyPort{tcpNPPort(5432)},
			}},
		},
	}
}
