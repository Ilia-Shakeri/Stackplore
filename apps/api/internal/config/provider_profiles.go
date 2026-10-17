package config

import (
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	dockerAccessProfileKey          = prefix + "DOCKER_ACCESS_PROFILE"
	dockerProxyOriginKey            = prefix + "DOCKER_PROXY_ORIGIN"
	dockerProxyCAFileKey            = prefix + "DOCKER_PROXY_CA_FILE"
	dockerProxyCertFileKey          = prefix + "DOCKER_PROXY_CERT_FILE"
	dockerProxyKeyFileKey           = prefix + "DOCKER_PROXY_KEY_FILE"
	kubernetesNamespacesKey         = prefix + "KUBERNETES_NAMESPACES"
	kubernetesClusterObservationKey = prefix + "KUBERNETES_CLUSTER_OBSERVATION"
	demoDeploymentKey               = prefix + "DEMO_DEPLOYMENT"
	guestDemoKey                    = prefix + "GUEST_DEMO"
	demoDataDirectoryKey            = prefix + "DEMO_DATA_DIRECTORY"
)

type DockerAccessProfile string

const (
	DockerProfileSocketDevelopment DockerAccessProfile = "docker_socket_dev"
	DockerProfileRestrictedProxy   DockerAccessProfile = "docker_restricted_proxy"
)

type ProviderProfiles struct {
	DockerAccessProfile          DockerAccessProfile
	DockerProxyOrigin            string
	DockerProxyCAFile            string
	DockerProxyCertFile          string
	DockerProxyKeyFile           string
	KubernetesNamespaces         []string
	KubernetesClusterObservation bool
	DemoDeployment               bool
	GuestDemo                    bool
	DemoDataDirectory            string
	providedProfileInputs        []string
}

var providerProfileKeys = []string{dockerAccessProfileKey, dockerProxyOriginKey, dockerProxyCAFileKey, dockerProxyCertFileKey, dockerProxyKeyFileKey, kubernetesNamespacesKey, kubernetesClusterObservationKey, demoDeploymentKey, guestDemoKey, demoDataDirectoryKey}
var namespaceLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

func loadProviderProfiles(getter Getter, result *Config) error {
	result.DockerAccessProfile = DockerProfileSocketDevelopment
	if value, present, err := readValue(getter, dockerAccessProfileKey); err != nil {
		return err
	} else if present {
		result.DockerAccessProfile = DockerAccessProfile(value)
	}
	for _, field := range []struct {
		key    string
		target *string
	}{
		{dockerProxyOriginKey, &result.DockerProxyOrigin}, {dockerProxyCAFileKey, &result.DockerProxyCAFile}, {dockerProxyCertFileKey, &result.DockerProxyCertFile}, {dockerProxyKeyFileKey, &result.DockerProxyKeyFile}, {demoDataDirectoryKey, &result.DemoDataDirectory},
	} {
		value, present, err := readValue(getter, field.key)
		if err != nil {
			return err
		}
		if present {
			*field.target = value
		}
	}
	for _, field := range []struct {
		key    string
		target *bool
	}{
		{kubernetesClusterObservationKey, &result.KubernetesClusterObservation}, {demoDeploymentKey, &result.DemoDeployment}, {guestDemoKey, &result.GuestDemo},
	} {
		value, err := loadBool(getter, field.key, false)
		if err != nil {
			return err
		}
		*field.target = value
	}
	if value, present, err := readValue(getter, kubernetesNamespacesKey); err != nil {
		return err
	} else if present {
		parts := strings.Split(value, ",")
		if len(parts) < 1 || len(parts) > 64 {
			return invalid(kubernetesNamespacesKey, "must contain one to 64 namespace names")
		}
		seen := map[string]struct{}{}
		for _, item := range parts {
			if !namespaceLabel.MatchString(item) {
				return invalid(kubernetesNamespacesKey, "is invalid")
			}
			seen[item] = struct{}{}
		}
		for item := range seen {
			result.KubernetesNamespaces = append(result.KubernetesNamespaces, item)
		}
		sort.Strings(result.KubernetesNamespaces)
	}
	// Record only key names, never values, so the isolated profile can reject
	// credential/path configuration even when another setting disables its lane.
	for _, key := range append([]string{dockerSocketPathKey, kubernetesTokenFileKey, kubernetesCAFileKey}, providerProfileKeys[:7]...) {
		if _, present := getter.LookupEnv(key); present {
			result.providedProfileInputs = append(result.providedProfileInputs, key)
		}
	}
	if result.DemoDeployment {
		for _, key := range []string{webhookReferenceKey, webhookURLKey, webhookSigningKeyKey, matrixReferenceKey, matrixOriginKey, matrixRoomIDKey, matrixAccessTokenKey} {
			if _, present := getter.LookupEnv(key); present {
				return invalid(key, "is forbidden in a demonstration deployment")
			}
		}
	}
	return nil
}

func validateProviderProfiles(result Config) error {
	profile := result.DockerAccessProfile
	if profile == "" {
		profile = DockerProfileSocketDevelopment
	}
	if profile != DockerProfileSocketDevelopment && profile != DockerProfileRestrictedProxy {
		return invalid(dockerAccessProfileKey, "is invalid")
	}
	proxyConfigured := result.DockerProxyOrigin != "" || result.DockerProxyCAFile != "" || result.DockerProxyCertFile != "" || result.DockerProxyKeyFile != ""
	if profile == DockerProfileRestrictedProxy {
		origin, err := url.Parse(result.DockerProxyOrigin)
		if err != nil || origin.Scheme != "https" || origin.Hostname() == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || len(result.DockerProxyOrigin) > 2048 {
			return invalid(dockerProxyOriginKey, "must name one HTTPS origin")
		}
		for _, field := range []struct{ key, value string }{{dockerProxyCAFileKey, result.DockerProxyCAFile}, {dockerProxyCertFileKey, result.DockerProxyCertFile}, {dockerProxyKeyFileKey, result.DockerProxyKeyFile}} {
			if !validAbsolutePath(field.value) {
				return invalid(field.key, "must be an absolute operator file reference")
			}
		}
		if result.DockerWriteEnabled {
			return invalid(dockerWriteEnabledKey, "is forbidden on the observation proxy")
		}
	} else if proxyConfigured {
		return invalid(dockerAccessProfileKey, "must select the restricted proxy for proxy references")
	}
	if result.DockerEnabled && profile == DockerProfileSocketDevelopment {
		listenerHost, _, _ := parseAddress(result.HTTPAddress)
		if result.Environment != EnvironmentDevelopment || !isLoopbackHost(listenerHost) || !isLoopbackHost(result.PublicOrigin.Hostname()) {
			return invalid(dockerEnabledKey, "requires the loopback development socket profile")
		}
	}
	if result.KubernetesEnabled && (len(result.KubernetesNamespaces) < 1 || len(result.KubernetesNamespaces) > 64) {
		return invalid(kubernetesNamespacesKey, "requires one to 64 explicit namespace names")
	}
	seen := map[string]struct{}{}
	for _, namespace := range result.KubernetesNamespaces {
		if !namespaceLabel.MatchString(namespace) {
			return invalid(kubernetesNamespacesKey, "is invalid")
		}
		if _, duplicate := seen[namespace]; duplicate {
			return invalid(kubernetesNamespacesKey, "is duplicated")
		}
		seen[namespace] = struct{}{}
	}
	if result.GuestDemo && !result.DemoDeployment {
		return invalid(guestDemoKey, "requires an isolated demonstration deployment")
	}
	if result.DemoDeployment {
		if result.DockerEnabled || result.KubernetesEnabled || result.MutationsEnabled || result.DockerWriteEnabled || result.KubernetesWriteEnabled || result.WebhookEnabled || result.MatrixEnabled ||
			result.WebhookURL.Present() || result.WebhookSigningKey.Present() || result.WebhookDestinationReferenceID != "" || result.MatrixOrigin.Present() || result.MatrixAccessToken.Present() || result.MatrixRoomID.Present() || result.MatrixDestinationReferenceID != "" || len(result.providedProfileInputs) > 0 || proxyConfigured || profile == DockerProfileRestrictedProxy || len(result.KubernetesNamespaces) > 0 || result.KubernetesClusterObservation ||
			result.DockerSocketPath != defaultDockerSocketPath || result.KubernetesTokenFile != defaultKubernetesTokenFile || result.KubernetesCAFile != defaultKubernetesCAFile {
			return invalid(demoDeploymentKey, "forbids real providers, outbound configuration and infrastructure writes")
		}
		if !validAbsolutePath(result.DemoDataDirectory) || validateDataDirectory(result.DemoDataDirectory, result.Environment) != nil {
			return invalid(demoDataDirectoryKey, "requires a dedicated absolute directory")
		}
		normal, err := filepath.Abs(result.DataDirectory)
		if err != nil {
			return invalid(dataDirectoryKey, "is invalid")
		}
		demo := filepath.Clean(result.DemoDataDirectory)
		if strings.EqualFold(demo, filepath.Clean(normal)) {
			return invalid(demoDataDirectoryKey, "must differ from the normal runtime directory")
		}
	} else if result.DemoDataDirectory != "" {
		return invalid(demoDataDirectoryKey, "requires a demonstration deployment")
	}
	return nil
}
