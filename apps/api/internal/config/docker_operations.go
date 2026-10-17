package config

import (
	"net/url"
	"path/filepath"
	"strings"
)

const (
	dockerWriterOriginKey   = prefix + "DOCKER_WRITER_ORIGIN"
	dockerWriterCAFileKey   = prefix + "DOCKER_WRITER_CA_FILE"
	dockerWriterCertFileKey = prefix + "DOCKER_WRITER_CERT_FILE"
	dockerWriterKeyFileKey  = prefix + "DOCKER_WRITER_KEY_FILE"
)

var dockerOperationKeys = []string{dockerWriterOriginKey, dockerWriterCAFileKey, dockerWriterCertFileKey, dockerWriterKeyFileKey}

type DockerOperationProfile struct {
	DockerWriterOrigin   string
	DockerWriterCAFile   string
	DockerWriterCertFile string
	DockerWriterKeyFile  string
}

func loadDockerOperations(getter Getter, result *Config) error {
	for _, field := range []struct {
		key    string
		target *string
	}{
		{dockerWriterOriginKey, &result.DockerWriterOrigin}, {dockerWriterCAFileKey, &result.DockerWriterCAFile},
		{dockerWriterCertFileKey, &result.DockerWriterCertFile}, {dockerWriterKeyFileKey, &result.DockerWriterKeyFile},
	} {
		if _, present := getter.LookupEnv(field.key); present && result.DemoDeployment {
			return invalid(field.key, "is forbidden in a demonstration deployment")
		}
		value, present, err := readValue(getter, field.key)
		if err != nil {
			return err
		}
		if present {
			*field.target = value
		}
	}
	return nil
}

func validateDockerOperations(result Config) error {
	configured := result.DockerWriterOrigin != "" || result.DockerWriterCAFile != "" || result.DockerWriterCertFile != "" || result.DockerWriterKeyFile != ""
	if !result.DockerWriteEnabled {
		if configured {
			return invalid(dockerWriteEnabledKey, "requires explicit write permission for writer references")
		}
		return nil
	}
	if !result.DockerEnabled || !result.MutationsEnabled || result.DemoDeployment {
		return invalid(dockerWriteEnabledKey, "requires a real enabled provider and infrastructure mutations")
	}
	origin, err := url.Parse(result.DockerWriterOrigin)
	if err != nil || origin.Scheme != "https" || origin.Hostname() == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || len(result.DockerWriterOrigin) > 2048 {
		return invalid(dockerWriterOriginKey, "must name one HTTPS writer origin")
	}
	if strings.EqualFold(result.DockerWriterOrigin, result.DockerProxyOrigin) {
		return invalid(dockerWriterOriginKey, "must differ from the observation origin")
	}
	for _, field := range []struct{ key, value string }{{dockerWriterCAFileKey, result.DockerWriterCAFile}, {dockerWriterCertFileKey, result.DockerWriterCertFile}, {dockerWriterKeyFileKey, result.DockerWriterKeyFile}} {
		if !validAbsolutePath(field.value) {
			return invalid(field.key, "must be an absolute operator file reference")
		}
	}
	for _, pair := range [][2]string{{result.DockerWriterCertFile, result.DockerProxyCertFile}, {result.DockerWriterKeyFile, result.DockerProxyKeyFile}} {
		if pair[1] != "" && strings.EqualFold(filepath.Clean(pair[0]), filepath.Clean(pair[1])) {
			return invalid(dockerWriterCertFileKey, "must differ from observation credential references")
		}
	}
	return nil
}
