package kubernetes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"
)

var ErrMountedProfile = errors.New("Kubernetes mounted profile rejected")
var documentSeparator = regexp.MustCompile(`(?m)^[\t ]*(?:---|\.\.\.)(?:[\t ]*|[\t ]+#.*)\r?$`)

// MountedKubeconfigOptions is deployment-owned. None of these values come from HTTP.
type MountedKubeconfigOptions struct {
	File      string
	Context   string
	Roots     []string
	Collector APICollectorOptions
}

// NewMountedKubeconfigCollector reads only bounded regular files beneath explicit
// operator mount roots. It never invokes credential plugins or default loading.
func NewMountedKubeconfigCollector(ctx context.Context, options MountedKubeconfigOptions) (*APICollector, error) {
	if ctx == nil || ctx.Err() != nil || len(options.Roots) < 1 || len(options.Roots) > 8 || options.Collector.Client == nil {
		return nil, ErrMountedProfile
	}
	data, err := readMountedFile(ctx, options.Roots, options.File, 1<<20)
	if err != nil {
		return nil, ErrMountedProfile
	}
	defer clear(data)
	content := bytes.TrimSpace(data)
	if bytes.HasPrefix(content, []byte("---")) {
		content = bytes.TrimSpace(content[3:])
	}
	if documentSeparator.Match(content) {
		return nil, ErrMountedProfile
	}
	encoded, err := yaml.YAMLToJSONStrict(content)
	if err != nil {
		return nil, ErrMountedProfile
	}
	defer clear(encoded)
	var configuration clientcmdv1.Config
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&configuration) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, ErrMountedProfile
	}
	if configuration.Kind != "Config" || configuration.APIVersion != "v1" || len(configuration.Extensions) > 0 || len(configuration.Preferences.Extensions) > 0 || configuration.Preferences.Colors || len(configuration.Clusters) < 1 || len(configuration.Clusters) > 64 || len(configuration.AuthInfos) < 1 || len(configuration.AuthInfos) > 64 || len(configuration.Contexts) < 1 || len(configuration.Contexts) > 64 {
		return nil, ErrMountedProfile
	}
	selected := options.Context
	if selected == "" {
		selected = configuration.CurrentContext
	}
	if !safeProfileName(selected) {
		return nil, ErrMountedProfile
	}
	clusters := map[string]clientcmdv1.Cluster{}
	users := map[string]clientcmdv1.AuthInfo{}
	contexts := map[string]clientcmdv1.Context{}
	for _, entry := range configuration.Clusters {
		c := entry.Cluster
		if !safeProfileName(entry.Name) || clusters[entry.Name].Server != "" || canonicalKubernetesOrigin(c.Server) == "" || c.TLSServerName != "" || c.InsecureSkipTLSVerify || len(c.CertificateAuthorityData) > 0 || c.ProxyURL != "" || len(c.Extensions) > 0 || c.CertificateAuthority == "" {
			return nil, ErrMountedProfile
		}
		clusters[entry.Name] = c
	}
	for _, entry := range configuration.AuthInfos {
		u := entry.AuthInfo
		if !safeProfileName(entry.Name) {
			return nil, ErrMountedProfile
		}
		if _, exists := users[entry.Name]; exists {
			return nil, ErrMountedProfile
		}
		if u.Token != "" || u.Username != "" || u.Password != "" || u.AuthProvider != nil || u.Exec != nil || u.Impersonate != "" || u.ImpersonateUID != "" || len(u.ImpersonateGroups) > 0 || len(u.ImpersonateUserExtra) > 0 || len(u.ClientCertificateData) > 0 || len(u.ClientKeyData) > 0 || len(u.Extensions) > 0 {
			return nil, ErrMountedProfile
		}
		token := u.TokenFile != ""
		certificate := u.ClientCertificate != "" && u.ClientKey != ""
		if token == certificate || !certificate && (u.ClientCertificate != "" || u.ClientKey != "") {
			return nil, ErrMountedProfile
		}
		users[entry.Name] = u
	}
	for _, entry := range configuration.Contexts {
		if !safeProfileName(entry.Name) || len(entry.Context.Extensions) > 0 {
			return nil, ErrMountedProfile
		}
		if _, exists := contexts[entry.Name]; exists {
			return nil, ErrMountedProfile
		}
		if _, exists := clusters[entry.Context.Cluster]; !exists {
			return nil, ErrMountedProfile
		}
		if _, exists := users[entry.Context.AuthInfo]; !exists {
			return nil, ErrMountedProfile
		}
		contexts[entry.Name] = entry.Context
	}
	chosen, exists := contexts[selected]
	if !exists {
		return nil, ErrMountedProfile
	}
	cluster, user := clusters[chosen.Cluster], users[chosen.AuthInfo]
	resolve := func(reference string) string {
		if !filepath.IsAbs(reference) {
			reference = filepath.Join(filepath.Dir(options.File), reference)
		}
		return filepath.Clean(reference)
	}
	// Every declared reference must stay in the operator roots, even when its
	// context is not selected. Only the selected credentials are read below.
	for _, cluster := range clusters {
		if !mountedPathWithinRoots(options.Roots, resolve(cluster.CertificateAuthority)) {
			return nil, ErrMountedProfile
		}
	}
	for _, user := range users {
		for _, reference := range []string{user.TokenFile, user.ClientCertificate, user.ClientKey} {
			if reference != "" && !mountedPathWithinRoots(options.Roots, resolve(reference)) {
				return nil, ErrMountedProfile
			}
		}
	}
	ca, err := readMountedFile(ctx, options.Roots, resolve(cluster.CertificateAuthority), 1<<20)
	if err != nil {
		return nil, ErrMountedProfile
	}
	defer clear(ca)
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, ErrMountedProfile
	}
	client := *options.Collector.Client
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		return nil, ErrMountedProfile
	}
	transport = transport.Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	opts := options.Collector
	opts.BaseURL = cluster.Server
	if user.TokenFile != "" {
		path := resolve(user.TokenFile)
		readToken := func(readContext context.Context) (string, error) {
			value, e := readMountedFile(readContext, options.Roots, path, 16<<10)
			if e != nil {
				return "", ErrMountedProfile
			}
			defer clear(value)
			token := strings.TrimSpace(string(value))
			if !validBearerToken(token) {
				return "", ErrMountedProfile
			}
			return token, nil
		}
		opts.BearerToken, err = readToken(ctx)
		if err != nil {
			return nil, ErrMountedProfile
		}
		opts.TokenReader = readToken
	} else {
		certificate, err := readMountedFile(ctx, options.Roots, resolve(user.ClientCertificate), 1<<20)
		if err != nil {
			return nil, ErrMountedProfile
		}
		defer clear(certificate)
		key, err := readMountedFile(ctx, options.Roots, resolve(user.ClientKey), 1<<20)
		if err != nil {
			return nil, ErrMountedProfile
		}
		defer clear(key)
		pair, err := tls.X509KeyPair(certificate, key)
		if err != nil {
			return nil, ErrMountedProfile
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{pair}
		opts.ClientCertificate = true
		opts.BearerToken = ""
		opts.TokenReader = nil
	}
	client.Transport = transport
	opts.Client = &client
	collector, err := NewAPICollector(opts)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, ErrMountedProfile
	}
	digest := sha256.Sum256(append(append([]byte("mounted-kubeconfig/v1\x00"+selected+"\x00"), encoded...), []byte(strings.Join(options.Roots, "\x00"))...))
	collector.operatorPolicyDigest = hex.EncodeToString(digest[:])
	return collector, nil
}

func safeProfileName(value string) bool {
	return value != "" && len(value) <= 256 && !strings.ContainsAny(value, "\x00\r\n\t")
}
func canonicalKubernetesOrigin(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" || len(raw) > 2048 {
		return ""
	}
	host := parsed.Hostname()
	if address, err := netip.ParseAddr(host); err == nil {
		if address.Zone() != "" || address.IsUnspecified() || address.IsMulticast() {
			return ""
		}
	} else if len(validation.IsDNS1123Subdomain(strings.ToLower(host))) != 0 {
		return ""
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil || number == 0 || strconv.FormatUint(number, 10) != port {
			return ""
		}
	} else if strings.HasSuffix(parsed.Host, ":") {
		return ""
	}
	return parsed.String()
}

func mountedPathWithinRoots(roots []string, path string) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(filepath.ToSlash(path), "//") {
		return false
	}
	for _, directory := range roots {
		relative, err := filepath.Rel(directory, path)
		if err != nil || relative == "." || !filepath.IsLocal(relative) {
			continue
		}
		for current := path; ; current = filepath.Dir(current) {
			info, err := os.Lstat(current)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return false
			}
			if current == path {
				if !info.Mode().IsRegular() {
					return false
				}
			} else if !info.IsDir() {
				return false
			}
			if filepath.Dir(current) == current {
				break
			}
		}
		return true
	}
	return false
}

func readMountedFile(ctx context.Context, roots []string, path string, maximum int64) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(filepath.ToSlash(path), "//") {
		return nil, ErrMountedProfile
	}
	for _, directory := range roots {
		if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || filepath.Dir(directory) == directory || strings.HasPrefix(filepath.ToSlash(directory), "//") {
			return nil, ErrMountedProfile
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil || relative == "." || !filepath.IsLocal(relative) {
			continue
		}
		// Check every directory above the mount as well as every component below it.
		for current := directory; ; current = filepath.Dir(current) {
			info, err := os.Lstat(current)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return nil, ErrMountedProfile
			}
			if filepath.Dir(current) == current {
				break
			}
		}
		root, err := os.OpenRoot(directory)
		if err != nil {
			return nil, ErrMountedProfile
		}
		value, readErr := readMountedRelative(ctx, root, relative, maximum)
		closeErr := root.Close()
		if readErr != nil || closeErr != nil {
			return nil, ErrMountedProfile
		}
		return value, nil
	}
	return nil, ErrMountedProfile
}
func readMountedRelative(ctx context.Context, root *os.Root, relative string, maximum int64) ([]byte, error) {
	parts := strings.Split(relative, string(os.PathSeparator))
	current := ""
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := root.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || index < len(parts)-1 && !info.IsDir() {
			return nil, ErrMountedProfile
		}
	}
	before, err := root.Lstat(relative)
	if err != nil || !before.Mode().IsRegular() || before.Size() > maximum {
		return nil, ErrMountedProfile
	}
	file, err := root.Open(relative)
	if err != nil {
		return nil, ErrMountedProfile
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, ErrMountedProfile
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	after, statErr := root.Lstat(relative)
	if err != nil || statErr != nil || ctx.Err() != nil || len(data) == 0 || int64(len(data)) > maximum || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, after) {
		clear(data)
		return nil, ErrMountedProfile
	}
	return data, nil
}

func (collector *APICollector) OperatorPolicyDigest() string { return collector.operatorPolicyDigest }
func (provider *Provider) OperatorPolicyDigest() string {
	if source, ok := provider.collector.(interface{ OperatorPolicyDigest() string }); ok {
		return source.OperatorPolicyDigest()
	}
	return ""
}
