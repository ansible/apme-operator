package postgres

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/ansible/apme-operator/internal/resolve"
)

func testDesired() resolve.Desired {
	return resolve.Desired{
		Name:                  "apme",
		Namespace:             "apme-dev",
		PostgresTLSSecretName: "apme-postgres-tls",
		PostgresSize:          resource.MustParse("1Gi"),
	}
}

func TestNewTLSSecretSANs(t *testing.T) {
	sec, err := NewTLSSecret(testDesired())
	if err != nil {
		t.Fatal(err)
	}
	if !HasTLSMaterial(sec) {
		t.Fatal("missing tls material")
	}
	block, _ := pem.Decode(sec.Data["tls.crt"])
	if block == nil {
		t.Fatal("tls.crt not PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"apme-postgres",
		"apme-postgres.apme-dev",
		"apme-postgres.apme-dev.svc",
		"apme-postgres.apme-dev.svc.cluster.local",
	}
	for _, h := range want {
		found := false
		for _, dns := range cert.DNSNames {
			if dns == h {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing SAN %q in %v", h, cert.DNSNames)
		}
	}
}

func TestWithVerifyFullTLS(t *testing.T) {
	got, err := WithVerifyFullTLS("postgresql+asyncpg://apme:x@apme-postgres.apme-dev.svc:5432/apme")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "ssl=verify-full") {
		t.Fatalf("expected ssl=verify-full in %q", got)
	}
	if strings.Contains(got, "sslmode=") {
		t.Fatalf("did not expect sslmode in %q", got)
	}
	got2, err := WithVerifyFullTLS(got)
	if err != nil {
		t.Fatal(err)
	}
	if got2 != got {
		t.Fatalf("idempotent expected, got %q vs %q", got2, got)
	}
	fromLibpq, err := WithVerifyFullTLS("postgresql+asyncpg://apme:x@apme-postgres.apme-dev.svc:5432/apme?sslmode=verify-full")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fromLibpq, "ssl=verify-full") || strings.Contains(fromLibpq, "sslmode=") {
		t.Fatalf("expected sslmode remapped to ssl only, got %q", fromLibpq)
	}
}

func TestNewSecretIncludesVerifyFull(t *testing.T) {
	d := testDesired()
	d.DatabaseSecretName = "apme-postgres"
	sec, err := NewSecret(d)
	if err != nil {
		t.Fatal(err)
	}
	u := sec.StringData["database-url"]
	if !strings.Contains(u, "ssl=verify-full") {
		t.Fatalf("url missing ssl=: %s", u)
	}
	if strings.Contains(u, "sslmode=") {
		t.Fatalf("did not expect sslmode in %s", u)
	}
}
