package config

import (
	"path/filepath"
	"slices"
	"strings"
)

const (
	kubernetesWriterTokenFileKey  = prefix + "KUBERNETES_WRITER_TOKEN_FILE"
	kubernetesWriterCAFileKey     = prefix + "KUBERNETES_WRITER_CA_FILE"
	kubernetesWriterNamespacesKey = prefix + "KUBERNETES_WRITER_NAMESPACES"
)

type KubernetesOperationProfile struct {
	KubernetesWriterTokenFile  string
	KubernetesWriterCAFile     string
	KubernetesWriterNamespaces []string
}

var kubernetesOperationKeys = []string{kubernetesWriterTokenFileKey, kubernetesWriterCAFileKey, kubernetesWriterNamespacesKey}

func loadKubernetesOperations(getter Getter, result *Config) error {
	if result.DemoDeployment {
		for _, key := range kubernetesOperationKeys {
			if _, present := getter.LookupEnv(key); present {
				return invalid(key, "is forbidden in a demonstration deployment")
			}
		}
	}
	for _, field := range []struct {
		key    string
		target *string
	}{{kubernetesWriterTokenFileKey, &result.KubernetesWriterTokenFile}, {kubernetesWriterCAFileKey, &result.KubernetesWriterCAFile}} {
		value, present, err := readValue(getter, field.key)
		if err != nil {
			return err
		}
		if present {
			*field.target = value
		}
	}
	if value, present, err := readValue(getter, kubernetesWriterNamespacesKey); err != nil {
		return err
	} else if present {
		result.KubernetesWriterNamespaces = strings.Split(value, ",")
	}
	return nil
}

func validateKubernetesOperations(result Config) error {
	configured := result.KubernetesWriterTokenFile != "" || result.KubernetesWriterCAFile != "" || len(result.KubernetesWriterNamespaces) > 0
	if !result.KubernetesWriteEnabled {
		if configured {
			return invalid(kubernetesWriteEnabledKey, "requires explicit write permission for writer references")
		}
		return nil
	}
	if !result.KubernetesEnabled || !result.MutationsEnabled || result.DemoDeployment {
		return invalid(kubernetesWriteEnabledKey, "requires a real enabled provider and infrastructure mutations")
	}
	if !validAbsolutePath(result.KubernetesWriterTokenFile) || !validAbsolutePath(result.KubernetesWriterCAFile) {
		return invalid(kubernetesWriterTokenFileKey, "requires absolute operator credential and CA file references")
	}
	if strings.EqualFold(filepath.Clean(result.KubernetesWriterTokenFile), filepath.Clean(result.KubernetesTokenFile)) {
		return invalid(kubernetesWriterTokenFileKey, "must differ from the observation credential reference")
	}
	if len(result.KubernetesWriterNamespaces) < 1 || len(result.KubernetesWriterNamespaces) > 64 {
		return invalid(kubernetesWriterNamespacesKey, "requires one to 64 explicit namespace names")
	}
	seen := map[string]bool{}
	for _, namespace := range result.KubernetesWriterNamespaces {
		if !namespaceLabel.MatchString(namespace) || !slices.Contains(result.KubernetesNamespaces, namespace) || seen[namespace] {
			return invalid(kubernetesWriterNamespacesKey, "must contain unique observation namespace names")
		}
		seen[namespace] = true
	}
	return nil
}
