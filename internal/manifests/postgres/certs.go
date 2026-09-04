package postgres

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ansible/apme-operator/internal/resolve"
)

const (
	tlsCRT = "tls.crt"
	tlsKey = "tls.key"
	caCRT  = "ca.crt"
	caKey  = "ca.key"
)

// ServiceHostnames returns DNS names that must appear on the Postgres server certificate.
func ServiceHostnames(d resolve.Desired) []string {
	base := fmt.Sprintf("%s-postgres", d.Name)
	return []string{
		base,
		fmt.Sprintf("%s.%s", base, d.Namespace),
		fmt.Sprintf("%s.%s.svc", base, d.Namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", base, d.Namespace),
	}
}

// NewTLSSecret generates a self-signed CA and a server certificate for managed Postgres.
func NewTLSSecret(d resolve.Desired) (*corev1.Secret, error) {
	caKeyPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}
	serverKeyPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate server key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("CA serial: %w", err)
	}
	now := time.Now().Add(-time.Hour)
	caTpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: fmt.Sprintf("%s-postgres-ca", d.Name)},
		NotBefore:             now,
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKeyPriv.PublicKey, caKeyPriv)
	if err != nil {
		return nil, fmt.Errorf("create CA cert: %w", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, fmt.Errorf("parse CA cert: %w", err)
	}

	serial, err = rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("server serial: %w", err)
	}
	hosts := ServiceHostnames(d)
	serverTpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: hosts[2]}, // name-postgres.ns.svc
		NotBefore:    now,
		NotAfter:     now.Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     hosts,
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTpl, caCert, &serverKeyPriv.PublicKey, caKeyPriv)
	if err != nil {
		return nil, fmt.Errorf("create server cert: %w", err)
	}

	caKeyPEM, err := pemEncodeECKey(caKeyPriv)
	if err != nil {
		return nil, err
	}
	serverKeyPEM, err := pemEncodeECKey(serverKeyPriv)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.PostgresTLSSecretName,
			Namespace: d.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "apme",
				"app.kubernetes.io/instance":   d.Name,
				"app.kubernetes.io/managed-by": "apme-operator",
				"app.kubernetes.io/component":  "postgres",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			tlsCRT: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}),
			tlsKey: serverKeyPEM,
			caCRT:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
			caKey:  caKeyPEM,
		},
	}, nil
}

func pemEncodeECKey(key *ecdsa.PrivateKey) ([]byte, error) {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal EC key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b}), nil
}

// HasTLSMaterial reports whether secret has tls.crt, tls.key, and ca.crt.
func HasTLSMaterial(sec *corev1.Secret) bool {
	if sec == nil {
		return false
	}
	return len(sec.Data[tlsCRT]) > 0 && len(sec.Data[tlsKey]) > 0 && len(sec.Data[caCRT]) > 0
}
