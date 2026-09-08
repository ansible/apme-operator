package containers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ansible/apme-operator/internal/resolve"
)

func tcpProbe(port int32, delay, period int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(port)},
		},
		InitialDelaySeconds: delay,
		PeriodSeconds:       period,
	}
}

func httpProbe(path, port string, delay, period int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromString(port)},
		},
		InitialDelaySeconds: delay,
		PeriodSeconds:       period,
	}
}

func env(kv ...string) []corev1.EnvVar {
	out := make([]corev1.EnvVar, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, corev1.EnvVar{Name: kv[i], Value: kv[i+1]})
	}
	return out
}

func secretRef(name, secret, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secret},
				Key:                  key,
			},
		},
	}
}

func withProxy(base []corev1.EnvVar, d resolve.Desired) []corev1.EnvVar {
	return append(base, d.ProxyEnv...)
}

func resources(d resolve.Desired) corev1.ResourceRequirements {
	return d.Resources
}

func pull(d resolve.Desired) corev1.PullPolicy {
	return d.PullPolicy
}

func emptySC() *corev1.SecurityContext {
	return &corev1.SecurityContext{}
}

// Engine is the orchestrator container.
func Engine(d resolve.Desired) corev1.Container {
	e := env(
		"UV_CACHE_DIR", "/tmp/uv-cache",
		"APME_ENGINE_LISTEN", "0.0.0.0:50051",
		"APME_ENGINE_MAX_RPCS", "16",
		"NATIVE_GRPC_ADDRESS", "127.0.0.1:50055",
		"OPA_GRPC_ADDRESS", "127.0.0.1:50054",
		"ANSIBLE_GRPC_ADDRESS", "127.0.0.1:50053",
		"APME_GALAXY_PROXY_URL", "http://127.0.0.1:8765",
		"APME_REPORTING_ENDPOINT", "127.0.0.1:50060",
	)
	if d.Gitleaks {
		e = append(e, corev1.EnvVar{Name: "GITLEAKS_GRPC_ADDRESS", Value: "127.0.0.1:50056"})
	}
	if d.CollectionHealth {
		e = append(e, corev1.EnvVar{Name: "COLLECTION_HEALTH_GRPC_ADDRESS", Value: "127.0.0.1:50058"})
	}
	if d.DepAudit {
		e = append(e, corev1.EnvVar{Name: "DEP_AUDIT_GRPC_ADDRESS", Value: "127.0.0.1:50059"})
	}
	mounts := []corev1.VolumeMount{{Name: "sessions", MountPath: "/sessions"}}
	if d.Abbenay {
		e = append(e,
			corev1.EnvVar{Name: "APME_ABBENAY_ADDR", Value: "unix:///tmp/abbenay-run/abbenay/daemon.sock"},
			secretRef("APME_ABBENAY_TOKEN", d.AbbenayTokenName, d.AbbenayTokenKey),
		)
		mounts = append(mounts, corev1.VolumeMount{Name: "abbenay-run", MountPath: "/tmp/abbenay-run"})
	}
	return corev1.Container{
		Name:            "engine",
		Image:           d.Image("engine"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env:             withProxy(e, d),
		Ports:           []corev1.ContainerPort{{Name: "grpc", ContainerPort: 50051}},
		ReadinessProbe:  tcpProbe(50051, 5, 10),
		LivenessProbe:   tcpProbe(50051, 15, 30),
		VolumeMounts:    mounts,
		Resources:       resources(d),
	}
}

// Native validator.
func Native(d resolve.Desired) corev1.Container {
	return corev1.Container{
		Name:            "native",
		Image:           d.Image("native"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env:             withProxy(env("APME_NATIVE_VALIDATOR_LISTEN", "0.0.0.0:50055"), d),
		ReadinessProbe:  tcpProbe(50055, 5, 10),
		LivenessProbe:   tcpProbe(50055, 15, 30),
		Resources:       resources(d),
	}
}

// OPA validator.
func OPA(d resolve.Desired) corev1.Container {
	return corev1.Container{
		Name:            "opa",
		Image:           d.Image("opa"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env:             withProxy(env("APME_OPA_VALIDATOR_LISTEN", "0.0.0.0:50054"), d),
		ReadinessProbe:  tcpProbe(50054, 5, 10),
		LivenessProbe:   tcpProbe(50054, 15, 30),
		Resources:       resources(d),
	}
}

// Ansible validator. Sessions mount is read-only.
func Ansible(d resolve.Desired) corev1.Container {
	ro := true
	return corev1.Container{
		Name:            "ansible",
		Image:           d.Image("ansible"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env: withProxy(env(
			"APME_ANSIBLE_VALIDATOR_LISTEN", "0.0.0.0:50053",
			"APME_GALAXY_PROXY_URL", "http://127.0.0.1:8765",
		), d),
		ReadinessProbe: tcpProbe(50053, 5, 10),
		LivenessProbe:  tcpProbe(50053, 15, 30),
		VolumeMounts:   []corev1.VolumeMount{{Name: "sessions", MountPath: "/sessions", ReadOnly: ro}},
		Resources:      resources(d),
	}
}

// Gitleaks validator.
func Gitleaks(d resolve.Desired) corev1.Container {
	return corev1.Container{
		Name:            "gitleaks",
		Image:           d.Image("gitleaks"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env:             withProxy(env("APME_GITLEAKS_VALIDATOR_LISTEN", "0.0.0.0:50056"), d),
		ReadinessProbe:  tcpProbe(50056, 5, 10),
		LivenessProbe:   tcpProbe(50056, 15, 30),
		Resources:       resources(d),
	}
}

// CollectionHealth validator.
func CollectionHealth(d resolve.Desired) corev1.Container {
	ro := true
	return corev1.Container{
		Name:            "collection-health",
		Image:           d.Image("collection-health"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env:             withProxy(env("APME_COLLECTION_HEALTH_VALIDATOR_LISTEN", "0.0.0.0:50058"), d),
		ReadinessProbe:  tcpProbe(50058, 5, 10),
		LivenessProbe:   tcpProbe(50058, 15, 30),
		VolumeMounts:    []corev1.VolumeMount{{Name: "sessions", MountPath: "/sessions", ReadOnly: ro}},
		Resources:       resources(d),
	}
}

// DepAudit validator.
func DepAudit(d resolve.Desired) corev1.Container {
	ro := true
	return corev1.Container{
		Name:            "dep-audit",
		Image:           d.Image("dep-audit"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env:             withProxy(env("APME_DEP_AUDIT_VALIDATOR_LISTEN", "0.0.0.0:50059"), d),
		ReadinessProbe:  tcpProbe(50059, 5, 10),
		LivenessProbe:   tcpProbe(50059, 15, 30),
		VolumeMounts:    []corev1.VolumeMount{{Name: "sessions", MountPath: "/sessions", ReadOnly: ro}},
		Resources:       resources(d),
	}
}

// GalaxyProxy serves PEP 503 wheels.
func GalaxyProxy(d resolve.Desired) corev1.Container {
	return corev1.Container{
		Name:            "galaxy-proxy",
		Image:           d.Image("galaxy-proxy"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env:             withProxy(nil, d),
		ReadinessProbe:  tcpProbe(8765, 5, 10),
		LivenessProbe:   tcpProbe(8765, 10, 30),
		VolumeMounts:    []corev1.VolumeMount{{Name: "proxy-cache", MountPath: "/cache"}},
		Resources:       resources(d),
	}
}

// Gateway is REST + reporting gRPC. Always Postgres via APME_DATABASE_URL.
func Gateway(d resolve.Desired) corev1.Container {
	e := env(
		"APME_ENGINE_ADDRESS", "127.0.0.1:50051",
		"APME_GATEWAY_GRPC_LISTEN", "0.0.0.0:50060",
		"APME_GATEWAY_HTTP_HOST", "0.0.0.0",
		"APME_GATEWAY_HTTP_PORT", "8080",
	)
	e = append(e, secretRef("APME_DATABASE_URL", d.DatabaseSecretName, d.DatabaseSecretKey))
	if d.CollectionHealth {
		e = append(e, corev1.EnvVar{Name: "COLLECTION_HEALTH_GRPC_ADDRESS", Value: "127.0.0.1:50058"})
	}
	if d.DepAudit {
		e = append(e, corev1.EnvVar{Name: "DEP_AUDIT_GRPC_ADDRESS", Value: "127.0.0.1:50059"})
	}
	var mounts []corev1.VolumeMount
	if d.GeneratePostgres {
		// Managed Postgres: trust the operator/user CA while keeping system CAs via init-built bundle.
		// SSL_CERT_FILE covers outbound HTTPS; PGSSLROOTCERT is what asyncpg/libpq use for
		// sslmode=verify-full (SSL_CERT_FILE alone is ignored and asyncpg falls back to
		// ~/.postgresql/root.crt, which does not exist in the Gateway image).
		caBundle := "/etc/apme/db-ca/ca-bundle.crt"
		e = append(e,
			corev1.EnvVar{Name: "SSL_CERT_FILE", Value: caBundle},
			corev1.EnvVar{Name: "PGSSLROOTCERT", Value: caBundle},
		)
		mounts = append(mounts, corev1.VolumeMount{Name: "db-ca-bundle", MountPath: "/etc/apme/db-ca", ReadOnly: true})
	}
	if d.Abbenay {
		e = append(e,
			corev1.EnvVar{Name: "APME_ABBENAY_HTTP_URL", Value: "http://127.0.0.1:8787"},
			secretRef("APME_ABBENAY_HTTP_TOKEN", d.AbbenayTokenName, d.AbbenayTokenKey),
			corev1.EnvVar{Name: "APME_ABBENAY_ADDR", Value: "unix:///tmp/abbenay-run/abbenay/daemon.sock"},
		)
		mounts = append(mounts, corev1.VolumeMount{Name: "abbenay-run", MountPath: "/tmp/abbenay-run"})
	}
	return corev1.Container{
		Name:            "gateway",
		Image:           d.Image("gateway"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env:             withProxy(e, d),
		Ports: []corev1.ContainerPort{
			{Name: "gateway-grpc", ContainerPort: 50060},
			{Name: "http", ContainerPort: 8080},
		},
		LivenessProbe:  httpProbe("/api/v1/dashboard/summary", "http", 10, 30),
		ReadinessProbe: httpProbe("/api/v1/dashboard/summary", "http", 5, 10),
		VolumeMounts:   mounts,
		Resources:      resources(d),
	}
}

// UI is the nginx SPA.
func UI(d resolve.Desired) corev1.Container {
	return corev1.Container{
		Name:            "ui",
		Image:           d.Image("ui"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Env:             withProxy(env("APME_API_BACKEND", "http://127.0.0.1:8080"), d),
		Ports:           []corev1.ContainerPort{{Name: "ui-http", ContainerPort: 8081}},
		LivenessProbe:   httpProbe("/", "ui-http", 5, 30),
		ReadinessProbe:  httpProbe("/", "ui-http", 3, 10),
		VolumeMounts: []corev1.VolumeMount{
			{Name: "nginx-cache", MountPath: "/var/cache/nginx"},
			{Name: "nginx-run", MountPath: "/var/run"},
			{Name: "nginx-conf", MountPath: "/etc/nginx/conf.d"},
		},
		Resources: resources(d),
	}
}

// Abbenay is the optional AI sidecar (loopback + unix socket).
// Probes use `/opt/abbenay/abbenay status` (checks the daemon Unix socket).
// Do not use `node -e`: the published Abbenay image is a single binary with no
// Node.js runtime, so those probes fail and keep the whole Simple pod NotReady.
func Abbenay(d resolve.Desired) corev1.Container {
	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{
			Command: []string{"/opt/abbenay/abbenay", "status"},
		}},
	}
	ready := *probe
	ready.InitialDelaySeconds, ready.PeriodSeconds = 5, 10
	live := *probe
	live.InitialDelaySeconds, live.PeriodSeconds = 15, 30
	return corev1.Container{
		Name:            "abbenay",
		Image:           d.AbbenayImage,
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Args:            []string{"web", "--host", "127.0.0.1", "--port", "8787", "--grpc-host", "127.0.0.1", "--grpc-port", "50057"},
		Env: withProxy([]corev1.EnvVar{
			secretRef("APME_ABBENAY_TOKEN", d.AbbenayTokenName, d.AbbenayTokenKey),
			secretRef("ABBENAY_API_TOKEN", d.AbbenayTokenName, d.AbbenayTokenKey),
			{Name: "XDG_RUNTIME_DIR", Value: "/tmp/abbenay-run"},
			{Name: "XDG_CONFIG_HOME", Value: "/etc/abbenay-config"},
		}, d),
		ReadinessProbe: &ready,
		LivenessProbe:  &live,
		VolumeMounts: []corev1.VolumeMount{
			{Name: "abbenay-config", MountPath: "/etc/abbenay-config"},
			{Name: "abbenay-run", MountPath: "/tmp/abbenay-run"},
		},
		Resources: resources(d),
	}
}

// InitNginx copies stock nginx conf into a writable emptyDir.
// InitDBCABundle merges the system CA bundle with the managed Postgres CA so
// Gateway can use sslmode=verify-full without breaking outbound HTTPS trust.
func InitDBCABundle(d resolve.Desired) corev1.Container {
	// Use cat (not cp): system CA bundles are mode 0444, and cp preserves that,
	// which makes the subsequent append fail and leaves a read-only file that
	// also breaks init retries on the same emptyDir.
	script := `set -e
BUNDLE=/work/ca-bundle.crt
rm -f "$BUNDLE"
if [ -f /etc/pki/tls/certs/ca-bundle.crt ]; then
  cat /etc/pki/tls/certs/ca-bundle.crt > "$BUNDLE"
elif [ -f /etc/ssl/certs/ca-certificates.crt ]; then
  cat /etc/ssl/certs/ca-certificates.crt > "$BUNDLE"
else
  : > "$BUNDLE"
fi
cat /db-ca/ca.crt >> "$BUNDLE"
`
	return corev1.Container{
		Name:            "init-db-ca",
		Image:           d.Image("gateway"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Command:         []string{"/bin/sh", "-c", script},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "db-ca", MountPath: "/db-ca", ReadOnly: true},
			{Name: "db-ca-bundle", MountPath: "/work"},
		},
	}
}

func InitNginx(d resolve.Desired) corev1.Container {
	return corev1.Container{
		Name:            "init-nginx-conf",
		Image:           d.Image("ui"),
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Command:         []string{"sh", "-c", "cp -a -r --no-preserve=mode,ownership,timestamps /etc/nginx/conf.d/. /mnt/conf.d/"},
		VolumeMounts:    []corev1.VolumeMount{{Name: "nginx-conf", MountPath: "/mnt/conf.d"}},
	}
}

// InitAbbenayConfig copies seed config.yaml once onto the config volume.
func InitAbbenayConfig(d resolve.Desired) corev1.Container {
	return corev1.Container{
		Name:            "init-abbenay-config",
		Image:           d.AbbenayImage,
		ImagePullPolicy: pull(d),
		SecurityContext: emptySC(),
		Command: []string{"sh", "-c",
			"mkdir -p /mnt/config/abbenay; if [ ! -f /mnt/config/abbenay/config.yaml ]; then cp /seed/config.yaml /mnt/config/abbenay/config.yaml; fi"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "abbenay-config-seed", MountPath: "/seed", ReadOnly: true},
			{Name: "abbenay-config", MountPath: "/mnt/config"},
		},
	}
}
