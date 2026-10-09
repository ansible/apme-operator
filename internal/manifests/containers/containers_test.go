package containers

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apmev1alpha1 "github.com/ansible/apme-operator/api/v1alpha1"
	"github.com/ansible/apme-operator/internal/resolve"
)

func TestEngineGalaxyProxyAndGatewayShareAdminToken(t *testing.T) {
	for _, abbenay := range []bool{false, true} {
		cr := &apmev1alpha1.Apme{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"}}
		cr.Spec.Abbenay.Enabled = abbenay
		d := resolve.From(cr)
		for _, c := range []corev1.Container{Engine(d), GalaxyProxy(d), Gateway(d)} {
			found := false
			for _, e := range c.Env {
				if e.Name == "APME_PROXY_ALLOW_UNAUTH_ADMIN" {
					t.Fatal("operator must not disable proxy admin authentication")
				}
				if e.Name != "APME_PROXY_ADMIN_TOKEN" {
					continue
				}
				found = true
				if e.Value != "" || e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
					t.Fatalf("%s admin token must use a Secret reference", c.Name)
				}
				ref := e.ValueFrom.SecretKeyRef
				if ref.Name != "test-proxy-admin" || ref.Key != "token" {
					t.Fatalf("%s admin token reference = %s/%s", c.Name, ref.Name, ref.Key)
				}
			}
			if !found {
				t.Fatalf("%s is missing proxy admin authentication (abbenay=%t)", c.Name, abbenay)
			}
		}
	}
}

func TestAbbenayProbesUseBinaryStatus(t *testing.T) {
	c := Abbenay(resolve.Desired{
		AbbenayImage:     "ghcr.io/redhat-developer/abbenay:v2026.8.7",
		AbbenayTokenName: "tok",
		AbbenayTokenKey:  "token",
	})
	want := []string{"/opt/abbenay/abbenay", "status"}
	for _, p := range []*struct {
		name string
		cmd  []string
	}{
		{"readiness", c.ReadinessProbe.Exec.Command},
		{"liveness", c.LivenessProbe.Exec.Command},
	} {
		if len(p.cmd) != len(want) || p.cmd[0] != want[0] || p.cmd[1] != want[1] {
			t.Fatalf("%s probe = %v, want %v (Abbenay image has no node binary)", p.name, p.cmd, want)
		}
		for _, arg := range p.cmd {
			if arg == "node" {
				t.Fatalf("%s probe must not invoke node: %v", p.name, p.cmd)
			}
		}
	}
}

func TestGalaxyTLSConfiguration(t *testing.T) {
	yes, no := true, false
	for _, verify := range []*bool{nil, &yes, &no} {
		cr := &apmev1alpha1.Apme{}
		cr.Spec.GalaxyProxy.TLS.Verify = verify
		cr.Spec.GalaxyProxy.TLS.CABundleConfigMapRef = &apmev1alpha1.ConfigMapKeyRef{Name: "hub-ca"}
		d := resolve.From(cr)
		if d.GalaxyCAConfigMap != "hub-ca" || d.GalaxyCAKey != "ca-bundle.crt" {
			t.Fatalf("CA reference defaults = %s/%s", d.GalaxyCAConfigMap, d.GalaxyCAKey)
		}
		c := GalaxyProxy(d)
		seen := false
		for _, e := range c.Env {
			if e.Name == "ANSIBLE_GALAXY_IGNORE" {
				seen = true
				if verify == nil || e.Value != map[bool]string{true: "false", false: "true"}[*verify] {
					t.Fatalf("unexpected TLS env: %v", e)
				}
			}
			if e.Name == "SSL_CERT_FILE" && e.Value != "/etc/apme/galaxy-ca/ca-bundle.crt" {
				t.Fatalf("unexpected CA path: %s", e.Value)
			}
		}
		if seen != (verify != nil) {
			t.Fatalf("TLS default override presence=%t", seen)
		}
		if len(InitGalaxyCABundle(d).VolumeMounts) != 2 {
			t.Fatal("CA initializer must mount source and destination")
		}
	}
}

func TestEnginePluginEnv(t *testing.T) {
	c := Engine(resolve.Desired{
		Plugins: []resolve.ResolvedPlugin{
			{Name: "orgpolicy", Image: "example/p:1", Port: 50100},
		},
	})
	found := false
	for _, e := range c.Env {
		if e.Name == "APME_PLUGIN_ORGPOLICY_ADDRESS" {
			found = true
			if e.Value != "127.0.0.1:50100" {
				t.Fatalf("value=%s", e.Value)
			}
		}
		if len(e.Name) > 12 && e.Name[:12] == "APME_PLUGIN_" && e.Name != "APME_PLUGIN_ORGPOLICY_ADDRESS" {
			t.Fatalf("unexpected plugin env %s", e.Name)
		}
	}
	if !found {
		t.Fatal("missing APME_PLUGIN_ORGPOLICY_ADDRESS")
	}
}

func TestEngineNoPluginEnvWhenEmpty(t *testing.T) {
	c := Engine(resolve.Desired{})
	for _, e := range c.Env {
		if len(e.Name) >= 12 && e.Name[:12] == "APME_PLUGIN_" {
			t.Fatalf("unexpected %s", e.Name)
		}
	}
}

func TestPluginContainer(t *testing.T) {
	p := resolve.ResolvedPlugin{
		Name:          "orgpolicy",
		Image:         "registry.example.com/apme-plugin-orgpolicy:1.0",
		Port:          50100,
		ConfigMapName: "orgpolicy-data",
	}
	c := Plugin(resolve.Desired{}, p)
	if c.Name != "plugin-orgpolicy" || c.Image != p.Image {
		t.Fatalf("container=%+v", c)
	}
	if c.ReadinessProbe.TCPSocket.Port.IntVal != 50100 {
		t.Fatalf("probe port=%v", c.ReadinessProbe.TCPSocket.Port)
	}
	listen := false
	for _, e := range c.Env {
		if e.Name == "APME_PLUGIN_LISTEN" && e.Value == "0.0.0.0:50100" {
			listen = true
		}
	}
	if !listen {
		t.Fatalf("missing listen env: %v", c.Env)
	}
	if len(c.VolumeMounts) != 1 || c.VolumeMounts[0].MountPath != "/etc/apme-plugin" || !c.VolumeMounts[0].ReadOnly {
		t.Fatalf("mounts=%v", c.VolumeMounts)
	}
	if c.VolumeMounts[0].Name != PluginConfigVolumeName("orgpolicy") {
		t.Fatalf("vol=%s", c.VolumeMounts[0].Name)
	}
}
