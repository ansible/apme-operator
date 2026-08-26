package controller

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestApplyPatch(t *testing.T) {
	t.Parallel()

	p := applyPatch{}
	if p.Type() != types.ApplyPatchType {
		t.Fatalf("Type() = %q, want %q", p.Type(), types.ApplyPatchType)
	}

	sa := &corev1.ServiceAccount{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"},
	}
	data, err := p.Data(sa)
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Data() is not JSON: %v", err)
	}
	if decoded["kind"] != "ServiceAccount" {
		t.Fatalf("kind = %v, want ServiceAccount", decoded["kind"])
	}
}
