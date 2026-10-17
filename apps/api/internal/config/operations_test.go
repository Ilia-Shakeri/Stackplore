package config

import (
	"path/filepath"
	"testing"
)

func TestWriterProfilesRequireSeparateExplicitPermissionAndCredentials(t *testing.T) {
	docker := map[string]string{dockerEnabledKey: "true", mutationsEnabledKey: "true", dockerWriteEnabledKey: "true", dockerAccessProfileKey: string(DockerProfileRestrictedProxy),
		dockerProxyOriginKey: "https://docker-reader.example:8443", dockerProxyCAFileKey: sourceAbsolutePath("server-ca.pem"), dockerProxyCertFileKey: sourceAbsolutePath("reader.pem"), dockerProxyKeyFileKey: sourceAbsolutePath("reader-key.pem"),
		dockerWriterOriginKey: "https://docker-writer.example:8443", dockerWriterCAFileKey: sourceAbsolutePath("server-ca.pem"), dockerWriterCertFileKey: sourceAbsolutePath("writer.pem"), dockerWriterKeyFileKey: sourceAbsolutePath("writer-key.pem")}
	kube := map[string]string{kubernetesEnabledKey: "true", mutationsEnabledKey: "true", kubernetesWriteEnabledKey: "true", kubernetesTokenFileKey: sourceAbsolutePath("reader-token"), kubernetesCAFileKey: sourceAbsolutePath("server-ca.pem"), kubernetesNamespacesKey: "shop,ops", kubernetesWriterNamespacesKey: "shop", kubernetesWriterTokenFileKey: sourceAbsolutePath("writer-token"), kubernetesWriterCAFileKey: sourceAbsolutePath("writer-ca.pem")}
	for name, base := range map[string]map[string]string{"docker": docker, "kubernetes": kube} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(getter(base)); err != nil {
				t.Fatal(err)
			}
			changes := []map[string]string{{mutationsEnabledKey: "false"}}
			if name == "docker" {
				changes = append(changes, map[string]string{dockerWriteEnabledKey: "false"}, map[string]string{dockerWriterOriginKey: "http://writer.example"}, map[string]string{dockerWriterOriginKey: base[dockerProxyOriginKey]}, map[string]string{dockerWriterCertFileKey: base[dockerProxyCertFileKey]}, map[string]string{dockerWriterKeyFileKey: base[dockerProxyKeyFileKey]}, map[string]string{dockerWriterCAFileKey: "relative"})
			} else {
				changes = append(changes, map[string]string{kubernetesWriteEnabledKey: "false"}, map[string]string{kubernetesWriterTokenFileKey: base[kubernetesTokenFileKey]}, map[string]string{kubernetesWriterNamespacesKey: "shop,shop"}, map[string]string{kubernetesWriterNamespacesKey: "other"}, map[string]string{kubernetesWriterNamespacesKey: "*"}, map[string]string{kubernetesWriterCAFileKey: "relative"})
			}
			for _, change := range changes {
				values := map[string]string{}
				for key, value := range base {
					values[key] = value
				}
				for key, value := range change {
					values[key] = value
				}
				if _, err := Load(getter(values)); err == nil {
					t.Fatal("unsafe writer configuration accepted")
				}
			}
		})
	}
	for _, key := range append(append([]string{}, dockerOperationKeys...), kubernetesOperationKeys...) {
		if _, err := Load(getter(map[string]string{demoDeploymentKey: "true", guestDemoKey: "true", demoDataDirectoryKey: filepath.Join(t.TempDir(), "demo"), key: ""})); err == nil {
			t.Fatal("writer reference accepted in isolated demo")
		}
	}
}

func TestAuditPolicyOperatorKeysReachClosedConfiguration(t *testing.T) {
	settings, err := Load(getter(map[string]string{auditRetentionDaysKey: "730", auditQuotaRowsKey: "2000"}))
	if err != nil || settings.AuditRetentionDays != 730 || settings.AuditQuotaRows != 2000 {
		t.Fatal("audit policy not loaded")
	}
	for _, values := range []map[string]string{{auditRetentionDaysKey: "89"}, {auditQuotaRowsKey: "999"}, {auditArchiveVerifiedThroughKey: "2100-01-01T00:00:00Z"}} {
		if _, err := Load(getter(values)); err == nil {
			t.Fatal("unsafe audit policy accepted")
		}
	}
}
