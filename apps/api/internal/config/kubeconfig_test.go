package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMountedKubeconfigConfigurationIsExplicitReadOnlyAndRootBound(t *testing.T) {
	directory := t.TempDir()
	base := map[string]string{kubernetesEnabledKey: "true", kubernetesNamespacesKey: "shop", kubernetesAccessProfileKey: KubernetesProfileKubeconfig, kubernetesKubeconfigFileKey: filepath.Join(directory, "config"), kubernetesReferenceRootsKey: directory, kubernetesContextKey: "production"}
	result, err := Load(getter(base))
	if err != nil || result.KubernetesAccessProfile != KubernetesProfileKubeconfig || result.KubernetesContext != "production" {
		t.Fatalf("explicit mounted profile rejected: %v", err)
	}
	for name, change := range map[string]map[string]string{
		"outside root": {kubernetesKubeconfigFileKey: filepath.Join(t.TempDir(), "config")},
		"missing root": {kubernetesReferenceRootsKey: ""}, "wrong profile": {kubernetesAccessProfileKey: "unknown"},
		"incluster mixed": {kubernetesTokenFileKey: filepath.Join(directory, "token")},
		"writer reuse":    {mutationsEnabledKey: "true", kubernetesWriteEnabledKey: "true", kubernetesWriterTokenFileKey: filepath.Join(directory, "writer-token"), kubernetesWriterCAFileKey: filepath.Join(directory, "ca"), kubernetesWriterNamespacesKey: "shop"},
		"disabled source": {kubernetesEnabledKey: "false"},
		"duplicate root":  {kubernetesReferenceRootsKey: directory + string(filepath.ListSeparator) + directory},
	} {
		t.Run(name, func(t *testing.T) {
			values := map[string]string{}
			for k, v := range base {
				values[k] = v
			}
			for k, v := range change {
				values[k] = v
			}
			if _, err := Load(getter(values)); err == nil {
				t.Fatal("unsafe mounted profile accepted")
			}
		})
	}
}

func TestAdditiveLogRedactionFieldsAreClosedAndBounded(t *testing.T) {
	result, err := Load(getter(map[string]string{logRedactionFieldsKey: "customer-id,InternalKey"}))
	if err != nil || len(result.LogRedactionFields) != 2 {
		t.Fatal("valid additive field policy rejected")
	}
	for _, value := range []string{"", "two words", "1invalid", "secret,SECRET", "customer-id,customer_id", strings.Repeat("a", 65), strings.Repeat("a,", 32) + "b"} {
		if _, err := Load(getter(map[string]string{logRedactionFieldsKey: value})); err == nil {
			t.Fatal("unsafe additive field policy accepted")
		}
	}
}
