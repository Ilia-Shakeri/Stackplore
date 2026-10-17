package config

import (
	"path/filepath"
	"strings"
)

const (
	kubernetesAccessProfileKey  = prefix + "KUBERNETES_ACCESS_PROFILE"
	kubernetesKubeconfigFileKey = prefix + "KUBERNETES_KUBECONFIG_FILE"
	kubernetesContextKey        = prefix + "KUBERNETES_CONTEXT"
	kubernetesReferenceRootsKey = prefix + "KUBERNETES_REFERENCE_ROOTS"
	KubernetesProfileInCluster  = "kubernetes_incluster"
	KubernetesProfileKubeconfig = "kubernetes_kubeconfig"
)

var kubeconfigKeys = []string{kubernetesAccessProfileKey, kubernetesKubeconfigFileKey, kubernetesContextKey, kubernetesReferenceRootsKey}

type KubeconfigProfile struct {
	KubernetesAccessProfile  string
	KubernetesKubeconfigFile string
	KubernetesContext        string
	KubernetesReferenceRoots []string
}

func loadKubeconfig(getter Getter, result *Config) error {
	for _, field := range []struct {
		key    string
		target *string
	}{{kubernetesAccessProfileKey, &result.KubernetesAccessProfile}, {kubernetesKubeconfigFileKey, &result.KubernetesKubeconfigFile}, {kubernetesContextKey, &result.KubernetesContext}} {
		value, present, err := readValue(getter, field.key)
		if err != nil {
			return err
		}
		if present {
			*field.target = value
		}
	}
	if value, present, err := readValue(getter, kubernetesReferenceRootsKey); err != nil {
		return err
	} else if present {
		result.KubernetesReferenceRoots = strings.Split(value, string(filepath.ListSeparator))
	}
	if result.DemoDeployment {
		for _, key := range kubeconfigKeys {
			if _, present := getter.LookupEnv(key); present {
				return invalid(key, "is forbidden in a demonstration deployment")
			}
		}
	}
	if result.KubernetesAccessProfile == KubernetesProfileKubeconfig {
		for _, key := range []string{kubernetesTokenFileKey, kubernetesCAFileKey} {
			if _, present := getter.LookupEnv(key); present {
				return invalid(key, "cannot mix in-cluster and kubeconfig credential references")
			}
		}
	}
	return nil
}

func validateKubeconfig(result Config) error {
	profile := result.KubernetesAccessProfile
	if profile == "" {
		profile = KubernetesProfileInCluster
	}
	if profile != KubernetesProfileInCluster && profile != KubernetesProfileKubeconfig {
		return invalid(kubernetesAccessProfileKey, "is invalid")
	}
	configured := result.KubernetesKubeconfigFile != "" || result.KubernetesContext != "" || len(result.KubernetesReferenceRoots) > 0
	if profile == KubernetesProfileInCluster {
		if configured {
			return invalid(kubernetesAccessProfileKey, "requires kubeconfig profile for mounted references")
		}
		return nil
	}
	if !result.KubernetesEnabled || result.KubernetesWriteEnabled || result.DemoDeployment {
		return invalid(kubernetesAccessProfileKey, "requires read-only Kubernetes observation")
	}
	if !validAbsolutePath(result.KubernetesKubeconfigFile) || len(result.KubernetesReferenceRoots) < 1 || len(result.KubernetesReferenceRoots) > 8 {
		return invalid(kubernetesKubeconfigFileKey, "requires an absolute file and one to eight mounted roots")
	}
	if len(result.KubernetesContext) > 256 || strings.ContainsAny(result.KubernetesContext, "\x00\r\n\t") {
		return invalid(kubernetesContextKey, "is invalid")
	}
	found := false
	seen := map[string]bool{}
	for _, root := range result.KubernetesReferenceRoots {
		if !validAbsolutePath(root) || filepath.Dir(root) == root || seen[strings.ToLower(root)] {
			return invalid(kubernetesReferenceRootsKey, "requires distinct absolute non-root directories")
		}
		seen[strings.ToLower(root)] = true
		relative, err := filepath.Rel(root, result.KubernetesKubeconfigFile)
		if err == nil && relative != "." && filepath.IsLocal(relative) {
			found = true
		}
	}
	if !found {
		return invalid(kubernetesKubeconfigFileKey, "must be beneath a mounted root")
	}
	return nil
}
