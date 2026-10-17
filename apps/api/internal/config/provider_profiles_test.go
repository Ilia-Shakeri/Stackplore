package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRestrictedDockerProfileRequiresExactSecureReferences(t *testing.T) {
	base := map[string]string{dockerEnabledKey: "true", dockerAccessProfileKey: string(DockerProfileRestrictedProxy), dockerProxyOriginKey: "https://docker-proxy.example:8443",
		dockerProxyCAFileKey: sourceAbsolutePath("docker-ca.pem"), dockerProxyCertFileKey: sourceAbsolutePath("docker-client.pem"), dockerProxyKeyFileKey: sourceAbsolutePath("docker-client-key.pem"), httpAddressKey: "0.0.0.0:8080"}
	if _, err := Load(getter(base)); err != nil {
		t.Fatal(err)
	}
	for name, replace := range map[string]map[string]string{
		"http": {dockerProxyOriginKey: "http://docker-proxy.example"}, "credentials": {dockerProxyOriginKey: "https://user:private@docker-proxy.example"},
		"path": {dockerProxyOriginKey: "https://docker-proxy.example/api"}, "query": {dockerProxyOriginKey: "https://docker-proxy.example?target=other"},
		"relative CA": {dockerProxyCAFileKey: "relative.pem"}, "missing identity": {dockerProxyCertFileKey: ""},
		"write lane": {mutationsEnabledKey: "true", dockerWriteEnabledKey: "true"},
	} {
		t.Run(name, func(t *testing.T) {
			values := map[string]string{}
			for key, value := range base {
				values[key] = value
			}
			for key, value := range replace {
				values[key] = value
			}
			if _, err := Load(getter(values)); err == nil {
				t.Fatal("unsafe Docker profile accepted")
			} else if strings.Contains(err.Error(), "private") {
				t.Fatal("raw configuration value leaked")
			}
		})
	}
	if _, err := Load(getter(map[string]string{dockerEnabledKey: "true", httpAddressKey: "0.0.0.0:8080"})); err == nil {
		t.Fatal("development socket exposed")
	}
}

func TestKubernetesNamespacesAreExplicitBoundedAndCanonical(t *testing.T) {
	base := map[string]string{kubernetesEnabledKey: "true", kubernetesTokenFileKey: sourceAbsolutePath("token"), kubernetesCAFileKey: sourceAbsolutePath("ca.pem"), kubernetesNamespacesKey: "shop,ops,shop", kubernetesClusterObservationKey: "true"}
	value, err := Load(getter(base))
	if err != nil || !reflect.DeepEqual(value.KubernetesNamespaces, []string{"ops", "shop"}) || !value.KubernetesClusterObservation {
		t.Fatalf("namespace configuration wrong: %v", err)
	}
	for _, input := range []string{"", "*", "shop, ops", "Shop", "../shop", strings.Repeat("a", 64), strings.Repeat("shop,", 64) + "last"} {
		values := map[string]string{}
		for key, value := range base {
			values[key] = value
		}
		values[kubernetesNamespacesKey] = input
		if _, err := Load(getter(values)); err == nil {
			t.Fatal("unsafe namespace scope accepted")
		}
	}
	delete(base, kubernetesNamespacesKey)
	if _, err := Load(getter(base)); err == nil {
		t.Fatal("implicit cluster scope accepted")
	}
}

func TestDemoDeploymentRejectsRealProfilesAndOutboundInputs(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "isolated-demo")
	base := map[string]string{demoDeploymentKey: "true", guestDemoKey: "true", demoDataDirectoryKey: directory, httpAddressKey: "0.0.0.0:8080"}
	settings, err := Load(getter(base))
	if err != nil || SetupBootstrapRequired(settings) {
		t.Fatalf("isolated guest profile failed: %v", err)
	}
	for _, key := range []string{dockerSocketPathKey, kubernetesTokenFileKey, kubernetesCAFileKey, dockerProxyOriginKey, webhookURLKey, matrixAccessTokenKey} {
		t.Run(key, func(t *testing.T) {
			values := map[string]string{}
			for name, value := range base {
				values[name] = value
			}
			values[key] = ""
			if _, err := Load(getter(values)); err == nil {
				t.Fatal("provided credential/path input accepted in demo")
			}
		})
	}
	for _, key := range []string{dockerEnabledKey, kubernetesEnabledKey, mutationsEnabledKey, webhookEnabledKey, matrixEnabledKey} {
		values := map[string]string{}
		for name, value := range base {
			values[name] = value
		}
		values[key] = "true"
		if _, err := Load(getter(values)); err == nil {
			t.Fatal("real activity accepted in demo")
		}
	}
	if _, err := Load(getter(map[string]string{guestDemoKey: "true"})); err == nil {
		t.Fatal("guest mode allowed in a normal installation")
	}
	if _, err := Load(getter(map[string]string{demoDeploymentKey: "true", demoDataDirectoryKey: "relative"})); err == nil {
		t.Fatal("relative demo directory accepted")
	}
	normal, err := filepath.Abs("data")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(getter(map[string]string{demoDeploymentKey: "true", demoDataDirectoryKey: normal})); err == nil {
		t.Fatal("normal runtime directory reused")
	}
	values := map[string]string{demoDeploymentKey: "true", demoDataDirectoryKey: directory, httpAddressKey: "0.0.0.0:8080"}
	settings, err = Load(getter(values))
	if err != nil || !SetupBootstrapRequired(settings) {
		t.Fatal("authenticated demo lost its setup gate")
	}
}
