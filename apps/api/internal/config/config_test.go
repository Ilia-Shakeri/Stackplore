package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type mapGetter struct {
	values  map[string]string
	entries []string
}

func (g mapGetter) LookupEnv(key string) (string, bool) {
	value, ok := g.values[key]
	return value, ok
}

func (g mapGetter) Environ() []string { return append([]string(nil), g.entries...) }

func getter(values map[string]string) mapGetter {
	entries := make([]string, 0, len(values))
	for key, value := range values {
		entries = append(entries, key+"="+value)
	}
	return mapGetter{values: values, entries: entries}
}

func productionDataDirectory() string {
	if filepath.Separator == '\\' {
		return `C:\stackplore\data`
	}
	return "/var/lib/stackplore"
}

func bootstrapEnvironment(values map[string]string) map[string]string {
	result := make(map[string]string, len(values)+2)
	for key, value := range values {
		result[key] = value
	}
	result[setupBootstrapProofKey] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	result[setupBootstrapExpiryKey] = time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second).Format(time.RFC3339)
	return result
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	got, err := Load(getter(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Environment != EnvironmentDevelopment {
		t.Errorf("Environment = %q; want %q", got.Environment, EnvironmentDevelopment)
	}
	if got.HTTPAddress != "127.0.0.1:8080" {
		t.Errorf("HTTPAddress = %q; want loopback default", got.HTTPAddress)
	}
	if got.PublicOrigin.String() != "http://127.0.0.1:8080" {
		t.Errorf("PublicOrigin = %q; want derived loopback origin", got.PublicOrigin.String())
	}
	wantMetrics := []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
	}
	if !reflect.DeepEqual(got.MetricsAllowedPrefixes, wantMetrics) {
		t.Errorf("MetricsAllowedPrefixes = %v; want %v", got.MetricsAllowedPrefixes, wantMetrics)
	}
	if len(got.TrustedProxyPrefixes) != 0 {
		t.Errorf("TrustedProxyPrefixes = %v; want empty", got.TrustedProxyPrefixes)
	}
	if got.LogLevel != slog.LevelInfo || got.LogFormat != LogFormatJSON {
		t.Errorf("logging = (%v, %q); want info and json", got.LogLevel, got.LogFormat)
	}
	if got.ReadHeaderTimeout != 5*time.Second || got.ReadTimeout != 15*time.Second || got.WriteTimeout != 30*time.Second || got.IdleTimeout != time.Minute || got.RequestTimeout != 15*time.Second || got.ShutdownTimeout != 15*time.Second {
		t.Errorf("timeouts = %+v; want bounded defaults", got)
	}
	if got.MaxHeaderBytes != 32*1024 || got.MaxRequestBodyBytes != 256*1024 || got.MaximumResponseBytes != 16*1024*1024 || got.MaxConnections != 512 || got.MaxInFlight != 256 {
		t.Errorf("bounds = (%d, %d, %d, %d, %d); want defaults", got.MaxHeaderBytes, got.MaxRequestBodyBytes, got.MaximumResponseBytes, got.MaxConnections, got.MaxInFlight)
	}
	if got.RatePerSecond != 20 || got.RateBurst != 40 || got.RateMaxClients != 4096 {
		t.Errorf("rate limits = (%v, %d, %d); want defaults", got.RatePerSecond, got.RateBurst, got.RateMaxClients)
	}
	if got.DataDirectory != "data" || got.DatabaseBusyTimeout != 5*time.Second {
		t.Errorf("database = (%q, %s); want data and 5s", got.DataDirectory, got.DatabaseBusyTimeout)
	}
}

func TestSetupBootstrapIsBoundedAndRedacted(t *testing.T) {
	values := bootstrapEnvironment(map[string]string{})
	got, err := Load(getter(values))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !SetupBootstrapRequired(got) || !got.SetupBootstrapProof.Present() || len(got.SetupBootstrapProof.Bytes()) != 32 {
		t.Fatal("configured bootstrap proof was not preserved")
	}
	if formatted := fmt.Sprintf("%+v", got); strings.Contains(formatted, values[setupBootstrapProofKey]) {
		t.Fatal("formatted configuration disclosed bootstrap proof")
	}

	copyOne := got.SetupBootstrapProof.Bytes()
	copyOne[0] = 1
	if got.SetupBootstrapProof.Bytes()[0] != 0 {
		t.Fatal("bootstrap proof returned shared storage")
	}
}

func TestRemoteSetupMarksBootstrapProofRequired(t *testing.T) {
	got, err := Load(getter(map[string]string{httpAddressKey: "0.0.0.0:8080"}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !SetupBootstrapRequired(got) || got.SetupBootstrapProof.Present() {
		t.Fatal("remote setup requirement was not preserved")
	}
}

func TestSetupBootstrapRejectsInvalidPairsAndLifetime(t *testing.T) {
	future := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second).Format(time.RFC3339)
	tests := []struct {
		name   string
		values map[string]string
		field  string
	}{
		{"proof alone", map[string]string{setupBootstrapProofKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, setupBootstrapExpiryKey},
		{"expiry alone", map[string]string{setupBootstrapExpiryKey: future}, setupBootstrapProofKey},
		{"bad proof", map[string]string{setupBootstrapProofKey: strings.Repeat("x", 43), setupBootstrapExpiryKey: future}, setupBootstrapProofKey},
		{"too long", map[string]string{setupBootstrapProofKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", setupBootstrapExpiryKey: time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second).Format(time.RFC3339)}, setupBootstrapExpiryKey},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Load(getter(test.values))
			assertFieldError(t, err, test.field)
		})
	}
}

func TestLoadAllFields(t *testing.T) {
	t.Parallel()

	got, err := Load(getter(bootstrapEnvironment(map[string]string{
		environmentKey:            "production",
		httpAddressKey:            "0.0.0.0:8443",
		publicOriginKey:           "https://stackplore.example:8443",
		trustedProxyPrefixesKey:   "10.0.0.0/8,2001:db8::/32",
		metricsAllowedPrefixesKey: "10.20.0.0/16,2001:db8:1::/48",
		logLevelKey:               "debug",
		logFormatKey:              "text",
		readHeaderTimeoutKey:      "2s",
		readTimeoutKey:            "12s",
		writeTimeoutKey:           "25s",
		idleTimeoutKey:            "2m",
		requestTimeoutKey:         "20s",
		shutdownTimeoutKey:        "30s",
		maxHeaderBytesKey:         "65536",
		maxRequestBodyBytesKey:    "1048576",
		maximumResponseBytesKey:   "2097152",
		maxConnectionsKey:         "1024",
		maxInFlightKey:            "128",
		ratePerSecondKey:          "12.5",
		rateBurstKey:              "25",
		rateMaxClientsKey:         "2048",
		dataDirectoryKey:          productionDataDirectory(),
		databaseBusyTimeoutKey:    "10s",
		developmentAPIOriginKey:   "http://127.0.0.1:8080",
	})))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Environment != EnvironmentProduction || got.HTTPAddress != "0.0.0.0:8443" || got.PublicOrigin.String() != "https://stackplore.example:8443" {
		t.Errorf("identity fields = (%q, %q, %q); want explicit values", got.Environment, got.HTTPAddress, got.PublicOrigin.String())
	}
	if got.LogLevel != slog.LevelDebug || got.LogFormat != LogFormatText {
		t.Errorf("logging = (%v, %q); want debug and text", got.LogLevel, got.LogFormat)
	}
	if got.ReadHeaderTimeout != 2*time.Second || got.ReadTimeout != 12*time.Second || got.WriteTimeout != 25*time.Second || got.IdleTimeout != 2*time.Minute || got.RequestTimeout != 20*time.Second || got.ShutdownTimeout != 30*time.Second {
		t.Error("configured timeouts were not preserved")
	}
	if got.MaxHeaderBytes != 65536 || got.MaxRequestBodyBytes != 1048576 || got.MaximumResponseBytes != 2097152 || got.MaxConnections != 1024 || got.MaxInFlight != 128 || got.RatePerSecond != 12.5 || got.RateBurst != 25 || got.RateMaxClients != 2048 {
		t.Error("configured bounds were not preserved")
	}
	if got.DataDirectory != productionDataDirectory() || got.DatabaseBusyTimeout != 10*time.Second {
		t.Error("configured database values were not preserved")
	}
	if want := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("2001:db8::/32")}; !reflect.DeepEqual(got.TrustedProxyPrefixes, want) {
		t.Errorf("TrustedProxyPrefixes = %v; want %v", got.TrustedProxyPrefixes, want)
	}
}

func TestPublicOriginDerivesFromAddress(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		address string
		origin  string
	}{
		{"IPv4", "127.0.0.2:9000", "http://127.0.0.2:9000"},
		{"IPv6", "[::1]:9000", "http://[::1]:9000"},
		{"host", "localhost:9000", "http://localhost:9000"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := Load(getter(map[string]string{httpAddressKey: test.address}))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got.PublicOrigin.String() != test.origin {
				t.Errorf("PublicOrigin = %q; want %q", got.PublicOrigin.String(), test.origin)
			}
		})
	}
}

func TestLoadRejectsInvalidFieldValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"blank environment", environmentKey, ""},
		{"spaced environment", environmentKey, " production"},
		{"unknown environment", environmentKey, "staging"},
		{"blank address", httpAddressKey, ""},
		{"missing listener host", httpAddressKey, ":8080"},
		{"missing listener port", httpAddressKey, "localhost"},
		{"zero listener port", httpAddressKey, "localhost:0"},
		{"leading zero listener port", httpAddressKey, "localhost:08080"},
		{"named listener port", httpAddressKey, "localhost:http"},
		{"large listener port", httpAddressKey, "localhost:65536"},
		{"uppercase listener host", httpAddressKey, "LOCALHOST:8080"},
		{"trailing dot listener host", httpAddressKey, "localhost.:8080"},
		{"underscore listener host", httpAddressKey, "bad_host:8080"},
		{"noncanonical listener IPv6", httpAddressKey, "[0:0::1]:8080"},
		{"origin trailing slash", publicOriginKey, "https://example.com/"},
		{"origin path", publicOriginKey, "https://example.com/path"},
		{"origin query", publicOriginKey, "https://example.com?x=1"},
		{"origin fragment", publicOriginKey, "https://example.com#part"},
		{"origin user", publicOriginKey, "https://user@example.com"},
		{"origin scheme", publicOriginKey, "ftp://example.com"},
		{"origin default HTTP port", publicOriginKey, "http://example.com:80"},
		{"origin default HTTPS port", publicOriginKey, "https://example.com:443"},
		{"origin uppercase host", publicOriginKey, "https://EXAMPLE.com"},
		{"origin missing host", publicOriginKey, "https:///path"},
		{"development origin blank", developmentAPIOriginKey, ""},
		{"development origin path", developmentAPIOriginKey, "http://127.0.0.1:8080/path"},
		{"trusted blank", trustedProxyPrefixesKey, ""},
		{"trusted broad IPv4", trustedProxyPrefixesKey, "0.0.0.0/0"},
		{"trusted broad IPv6", trustedProxyPrefixesKey, "::/0"},
		{"trusted host bits", trustedProxyPrefixesKey, "10.0.0.1/8"},
		{"trusted mapped IPv4", trustedProxyPrefixesKey, "::ffff:192.0.2.0/120"},
		{"trusted duplicate", trustedProxyPrefixesKey, "10.0.0.0/8,10.0.0.0/8"},
		{"trusted list whitespace", trustedProxyPrefixesKey, "10.0.0.0/8, 192.168.0.0/16"},
		{"metrics broad", metricsAllowedPrefixesKey, "0.0.0.0/0"},
		{"metrics mapped IPv4", metricsAllowedPrefixesKey, "::ffff:192.0.2.0/120"},
		{"metrics invalid", metricsAllowedPrefixesKey, "not-a-prefix"},
		{"log level case", logLevelKey, "INFO"},
		{"log level unknown", logLevelKey, "trace"},
		{"log format case", logFormatKey, "JSON"},
		{"log format unknown", logFormatKey, "pretty"},
		{"duration malformed", readTimeoutKey, "soon"},
		{"duration too short", readHeaderTimeoutKey, "999ms"},
		{"duration too long", idleTimeoutKey, "601s"},
		{"header bytes malformed", maxHeaderBytesKey, "large"},
		{"header bytes too small", maxHeaderBytesKey, "4095"},
		{"header bytes too large", maxHeaderBytesKey, "1048577"},
		{"body bytes too small", maxRequestBodyBytesKey, "1023"},
		{"body bytes too large", maxRequestBodyBytesKey, "16777217"},
		{"response bytes too small", maximumResponseBytesKey, "65535"},
		{"response bytes too large", maximumResponseBytesKey, "67108865"},
		{"connections zero", maxConnectionsKey, "0"},
		{"connections too large", maxConnectionsKey, "16385"},
		{"in flight zero", maxInFlightKey, "0"},
		{"in flight too large", maxInFlightKey, "4097"},
		{"rate malformed", ratePerSecondKey, "fast"},
		{"rate nan", ratePerSecondKey, "NaN"},
		{"rate infinity", ratePerSecondKey, "+Inf"},
		{"rate too small", ratePerSecondKey, "0.09"},
		{"rate too large", ratePerSecondKey, "10000.1"},
		{"burst zero", rateBurstKey, "0"},
		{"burst too large", rateBurstKey, "10001"},
		{"clients zero", rateMaxClientsKey, "0"},
		{"clients too large", rateMaxClientsKey, "65537"},
		{"data directory blank", dataDirectoryKey, ""},
		{"data directory dot", dataDirectoryKey, "."},
		{"data directory not clean", dataDirectoryKey, "data/../data"},
		{"data directory trailing separator", dataDirectoryKey, "data/"},
		{"data directory parent escape", dataDirectoryKey, "../data"},
		{"data directory host root", dataDirectoryKey, string(filepath.Separator)},
		{"data directory drive root", dataDirectoryKey, `C:\`},
		{"data directory UNC", dataDirectoryKey, `\\server\share\data`},
		{"data directory device namespace", dataDirectoryKey, `\\.\PhysicalDrive0`},
		{"data directory device name", dataDirectoryKey, "NUL"},
		{"data directory nested device", dataDirectoryKey, "data/CON.txt"},
		{"data directory superscript device", dataDirectoryKey, "data/COM\u00b9.txt"},
		{"data directory control", dataDirectoryKey, "data/\u0001child"},
		{"data directory NUL", dataDirectoryKey, "data\x00child"},
		{"data directory invalid UTF-8", dataDirectoryKey, string([]byte{'d', 'a', 't', 'a', 0xff})},
		{"data directory too long", dataDirectoryKey, strings.Repeat("d", maximumDataDirectoryBytes+1)},
		{"busy timeout malformed", databaseBusyTimeoutKey, "soon"},
		{"busy timeout too short", databaseBusyTimeoutKey, "99ms"},
		{"busy timeout sub-millisecond", databaseBusyTimeoutKey, "100ms1ns"},
		{"busy timeout too long", databaseBusyTimeoutKey, "30s1ns"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(getter(map[string]string{test.key: test.value}))
			assertFieldError(t, err, test.key)
		})
	}
}

func TestLoadAcceptsDocumentedBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values map[string]string
	}{
		{"minimum read header", map[string]string{readHeaderTimeoutKey: "1s"}},
		{"maximum read header", map[string]string{readHeaderTimeoutKey: "30s", readTimeoutKey: "30s"}},
		{"minimum idle", map[string]string{idleTimeoutKey: "1s"}},
		{"maximum idle", map[string]string{idleTimeoutKey: "10m"}},
		{"minimum header", map[string]string{maxHeaderBytesKey: "4096"}},
		{"maximum header", map[string]string{maxHeaderBytesKey: "1048576"}},
		{"minimum body", map[string]string{maxRequestBodyBytesKey: "1024"}},
		{"maximum body", map[string]string{maxRequestBodyBytesKey: "16777216"}},
		{"minimum response", map[string]string{maximumResponseBytesKey: "65536"}},
		{"maximum response", map[string]string{maximumResponseBytesKey: "67108864"}},
		{"minimum connections", map[string]string{maxConnectionsKey: "1"}},
		{"maximum connections", map[string]string{maxConnectionsKey: "16384"}},
		{"minimum in flight", map[string]string{maxInFlightKey: "1"}},
		{"maximum in flight", map[string]string{maxInFlightKey: "4096"}},
		{"minimum rate", map[string]string{ratePerSecondKey: "0.1"}},
		{"maximum rate", map[string]string{ratePerSecondKey: "10000"}},
		{"minimum burst", map[string]string{rateBurstKey: "1"}},
		{"maximum burst", map[string]string{rateBurstKey: "10000"}},
		{"minimum clients", map[string]string{rateMaxClientsKey: "1"}},
		{"maximum clients", map[string]string{rateMaxClientsKey: "65536"}},
		{"maximum data directory bytes", map[string]string{dataDirectoryKey: strings.Repeat("d", maximumDataDirectoryBytes)}},
		{"Unicode data directory", map[string]string{dataDirectoryKey: "dâtâ"}},
		{"minimum busy timeout", map[string]string{databaseBusyTimeoutKey: "100ms"}},
		{"maximum busy timeout", map[string]string{databaseBusyTimeoutKey: "30s"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Load(getter(test.values)); err != nil {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

func TestValidateRejectsForgedTypedConfig(t *testing.T) {
	t.Parallel()
	base, err := Load(getter(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := Validate(base); err != nil {
		t.Fatalf("Validate(default) error = %v", err)
	}

	tests := []struct {
		name   string
		change func(*Config)
	}{
		{"environment", func(value *Config) { value.Environment = Environment("unknown") }},
		{"listener", func(value *Config) { value.HTTPAddress = ":0" }},
		{"origin", func(value *Config) { value.PublicOrigin.Host = "" }},
		{"proxy prefix", func(value *Config) { value.TrustedProxyPrefixes = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")} }},
		{"metrics prefix", func(value *Config) {
			value.MetricsAllowedPrefixes = []netip.Prefix{netip.MustParsePrefix("::ffff:192.0.2.0/120")}
		}},
		{"log level", func(value *Config) { value.LogLevel = slog.Level(12) }},
		{"read header timeout", func(value *Config) { value.ReadHeaderTimeout = 0 }},
		{"header bytes", func(value *Config) { value.MaxHeaderBytes = 0 }},
		{"connections", func(value *Config) { value.MaxConnections = 0 }},
		{"request body", func(value *Config) { value.MaxRequestBodyBytes = 0 }},
		{"response body", func(value *Config) { value.MaximumResponseBytes = 0 }},
		{"rate", func(value *Config) { value.RatePerSecond = 0 }},
		{"data directory", func(value *Config) { value.DataDirectory = "../data" }},
		{"production relative data directory", func(value *Config) { value.Environment = EnvironmentProduction }},
		{"database busy timeout", func(value *Config) { value.DatabaseBusyTimeout = 0 }},
		{"database busy timeout precision", func(value *Config) { value.DatabaseBusyTimeout = 100*time.Millisecond + time.Nanosecond }},
		{"bootstrap expiry without proof", func(value *Config) { value.SetupBootstrapExpiresAt = time.Now().UTC().Truncate(time.Second) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := base
			test.change(&value)
			if err := Validate(value); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate() error = %v; want ErrInvalid", err)
			}
		})
	}
}

func TestLoadRejectsConflictingTimeouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values map[string]string
		field  string
	}{
		{
			name: "read header exceeds read",
			values: map[string]string{
				readHeaderTimeoutKey: "10s",
				readTimeoutKey:       "5s",
			},
			field: readHeaderTimeoutKey,
		},
		{
			name: "request exceeds write",
			values: map[string]string{
				requestTimeoutKey: "10s",
				writeTimeoutKey:   "5s",
			},
			field: requestTimeoutKey,
		},
		{
			name: "request equals write",
			values: map[string]string{
				requestTimeoutKey: "10s",
				writeTimeoutKey:   "10s",
			},
			field: requestTimeoutKey,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(getter(test.values))
			assertFieldError(t, err, test.field)
		})
	}
}

func TestProductionExposureRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  map[string]string
		field   string
		wantErr bool
	}{
		{
			name: "loopback needs no edge",
			values: map[string]string{
				environmentKey:   "production",
				dataDirectoryKey: productionDataDirectory(),
			},
		},
		{
			name: "remote listener requires HTTPS",
			values: map[string]string{
				environmentKey:   "production",
				httpAddressKey:   "0.0.0.0:8080",
				publicOriginKey:  "http://stackplore.example",
				dataDirectoryKey: productionDataDirectory(),
			},
			field: publicOriginKey, wantErr: true,
		},
		{
			name: "remote origin requires HTTPS",
			values: map[string]string{
				environmentKey:   "production",
				publicOriginKey:  "http://stackplore.example",
				dataDirectoryKey: productionDataDirectory(),
			},
			field: publicOriginKey, wantErr: true,
		},
		{
			name: "remote exposure requires proxy",
			values: map[string]string{
				environmentKey:   "production",
				httpAddressKey:   "0.0.0.0:8080",
				publicOriginKey:  "https://stackplore.example",
				dataDirectoryKey: productionDataDirectory(),
			},
			field: trustedProxyPrefixesKey, wantErr: true,
		},
		{
			name: "remote exposure with edge",
			values: bootstrapEnvironment(map[string]string{
				environmentKey:          "production",
				httpAddressKey:          "0.0.0.0:8080",
				publicOriginKey:         "https://stackplore.example",
				trustedProxyPrefixesKey: "10.0.0.0/8",
				dataDirectoryKey:        productionDataDirectory(),
			}),
		},
		{
			name: "development may bind remote with proof",
			values: bootstrapEnvironment(map[string]string{
				httpAddressKey: "0.0.0.0:8080",
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(getter(test.values))
			if test.wantErr {
				assertFieldError(t, err, test.field)
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

func TestDataDirectoryEnvironmentRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  map[string]string
		wantErr bool
	}{
		{
			name: "production rejects relative",
			values: map[string]string{
				environmentKey:   "production",
				dataDirectoryKey: "data",
			},
			wantErr: true,
		},
		{
			name: "production accepts absolute",
			values: map[string]string{
				environmentKey:   "production",
				dataDirectoryKey: productionDataDirectory(),
			},
		},
		{
			name: "development accepts relative",
			values: map[string]string{
				environmentKey:   "development",
				dataDirectoryKey: "local-data",
			},
		},
		{
			name: "test accepts relative",
			values: map[string]string{
				environmentKey:   "test",
				dataDirectoryKey: "test-data",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(getter(test.values))
			if test.wantErr {
				assertFieldError(t, err, dataDirectoryKey)
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

func TestLoadRejectsUnknownAndDuplicateVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		getter mapGetter
		field  string
	}{
		{
			name: "unknown",
			getter: mapGetter{
				values:  map[string]string{prefix + "TYPO": "x"},
				entries: []string{prefix + "TYPO=x"},
			},
			field: prefix + "TYPO",
		},
		{
			name: "in flight typo",
			getter: mapGetter{
				values:  map[string]string{prefix + "HTTP_MAX_INFLIGHT": "256"},
				entries: []string{prefix + "HTTP_MAX_INFLIGHT=256"},
			},
			field: prefix + "HTTP_MAX_INFLIGHT",
		},
		{
			name: "data directory typo",
			getter: mapGetter{
				values:  map[string]string{prefix + "DATA_DIR": "data"},
				entries: []string{prefix + "DATA_DIR=data"},
			},
			field: prefix + "DATA_DIR",
		},
		{
			name: "wrong case",
			getter: mapGetter{
				values:  map[string]string{"stackplore_environment": "test"},
				entries: []string{"stackplore_environment=test"},
			},
			field: "stackplore_environment",
		},
		{
			name: "duplicate",
			getter: mapGetter{
				values:  map[string]string{environmentKey: "test"},
				entries: []string{environmentKey + "=test", environmentKey + "=production"},
			},
			field: environmentKey,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(test.getter)
			assertFieldError(t, err, test.field)
		})
	}
}

func TestErrorsNeverContainValues(t *testing.T) {
	t.Parallel()

	const marker = "do-not-disclose-9f28b74a"
	tests := []mapGetter{
		getter(map[string]string{environmentKey: marker}),
		getter(map[string]string{publicOriginKey: marker}),
		getter(map[string]string{trustedProxyPrefixesKey: marker}),
		getter(map[string]string{logLevelKey: marker}),
		getter(map[string]string{dataDirectoryKey: marker + "\x00"}),
		getter(map[string]string{databaseBusyTimeoutKey: marker}),
		getter(map[string]string{setupBootstrapProofKey: marker}),
		getter(map[string]string{readTimeoutKey: marker}),
		getter(map[string]string{maxHeaderBytesKey: marker}),
		getter(map[string]string{maximumResponseBytesKey: marker}),
		getter(map[string]string{maxInFlightKey: marker}),
		getter(map[string]string{ratePerSecondKey: marker}),
		{
			values:  map[string]string{prefix + "UNKNOWN": marker},
			entries: []string{prefix + "UNKNOWN=" + marker},
		},
	}
	for index, source := range tests {
		_, err := Load(source)
		if err == nil {
			t.Fatalf("case %d: Load() error = nil; want error", index)
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("case %d: error disclosed configuration value", index)
		}
	}
}

func TestLoadReturnsIndependentPrefixSlices(t *testing.T) {
	t.Parallel()

	first, err := Load(getter(nil))
	if err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	second, err := Load(getter(nil))
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	first.MetricsAllowedPrefixes[0] = netip.MustParsePrefix("10.0.0.0/8")
	if second.MetricsAllowedPrefixes[0] != netip.MustParsePrefix("127.0.0.0/8") {
		t.Fatal("Load() shared mutable prefix storage")
	}
}

func TestLoadRejectsNilGetter(t *testing.T) {
	t.Parallel()

	_, err := Load(nil)
	assertFieldError(t, err, "environment")
}

func assertFieldError(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatal("Load() error = nil; want error")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("errors.Is(error, ErrInvalid) = false; error = %v", err)
	}
	var fieldError *FieldError
	if !errors.As(err, &fieldError) {
		t.Fatalf("error type = %T; want *FieldError", err)
	}
	if fieldError.Field != field {
		t.Errorf("error field = %q; want %q", fieldError.Field, field)
	}
}
