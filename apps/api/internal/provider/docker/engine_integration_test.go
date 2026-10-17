package docker

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const proofProxyImage = "haproxy:3.2-alpine@sha256:254e6370f01cd654e0744edf607c67f33777c1b6a9dc0d4b28bcba7171460122"

// This opt-in proof creates only disposable fixtures on its dedicated CI Engine.
// Product observation uses the restricted proxy; setup uses fixed Docker commands.
func TestRestrictedProxyOnRealDockerEngine(t *testing.T) {
	if os.Getenv("STACKPLORE_TEST_DOCKER_ENGINE") != "1" {
		t.Skip("real disposable Docker Engine proof not configured")
	}
	if runtime.GOOS != "linux" {
		t.Fatal("real Engine proof requires disposable Linux")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	proofName := fmt.Sprintf("stackplore-proof-%d", time.Now().UnixNano())
	label := "stackplore.proof=" + proofName
	t.Log("Disposable proof label: " + label)
	networkName, volumeName, imageName := proofName+"-network", proofName+"-volume", proofName+":fixture"
	var ownedContainers []string
	t.Cleanup(func() {
		cleanup, cancelCleanup := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancelCleanup()
		for _, identifier := range ownedContainers {
			_, _ = proofCommand(cleanup, "docker", "rm", "-f", identifier)
		}
		_, _ = proofCommand(cleanup, "docker", "network", "rm", networkName)
		_, _ = proofCommand(cleanup, "docker", "volume", "rm", volumeName)
		_, _ = proofCommand(cleanup, "docker", "image", "rm", imageName)
	})
	docker := func(args ...string) string {
		t.Helper()
		output, err := proofCommand(ctx, "docker", args...)
		if err != nil {
			t.Fatalf("disposable Docker fixture step failed: %s", args[0])
		}
		return strings.TrimSpace(output)
	}
	// The fixture image is a fixed Go program built FROM scratch; no registry
	// fixture image, shell access, credentials or customer files are involved.
	buildDirectory := t.TempDir()
	buildContext, cancelBuild := context.WithTimeout(ctx, 2*time.Minute)
	command := exec.CommandContext(buildContext, "go", "build", "-trimpath", "-o", filepath.Join(buildDirectory, "fixture"), "./testdata/engine-fixture")
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	if command.Run() != nil {
		cancelBuild()
		t.Fatal("fixture binary build failed")
	}
	cancelBuild()
	dockerfile := `FROM scratch
COPY fixture /fixture
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=1s --timeout=1s --start-period=1s --retries=5 CMD ["/fixture","health"]
ENTRYPOINT ["/fixture"]
`
	if os.WriteFile(filepath.Join(buildDirectory, "Dockerfile"), []byte(dockerfile), 0600) != nil {
		t.Fatal("fixture recipe write failed")
	}
	docker("build", "--quiet", "--tag", imageName, buildDirectory)
	docker("network", "create", "--label", label, networkName)
	docker("volume", "create", "--label", label, volumeName)
	start := time.Now().UTC().Truncate(time.Microsecond)
	for _, service := range []string{"api", "worker"} {
		args := []string{"run", "-d", "--name", proofName + "-" + service, "--label", label, "--label", "com.docker.compose.project=" + proofName, "--label", "com.docker.compose.service=" + service,
			"--network", networkName, "--memory", "64m", "--cpus", "0.25", "--publish", "127.0.0.1::8080"}
		if service == "api" {
			args = append(args, "--label", "stackplore.io/depends-on=worker", "--mount", "type=volume,source="+volumeName+",target=/data,readonly")
		} else {
			args = append(args, "--tty")
		}
		identifier := docker(append(args, imageName)...)
		if len(identifier) != 64 || !safeDockerID(identifier) {
			t.Fatal("fixture identifier invalid")
		}
		ownedContainers = append(ownedContainers, identifier)
	}
	apiID, ttyID := ownedContainers[0], ownedContainers[1]
	certificateDirectory, tlsConfig := proofCertificates(t)
	proxyConfig, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "deploy", "docker-proxy", "haproxy.cfg"))
	if err != nil {
		t.Fatal("proxy configuration path failed")
	}
	if _, err = os.Stat(proxyConfig); err != nil {
		t.Fatal("reviewed proxy configuration missing")
	}
	docker("pull", proofProxyImage)
	// Keep fixture keys private to their owner. Root with all capabilities dropped
	// cannot read another user's 0600 files. The proof runs as that same owner and
	// retains only its existing groups so it can reach the mounted Engine socket.
	proxyUser := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	groups, groupErr := os.Getgroups()
	if groupErr != nil || len(groups) > 64 {
		t.Fatal("fixture proxy groups unavailable")
	}
	docker("run", "--rm", "--user", proxyUser, "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--mount", "type=bind,source="+certificateDirectory+",target=/run/stackplore-docker-proxy,readonly",
		"--mount", "type=bind,source="+proxyConfig+",target=/usr/local/etc/haproxy/haproxy.cfg,readonly", proofProxyImage, "haproxy", "-c", "-f", "/usr/local/etc/haproxy/haproxy.cfg")
	proxyArgs := []string{"run", "-d", "--name", proofName + "-proxy", "--label", label, "--user", proxyUser, "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--memory", "64m", "--pids-limit", "32"}
	for _, group := range groups {
		proxyArgs = append(proxyArgs, "--group-add", strconv.Itoa(group))
	}
	proxyArgs = append(proxyArgs,
		"--publish", "127.0.0.1::8443", "--mount", "type=bind,source="+certificateDirectory+",target=/run/stackplore-docker-proxy,readonly",
		"--mount", "type=bind,source="+proxyConfig+",target=/usr/local/etc/haproxy/haproxy.cfg,readonly", "--mount", "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock,readonly",
		proofProxyImage, "haproxy", "-W", "-db", "-f", "/usr/local/etc/haproxy/haproxy.cfg")
	proxyID := docker(proxyArgs...)
	ownedContainers = append(ownedContainers, proxyID)
	proxyAddress := docker("port", proxyID, "8443/tcp")
	_, proxyPort, err := net.SplitHostPort(proxyAddress)
	if err != nil {
		t.Fatal("proxy port missing")
	}
	origin := "https://localhost:" + proxyPort
	transport := &http.Transport{Proxy: nil, TLSClientConfig: tlsConfig, DisableCompression: true, MaxIdleConns: 8, MaxConnsPerHost: 8, ResponseHeaderTimeout: 5 * time.Second}
	httpClient := &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrUnavailable }}
	t.Cleanup(transport.CloseIdleConnections)
	proofEventually(t, ctx, func() bool {
		request, _ := http.NewRequestWithContext(ctx, http.MethodHead, origin+"/_ping", nil)
		response, err := httpClient.Do(request)
		if err != nil {
			return false
		}
		_ = response.Body.Close()
		return response.StatusCode == http.StatusOK
	})
	collector, err := NewEngineCollector(EngineCollectorOptions{Client: httpClient, BaseURL: origin, MaximumItems: 10000, MaximumResponse: 8 << 20, InspectConcurrency: 4})
	if err != nil {
		t.Fatal("SDK collector creation failed")
	}
	t.Cleanup(func() { _ = collector.Close() })
	proofEventually(t, ctx, func() bool {
		snapshot, err := collector.Snapshot(ctx)
		if err != nil {
			return false
		}
		for _, item := range snapshot.Containers {
			if item.ID == apiID {
				return item.Health == "healthy"
			}
		}
		return false
	})
	snapshot, err := collector.Snapshot(ctx)
	if err != nil || snapshot.EngineID == "" || snapshot.EngineVersion == "" {
		t.Fatal("real Engine identity/version missing")
	}
	t.Logf("Engine version %s; negotiated API %s", snapshot.EngineVersion, collector.sdk.ClientVersion())
	var apiContainer Container
	for _, item := range snapshot.Containers {
		if item.ID == apiID {
			apiContainer = item
		}
	}
	if apiContainer.ComposeProject != proofName || apiContainer.ComposeService != "api" || apiContainer.Metadata == nil || apiContainer.Metadata.Validate() != nil ||
		len(apiContainer.VolumeNames) != 1 || len(apiContainer.NetworkIDs) != 1 || len(apiContainer.Metadata.Ports) == 0 || apiContainer.Metadata.CPULimitCores == nil || apiContainer.Metadata.MemoryLimitBytes == nil {
		t.Fatal("real safe discovery metadata incomplete")
	}
	if len(apiContainer.Metadata.Mounts) != 1 || apiContainer.Metadata.Mounts[0].Target != "/data" || !apiContainer.Metadata.Mounts[0].ReadOnly {
		t.Fatal("safe mount metadata wrong")
	}
	provider, _ := New("source_docker_proof", "Disposable Engine", collector)
	capture, err := provider.CaptureInventory(ctx)
	if err != nil || capture.ValidateFor(capture.Source) != nil {
		t.Fatal("real provider contract failed")
	}
	dependency := false
	for _, edge := range capture.Relationships {
		if edge.Kind == domain.RelationshipDependsOn {
			dependency = true
		}
	}
	if !dependency {
		t.Fatal("declared Compose dependency missing")
	}
	apiAddress := docker("port", apiID, "8080/tcp")
	trafficClient := &http.Client{Timeout: 2 * time.Second}
	for range 4 {
		response, err := trafficClient.Get("http://" + apiAddress + "/health")
		if err != nil {
			t.Fatal("fixture network probe failed")
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
	stats, err := collector.CurrentContainerStats(ctx, []string{apiID})
	if err != nil || len(stats) != 1 || stats[0].CPUUnavailable || stats[0].CPUUsageCores <= 0 || stats[0].MemoryUsageBytes == 0 || stats[0].NetworkReadBytes == 0 {
		t.Fatal("real two-sample CPU/memory/network proof failed")
	}
	end := time.Now().UTC().Truncate(time.Microsecond)
	for _, identifier := range []string{apiID, ttyID} {
		lines, err := collector.ReadContainerLogs(ctx, ContainerLogRequest{ContainerIDs: []string{identifier}, Start: start, End: end, LineLimit: 20, ByteLimit: 64 << 10})
		if err != nil || len(lines) == 0 {
			t.Fatal("real bounded historic logs failed")
		}
		for _, line := range lines {
			if line.Timestamp.Before(start) || !line.Timestamp.Before(end) || !strings.Contains(line.Message, "fixture ") {
				t.Fatal("real log range/content failed")
			}
			if identifier == ttyID && line.Stream != domain.LogStreamUnknown {
				t.Fatal("TTY stream fabricated")
			}
		}
	}
	events, err := collector.EngineEvents(ctx, start, end)
	if err != nil || len(events.Events) == 0 {
		t.Fatal("real historical event read failed")
	}
	foundEvent := false
	for _, event := range events.Events {
		if event.ActorID == apiID {
			foundEvent = true
		}
	}
	if !foundEvent {
		t.Fatal("fixture event missing")
	}
	// Exercise the proxy directly so adapter guards cannot conceal a broad proxy.
	for _, test := range []struct{ method, path string }{
		{"POST", "/containers/" + apiID + "/restart?t=1"}, {"DELETE", "/containers/" + apiID},
		{"GET", "/containers/" + apiID + "/archive?path=/etc/passwd"}, {"GET", "/containers/" + apiID + "/export"},
		{"GET", "/containers/" + apiID + "/attach/ws"}, {"GET", "/exec/" + apiID + "/json"}, {"GET", "/secrets"}, {"GET", "/configs"},
		{"GET", "/containers/" + apiID + "/stats?stream=true"}, {"GET", "/containers/" + apiID + "/logs?follow=1"},
		{"GET", "/containers/json?all=1&all=0"}, {"GET", "/containers/%2e%2e/info"},
	} {
		request, _ := http.NewRequestWithContext(ctx, test.method, origin+test.path, nil)
		response, err := httpClient.Do(request)
		if err != nil {
			t.Fatal("proxy denial probe failed")
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("proxy allowed prohibited surface: %s", test.method)
		}
	}
	after, err := collector.Snapshot(ctx)
	if err != nil {
		t.Fatal("post-denial snapshot failed")
	}
	for _, item := range after.Containers {
		if item.ID == apiID && item.Version != apiContainer.Version {
			t.Fatal("denied write changed fixture")
		}
	}
	withoutCertificate := tlsConfig.Clone()
	withoutCertificate.Certificates = nil
	unauthenticated := &http.Client{Transport: &http.Transport{TLSClientConfig: withoutCertificate}, Timeout: 3 * time.Second}
	defer unauthenticated.CloseIdleConnections()
	if response, err := unauthenticated.Get(origin + "/info"); err == nil {
		if response != nil {
			_ = response.Body.Close()
		}
		t.Fatal("proxy accepted missing client certificate")
	}
	// A container target may legitimately equal a host path (the proxy itself
	// mounts its socket at the same target). Check the owned fixture's actual
	// host volume source against its approved metadata, not unrelated targets.
	hostVolumePath := docker("volume", "inspect", "--format", "{{.Mountpoint}}", volumeName)
	encoded, err := json.Marshal(apiContainer.Metadata)
	if hostVolumePath == "" || err != nil || bytes.Contains(encoded, []byte(hostVolumePath)) ||
		bytes.Contains(encoded, []byte(`"host_ip"`)) || bytes.Contains(encoded, []byte(`"source"`)) || bytes.Contains(encoded, []byte(`"environment"`)) {
		t.Fatal("host source or unreviewed field crossed normalized metadata")
	}
	t.Log("REAL_DOCKER_RESTRICTED_MTLS_SDK_INVENTORY_STATS_LOGS_EVENTS_AND_DENIALS_PASS")
}

func proofCommand(ctx context.Context, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	var output boundedProofOutput
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return output.String(), err
}

type boundedProofOutput struct{ bytes.Buffer }

func (output *boundedProofOutput) Write(data []byte) (int, error) {
	size := len(data)
	if output.Len() < 8192 {
		_, _ = output.Buffer.Write(data[:min(size, 8192-output.Len())])
	}
	return size, nil
}

func proofEventually(t *testing.T, ctx context.Context, check func() bool) {
	t.Helper()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("fixture proof deadline exceeded")
		case <-deadline.C:
			t.Fatal("fixture did not become ready")
		case <-ticker.C:
		}
	}
}

func proofCertificates(t *testing.T) (string, *tls.Config) {
	t.Helper()
	directory := t.TempDir()
	now := time.Now()
	newKey := func() *ecdsa.PrivateKey {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal("fixture key generation failed")
		}
		return key
	}
	caKey := newKey()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Disposable Engine proof CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal("fixture CA generation failed")
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	issue := func(serial int64, server bool) ([]byte, []byte) {
		key := newKey()
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "Disposable Engine proof"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
		if server {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			template.DNSNames = []string{"localhost", "docker-proxy"}
		} else {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}
		certificate, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal("fixture certificate failed")
		}
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal("fixture key encoding failed")
		}
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	}
	serverPEM, serverKey := issue(2, true)
	clientPEM, clientKey := issue(3, false)
	if os.WriteFile(filepath.Join(directory, "ca.pem"), caPEM, 0600) != nil || os.WriteFile(filepath.Join(directory, "server.pem"), append(serverPEM, serverKey...), 0600) != nil {
		t.Fatal("fixture TLS files failed")
	}
	certificate, err := tls.X509KeyPair(clientPEM, clientKey)
	if err != nil {
		t.Fatal("fixture client identity failed")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("fixture CA pool failed")
	}
	return directory, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}}
}
