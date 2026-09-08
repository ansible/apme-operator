package containers

import (
	"testing"

	"github.com/ansible/apme-operator/internal/resolve"
)

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
