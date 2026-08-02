// Package config loads and validates process configuration.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	prefix = "STACKPLORE_"

	environmentKey            = prefix + "ENVIRONMENT"
	httpAddressKey            = prefix + "HTTP_ADDRESS"
	publicOriginKey           = prefix + "PUBLIC_ORIGIN"
	trustedProxyPrefixesKey   = prefix + "TRUSTED_PROXY_PREFIXES"
	metricsAllowedPrefixesKey = prefix + "METRICS_ALLOWED_PREFIXES"
	logLevelKey               = prefix + "LOG_LEVEL"
	logFormatKey              = prefix + "LOG_FORMAT"
	readHeaderTimeoutKey      = prefix + "HTTP_READ_HEADER_TIMEOUT"
	readTimeoutKey            = prefix + "HTTP_READ_TIMEOUT"
	writeTimeoutKey           = prefix + "HTTP_WRITE_TIMEOUT"
	idleTimeoutKey            = prefix + "HTTP_IDLE_TIMEOUT"
	requestTimeoutKey         = prefix + "HTTP_REQUEST_TIMEOUT"
	shutdownTimeoutKey        = prefix + "HTTP_SHUTDOWN_TIMEOUT"
	maxHeaderBytesKey         = prefix + "HTTP_MAX_HEADER_BYTES"
	maxRequestBodyBytesKey    = prefix + "HTTP_MAX_REQUEST_BODY_BYTES"
	maximumResponseBytesKey   = prefix + "HTTP_MAX_RESPONSE_BYTES"
	maxConnectionsKey         = prefix + "HTTP_MAX_CONNECTIONS"
	maxInFlightKey            = prefix + "HTTP_MAX_IN_FLIGHT"
	ratePerSecondKey          = prefix + "RATE_LIMIT_PER_SECOND"
	rateBurstKey              = prefix + "RATE_LIMIT_BURST"
	rateMaxClientsKey         = prefix + "RATE_LIMIT_MAX_CLIENTS"
	dataDirectoryKey          = prefix + "DATA_DIRECTORY"
	databaseBusyTimeoutKey    = prefix + "DATABASE_BUSY_TIMEOUT"
	setupBootstrapProofKey    = prefix + "SETUP_BOOTSTRAP_PROOF"
	setupBootstrapExpiryKey   = prefix + "SETUP_BOOTSTRAP_EXPIRES_AT"

	// This variable belongs to the browser development process and is allowed
	// in the shared process environment without becoming server configuration.
	developmentAPIOriginKey = prefix + "DEV_API_ORIGIN"

	defaultHTTPAddress          = "127.0.0.1:8080"
	defaultReadHeaderTimeout    = 5 * time.Second
	defaultReadTimeout          = 15 * time.Second
	defaultWriteTimeout         = 30 * time.Second
	defaultIdleTimeout          = 60 * time.Second
	defaultRequestTimeout       = 15 * time.Second
	defaultShutdownTimeout      = 15 * time.Second
	defaultMaxHeaderBytes       = 32 * 1024
	defaultMaxRequestBodyBytes  = 256 * 1024
	defaultMaximumResponseBytes = 16 * 1024 * 1024
	defaultMaxConnections       = 512
	defaultMaxInFlight          = 256
	defaultRatePerSecond        = 20.0
	defaultRateBurst            = 40
	defaultRateMaxClients       = 4096
	defaultDataDirectory        = "data"
	defaultDatabaseBusyTimeout  = 5 * time.Second
	maximumBootstrapLifetime    = time.Hour
)

const (
	minimumReadHeaderTimeout   = time.Second
	maximumReadHeaderTimeout   = 30 * time.Second
	minimumReadTimeout         = time.Second
	maximumReadTimeout         = 2 * time.Minute
	minimumWriteTimeout        = time.Second
	maximumWriteTimeout        = 5 * time.Minute
	minimumIdleTimeout         = time.Second
	maximumIdleTimeout         = 10 * time.Minute
	minimumRequestTimeout      = 100 * time.Millisecond
	maximumRequestTimeout      = 2 * time.Minute
	minimumShutdownTimeout     = time.Second
	maximumShutdownTimeout     = 2 * time.Minute
	minimumMaxHeaderBytes      = 4 * 1024
	maximumMaxHeaderBytes      = 1024 * 1024
	minimumRequestBodyBytes    = 1024
	maximumRequestBodyBytes    = 16 * 1024 * 1024
	minimumResponseBytes       = 64 * 1024
	maximumResponseBytes       = 64 * 1024 * 1024
	minimumMaxConnections      = 1
	maximumMaxConnections      = 16_384
	minimumMaxInFlight         = 1
	maximumMaxInFlight         = 4096
	minimumRatePerSecond       = 0.1
	maximumRatePerSecond       = 10_000.0
	minimumRateBurst           = 1
	maximumRateBurst           = 10_000
	minimumRateMaxClients      = 1
	maximumRateMaxClients      = 65_536
	maximumDataDirectoryBytes  = 4096
	minimumDatabaseBusyTimeout = 100 * time.Millisecond
	maximumDatabaseBusyTimeout = 30 * time.Second
)

// Environment selects runtime validation rules.
type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentProduction  Environment = "production"
	EnvironmentTest        Environment = "test"
)

// LogFormat selects structured JSON or human-readable text output.
type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

// Config is the fully validated server configuration.
type Config struct {
	Environment Environment

	HTTPAddress            string
	PublicOrigin           url.URL
	TrustedProxyPrefixes   []netip.Prefix
	MetricsAllowedPrefixes []netip.Prefix

	LogLevel  slog.Level
	LogFormat LogFormat

	ReadHeaderTimeout    time.Duration
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	IdleTimeout          time.Duration
	RequestTimeout       time.Duration
	ShutdownTimeout      time.Duration
	MaxHeaderBytes       int
	MaxRequestBodyBytes  int64
	MaximumResponseBytes int64
	MaxConnections       int
	MaxInFlight          int

	RatePerSecond  float64
	RateBurst      int
	RateMaxClients int

	DataDirectory       string
	DatabaseBusyTimeout time.Duration

	SetupBootstrapProof     Secret
	SetupBootstrapExpiresAt time.Time
}

// Secret keeps one fixed-size process secret out of formatted configuration.
type Secret struct {
	value [32]byte
	set   bool
}

// Present reports whether a secret was configured.
func (secret Secret) Present() bool { return secret.set }

// Bytes returns an independent copy for a short-lived consumer.
func (secret Secret) Bytes() []byte {
	if !secret.set {
		return nil
	}
	result := make([]byte, len(secret.value))
	copy(result, secret.value[:])
	return result
}

// Format prevents configuration formatting from disclosing secret bytes.
func (Secret) Format(state fmt.State, _ rune) { _, _ = state.Write([]byte("[redacted]")) }

// Getter makes environment access explicit and replaceable in tests.
type Getter interface {
	LookupEnv(string) (string, bool)
	Environ() []string
}

// OSGetter reads the current process environment.
type OSGetter struct{}

// LookupEnv implements Getter.
func (OSGetter) LookupEnv(key string) (string, bool) { return os.LookupEnv(key) }

// Environ implements Getter.
func (OSGetter) Environ() []string { return os.Environ() }

// ErrInvalid marks configuration that cannot safely start the service.
var ErrInvalid = errors.New("invalid configuration")

// FieldError identifies a bad field without retaining its value.
type FieldError struct {
	Field  string
	Reason string
}

// Error implements error.
func (e *FieldError) Error() string {
	return fmt.Sprintf("configuration field %s %s", e.Field, e.Reason)
}

// Unwrap makes errors.Is(err, ErrInvalid) useful to callers.
func (e *FieldError) Unwrap() error { return ErrInvalid }

// Load reads and validates configuration from getter.
func Load(getter Getter) (Config, error) {
	if getter == nil {
		return Config{}, invalid("environment", "is unavailable")
	}
	if err := validateEnvironmentKeys(getter.Environ()); err != nil {
		return Config{}, err
	}
	if err := validateDevelopmentAPIOrigin(getter); err != nil {
		return Config{}, err
	}

	result := defaultConfig()
	var err error

	if result.Environment, err = loadEnvironment(getter, result.Environment); err != nil {
		return Config{}, err
	}
	if result.HTTPAddress, err = loadAddress(getter, result.HTTPAddress); err != nil {
		return Config{}, err
	}
	if result.PublicOrigin, err = loadPublicOrigin(getter, result.HTTPAddress); err != nil {
		return Config{}, err
	}
	if result.SetupBootstrapProof, result.SetupBootstrapExpiresAt, err = loadSetupBootstrap(getter); err != nil {
		return Config{}, err
	}
	if result.TrustedProxyPrefixes, err = loadPrefixes(getter, trustedProxyPrefixesKey, nil); err != nil {
		return Config{}, err
	}
	if result.MetricsAllowedPrefixes, err = loadPrefixes(getter, metricsAllowedPrefixesKey, result.MetricsAllowedPrefixes); err != nil {
		return Config{}, err
	}
	if result.LogLevel, err = loadLogLevel(getter, result.LogLevel); err != nil {
		return Config{}, err
	}
	if result.LogFormat, err = loadLogFormat(getter, result.LogFormat); err != nil {
		return Config{}, err
	}
	if result.DataDirectory, err = loadDataDirectory(getter, result.DataDirectory); err != nil {
		return Config{}, err
	}

	durationFields := []struct {
		key     string
		target  *time.Duration
		minimum time.Duration
		maximum time.Duration
	}{
		{readHeaderTimeoutKey, &result.ReadHeaderTimeout, minimumReadHeaderTimeout, maximumReadHeaderTimeout},
		{readTimeoutKey, &result.ReadTimeout, minimumReadTimeout, maximumReadTimeout},
		{writeTimeoutKey, &result.WriteTimeout, minimumWriteTimeout, maximumWriteTimeout},
		{idleTimeoutKey, &result.IdleTimeout, minimumIdleTimeout, maximumIdleTimeout},
		{requestTimeoutKey, &result.RequestTimeout, minimumRequestTimeout, maximumRequestTimeout},
		{shutdownTimeoutKey, &result.ShutdownTimeout, minimumShutdownTimeout, maximumShutdownTimeout},
		{databaseBusyTimeoutKey, &result.DatabaseBusyTimeout, minimumDatabaseBusyTimeout, maximumDatabaseBusyTimeout},
	}
	for _, field := range durationFields {
		*field.target, err = loadDuration(getter, field.key, *field.target, field.minimum, field.maximum)
		if err != nil {
			return Config{}, err
		}
	}

	result.MaxHeaderBytes, err = loadInt(getter, maxHeaderBytesKey, result.MaxHeaderBytes, minimumMaxHeaderBytes, maximumMaxHeaderBytes)
	if err != nil {
		return Config{}, err
	}
	result.MaxRequestBodyBytes, err = loadInt64(getter, maxRequestBodyBytesKey, result.MaxRequestBodyBytes, minimumRequestBodyBytes, maximumRequestBodyBytes)
	if err != nil {
		return Config{}, err
	}
	result.MaximumResponseBytes, err = loadInt64(getter, maximumResponseBytesKey, result.MaximumResponseBytes, minimumResponseBytes, maximumResponseBytes)
	if err != nil {
		return Config{}, err
	}
	result.MaxConnections, err = loadInt(getter, maxConnectionsKey, result.MaxConnections, minimumMaxConnections, maximumMaxConnections)
	if err != nil {
		return Config{}, err
	}
	result.MaxInFlight, err = loadInt(getter, maxInFlightKey, result.MaxInFlight, minimumMaxInFlight, maximumMaxInFlight)
	if err != nil {
		return Config{}, err
	}
	result.RatePerSecond, err = loadFloat(getter, ratePerSecondKey, result.RatePerSecond, minimumRatePerSecond, maximumRatePerSecond)
	if err != nil {
		return Config{}, err
	}
	result.RateBurst, err = loadInt(getter, rateBurstKey, result.RateBurst, minimumRateBurst, maximumRateBurst)
	if err != nil {
		return Config{}, err
	}
	result.RateMaxClients, err = loadInt(getter, rateMaxClientsKey, result.RateMaxClients, minimumRateMaxClients, maximumRateMaxClients)
	if err != nil {
		return Config{}, err
	}

	if err := Validate(result); err != nil {
		return Config{}, err
	}

	return result, nil
}

func validateDevelopmentAPIOrigin(getter Getter) error {
	value, present, err := readValue(getter, developmentAPIOriginKey)
	if err != nil || !present {
		return err
	}
	if _, err := parsePublicOrigin(value); err != nil {
		return invalid(developmentAPIOriginKey, "is invalid")
	}
	return nil
}

// Validate checks a typed configuration before it reaches runtime wiring.
func Validate(result Config) error {
	switch result.Environment {
	case EnvironmentDevelopment, EnvironmentProduction, EnvironmentTest:
	default:
		return invalid(environmentKey, "is invalid")
	}
	if _, _, err := parseAddress(result.HTTPAddress); err != nil {
		return invalid(httpAddressKey, "is invalid")
	}
	parsedOrigin, err := parsePublicOrigin(result.PublicOrigin.String())
	if err != nil || parsedOrigin != result.PublicOrigin {
		return invalid(publicOriginKey, "is invalid")
	}
	if err := validatePrefixSet(trustedProxyPrefixesKey, result.TrustedProxyPrefixes); err != nil {
		return err
	}
	if err := validatePrefixSet(metricsAllowedPrefixesKey, result.MetricsAllowedPrefixes); err != nil {
		return err
	}
	if !validLevel(result.LogLevel) {
		return invalid(logLevelKey, "is invalid")
	}
	if result.LogFormat != LogFormatJSON && result.LogFormat != LogFormatText {
		return invalid(logFormatKey, "is invalid")
	}
	if err := validateDataDirectory(result.DataDirectory, result.Environment); err != nil {
		return err
	}

	durations := []struct {
		key     string
		value   time.Duration
		minimum time.Duration
		maximum time.Duration
	}{
		{readHeaderTimeoutKey, result.ReadHeaderTimeout, minimumReadHeaderTimeout, maximumReadHeaderTimeout},
		{readTimeoutKey, result.ReadTimeout, minimumReadTimeout, maximumReadTimeout},
		{writeTimeoutKey, result.WriteTimeout, minimumWriteTimeout, maximumWriteTimeout},
		{idleTimeoutKey, result.IdleTimeout, minimumIdleTimeout, maximumIdleTimeout},
		{requestTimeoutKey, result.RequestTimeout, minimumRequestTimeout, maximumRequestTimeout},
		{shutdownTimeoutKey, result.ShutdownTimeout, minimumShutdownTimeout, maximumShutdownTimeout},
		{databaseBusyTimeoutKey, result.DatabaseBusyTimeout, minimumDatabaseBusyTimeout, maximumDatabaseBusyTimeout},
	}
	for _, field := range durations {
		if field.value < field.minimum || field.value > field.maximum {
			return invalid(field.key, "is outside the allowed range")
		}
	}
	if result.DatabaseBusyTimeout%time.Millisecond != 0 {
		return invalid(databaseBusyTimeoutKey, "must use whole milliseconds")
	}
	if result.MaxHeaderBytes < minimumMaxHeaderBytes || result.MaxHeaderBytes > maximumMaxHeaderBytes {
		return invalid(maxHeaderBytesKey, "is outside the allowed range")
	}
	if result.MaxRequestBodyBytes < minimumRequestBodyBytes || result.MaxRequestBodyBytes > maximumRequestBodyBytes {
		return invalid(maxRequestBodyBytesKey, "is outside the allowed range")
	}
	if result.MaximumResponseBytes < minimumResponseBytes || result.MaximumResponseBytes > maximumResponseBytes {
		return invalid(maximumResponseBytesKey, "is outside the allowed range")
	}
	if result.MaxConnections < minimumMaxConnections || result.MaxConnections > maximumMaxConnections {
		return invalid(maxConnectionsKey, "is outside the allowed range")
	}
	if result.MaxInFlight < minimumMaxInFlight || result.MaxInFlight > maximumMaxInFlight {
		return invalid(maxInFlightKey, "is outside the allowed range")
	}
	if math.IsNaN(result.RatePerSecond) ||
		math.IsInf(result.RatePerSecond, 0) ||
		result.RatePerSecond < minimumRatePerSecond ||
		result.RatePerSecond > maximumRatePerSecond {
		return invalid(ratePerSecondKey, "is outside the allowed range")
	}
	if result.RateBurst < minimumRateBurst || result.RateBurst > maximumRateBurst {
		return invalid(rateBurstKey, "is outside the allowed range")
	}
	if result.RateMaxClients < minimumRateMaxClients || result.RateMaxClients > maximumRateMaxClients {
		return invalid(rateMaxClientsKey, "is outside the allowed range")
	}
	if result.ReadHeaderTimeout > result.ReadTimeout {
		return invalid(readHeaderTimeoutKey, "must not exceed the read timeout")
	}
	if result.RequestTimeout >= result.WriteTimeout {
		return invalid(requestTimeoutKey, "must be shorter than the write timeout")
	}
	if err := validateProduction(result); err != nil {
		return err
	}
	return validateSetupBootstrap(result)
}

func defaultConfig() Config {
	publicOrigin, err := parsePublicOrigin("http://" + defaultHTTPAddress)
	if err != nil {
		panic("invalid built-in public origin")
	}

	return Config{
		Environment:            EnvironmentDevelopment,
		HTTPAddress:            defaultHTTPAddress,
		PublicOrigin:           publicOrigin,
		MetricsAllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")},
		LogLevel:               slog.LevelInfo,
		LogFormat:              LogFormatJSON,
		ReadHeaderTimeout:      defaultReadHeaderTimeout,
		ReadTimeout:            defaultReadTimeout,
		WriteTimeout:           defaultWriteTimeout,
		IdleTimeout:            defaultIdleTimeout,
		RequestTimeout:         defaultRequestTimeout,
		ShutdownTimeout:        defaultShutdownTimeout,
		MaxHeaderBytes:         defaultMaxHeaderBytes,
		MaxRequestBodyBytes:    defaultMaxRequestBodyBytes,
		MaximumResponseBytes:   defaultMaximumResponseBytes,
		MaxConnections:         defaultMaxConnections,
		MaxInFlight:            defaultMaxInFlight,
		RatePerSecond:          defaultRatePerSecond,
		RateBurst:              defaultRateBurst,
		RateMaxClients:         defaultRateMaxClients,
		DataDirectory:          defaultDataDirectory,
		DatabaseBusyTimeout:    defaultDatabaseBusyTimeout,
	}
}

func validateEnvironmentKeys(entries []string) error {
	allowed := map[string]struct{}{
		environmentKey: {}, httpAddressKey: {}, publicOriginKey: {},
		trustedProxyPrefixesKey: {}, metricsAllowedPrefixesKey: {},
		logLevelKey: {}, logFormatKey: {}, readHeaderTimeoutKey: {},
		readTimeoutKey: {}, writeTimeoutKey: {}, idleTimeoutKey: {},
		requestTimeoutKey: {}, shutdownTimeoutKey: {}, maxHeaderBytesKey: {},
		maxRequestBodyBytesKey: {}, maximumResponseBytesKey: {}, maxConnectionsKey: {}, maxInFlightKey: {}, ratePerSecondKey: {}, rateBurstKey: {},
		rateMaxClientsKey: {}, dataDirectoryKey: {}, databaseBusyTimeoutKey: {}, setupBootstrapProofKey: {}, setupBootstrapExpiryKey: {}, developmentAPIOriginKey: {},
	}
	seen := make(map[string]struct{}, len(allowed))
	for _, entry := range entries {
		name, _, found := strings.Cut(entry, "=")
		if !found || !strings.HasPrefix(strings.ToUpper(name), prefix) {
			continue
		}
		if _, ok := allowed[name]; !ok {
			return invalid(name, "is unknown")
		}
		if _, ok := seen[name]; ok {
			return invalid(name, "is duplicated")
		}
		seen[name] = struct{}{}
	}
	return nil
}

func loadEnvironment(getter Getter, fallback Environment) (Environment, error) {
	value, present, err := readValue(getter, environmentKey)
	if err != nil || !present {
		return fallback, err
	}
	switch Environment(value) {
	case EnvironmentDevelopment, EnvironmentProduction, EnvironmentTest:
		return Environment(value), nil
	default:
		return "", invalid(environmentKey, "is invalid")
	}
}

func loadAddress(getter Getter, fallback string) (string, error) {
	value, present, err := readValue(getter, httpAddressKey)
	if err != nil || !present {
		return fallback, err
	}
	if _, _, err := parseAddress(value); err != nil {
		return "", invalid(httpAddressKey, "is invalid")
	}
	return value, nil
}

func loadPublicOrigin(getter Getter, address string) (url.URL, error) {
	value, present, err := readValue(getter, publicOriginKey)
	if err != nil {
		return url.URL{}, err
	}
	if !present {
		value = "http://" + address
	}
	result, err := parsePublicOrigin(value)
	if err != nil {
		return url.URL{}, invalid(publicOriginKey, "is invalid")
	}
	return result, nil
}

func loadSetupBootstrap(getter Getter) (Secret, time.Time, error) {
	proofText, proofPresent, err := readValue(getter, setupBootstrapProofKey)
	if err != nil {
		return Secret{}, time.Time{}, err
	}
	expiryText, expiryPresent, err := readValue(getter, setupBootstrapExpiryKey)
	if err != nil {
		return Secret{}, time.Time{}, err
	}
	if !proofPresent && !expiryPresent {
		return Secret{}, time.Time{}, nil
	}
	if !proofPresent {
		return Secret{}, time.Time{}, invalid(setupBootstrapProofKey, "is required with bootstrap expiry")
	}
	if !expiryPresent {
		return Secret{}, time.Time{}, invalid(setupBootstrapExpiryKey, "is required with bootstrap proof")
	}
	decoded, decodeErr := base64.RawURLEncoding.Strict().DecodeString(proofText)
	if decodeErr != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != proofText {
		return Secret{}, time.Time{}, invalid(setupBootstrapProofKey, "is invalid")
	}
	expiresAt, parseErr := time.Parse(time.RFC3339, expiryText)
	now := time.Now().UTC()
	if parseErr != nil || expiresAt.Location() != time.UTC || expiresAt.Format(time.RFC3339) != expiryText || expiresAt.After(now.Add(maximumBootstrapLifetime)) {
		return Secret{}, time.Time{}, invalid(setupBootstrapExpiryKey, "must be a canonical short-lived UTC time")
	}
	var result Secret
	copy(result.value[:], decoded)
	result.set = true
	return result, expiresAt, nil
}

func validateSetupBootstrap(result Config) error {
	if !result.SetupBootstrapProof.Present() {
		if !result.SetupBootstrapExpiresAt.IsZero() {
			return invalid(setupBootstrapProofKey, "is required with bootstrap expiry")
		}
		return nil
	}
	if result.SetupBootstrapExpiresAt.IsZero() || result.SetupBootstrapExpiresAt.Location() != time.UTC || result.SetupBootstrapExpiresAt.Nanosecond() != 0 {
		return invalid(setupBootstrapExpiryKey, "is invalid")
	}
	return nil
}

// SetupBootstrapRequired reports whether first-run setup needs operator proof.
func SetupBootstrapRequired(result Config) bool {
	return setupExposureRequired(result) || result.SetupBootstrapProof.Present()
}

func setupExposureRequired(result Config) bool {
	listenerHost, _, err := parseAddress(result.HTTPAddress)
	if err != nil || result.PublicOrigin.Hostname() == "" {
		return true
	}
	return !isLoopbackHost(listenerHost) || !isLoopbackHost(result.PublicOrigin.Hostname())
}

func loadPrefixes(getter Getter, key string, fallback []netip.Prefix) ([]netip.Prefix, error) {
	value, present, err := readValue(getter, key)
	if err != nil || !present {
		return append([]netip.Prefix(nil), fallback...), err
	}

	parts := strings.Split(value, ",")
	result := make([]netip.Prefix, 0, len(parts))
	seen := make(map[netip.Prefix]struct{}, len(parts))
	for _, part := range parts {
		if part == "" || part != strings.TrimSpace(part) {
			return nil, invalid(key, "is invalid")
		}
		parsed, parseErr := netip.ParsePrefix(part)
		if parseErr != nil ||
			parsed.Addr().Zone() != "" ||
			parsed.Addr().Is4In6() ||
			parsed.Bits() == 0 ||
			parsed != parsed.Masked() ||
			parsed.String() != part {
			return nil, invalid(key, "is invalid")
		}
		if _, exists := seen[parsed]; exists {
			return nil, invalid(key, "contains a duplicate")
		}
		seen[parsed] = struct{}{}
		result = append(result, parsed)
	}
	return result, nil
}

func validatePrefixSet(key string, prefixes []netip.Prefix) error {
	seen := make(map[netip.Prefix]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		if !prefix.IsValid() ||
			prefix.Addr().Zone() != "" ||
			prefix.Addr().Is4In6() ||
			prefix.Bits() == 0 ||
			prefix != prefix.Masked() {
			return invalid(key, "is invalid")
		}
		if _, exists := seen[prefix]; exists {
			return invalid(key, "contains a duplicate")
		}
		seen[prefix] = struct{}{}
	}
	return nil
}

func loadLogLevel(getter Getter, fallback slog.Level) (slog.Level, error) {
	value, present, err := readValue(getter, logLevelKey)
	if err != nil || !present {
		return fallback, err
	}
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, invalid(logLevelKey, "is invalid")
	}
}

func validLevel(level slog.Level) bool {
	switch level {
	case slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError:
		return true
	default:
		return false
	}
}

func loadLogFormat(getter Getter, fallback LogFormat) (LogFormat, error) {
	value, present, err := readValue(getter, logFormatKey)
	if err != nil || !present {
		return fallback, err
	}
	switch LogFormat(value) {
	case LogFormatJSON, LogFormatText:
		return LogFormat(value), nil
	default:
		return "", invalid(logFormatKey, "is invalid")
	}
}

func loadDataDirectory(getter Getter, fallback string) (string, error) {
	value, present, err := readValue(getter, dataDirectoryKey)
	if err != nil || !present {
		return fallback, err
	}
	if err := validateDataDirectory(value, EnvironmentDevelopment); err != nil {
		return "", err
	}
	return value, nil
}

func validateDataDirectory(value string, environment Environment) error {
	if value == "" ||
		len(value) > maximumDataDirectoryBytes ||
		!utf8.ValidString(value) ||
		filepath.Clean(value) != value ||
		value == "." ||
		isRootDirectory(value) ||
		isUNCOrDevicePath(value) ||
		hasWindowsDeviceName(value) {
		return invalid(dataDirectoryKey, "is invalid")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return invalid(dataDirectoryKey, "is invalid")
		}
	}

	absolute := filepath.IsAbs(value)
	if !absolute && !filepath.IsLocal(value) {
		return invalid(dataDirectoryKey, "is invalid")
	}
	if environment == EnvironmentProduction && !absolute {
		return invalid(dataDirectoryKey, "must be absolute in production")
	}
	return nil
}

func isRootDirectory(value string) bool {
	if len(value) == 3 && value[1] == ':' && (value[2] == '/' || value[2] == '\\') {
		first := value[0]
		return first >= 'A' && first <= 'Z' || first >= 'a' && first <= 'z'
	}
	volume := filepath.VolumeName(value)
	remainder := strings.TrimPrefix(value, volume)
	if remainder == "" {
		return filepath.IsAbs(value)
	}
	for _, character := range remainder {
		if character != '/' && character != '\\' {
			return false
		}
	}
	return true
}

func isUNCOrDevicePath(value string) bool {
	upper := strings.ToUpper(value)
	return strings.HasPrefix(value, `\\`) ||
		strings.HasPrefix(value, "//") ||
		strings.HasPrefix(upper, `\??\`) ||
		strings.HasPrefix(upper, `\DEVICE\`) ||
		strings.HasPrefix(upper, `\GLOBALROOT\`)
}

func hasWindowsDeviceName(value string) bool {
	components := strings.FieldsFunc(value, func(character rune) bool {
		return character == '/' || character == '\\'
	})
	for _, component := range components {
		component = strings.TrimRight(component, ". ")
		base, _, _ := strings.Cut(component, ".")
		upper := strings.ToUpper(base)
		switch upper {
		case "CON", "PRN", "AUX", "NUL", "CLOCK$", "CONIN$", "CONOUT$":
			return true
		}
		characters := []rune(upper)
		if len(characters) == 4 &&
			(string(characters[:3]) == "COM" || string(characters[:3]) == "LPT") &&
			isWindowsDeviceDigit(characters[3]) {
			return true
		}
	}
	return false
}

func isWindowsDeviceDigit(character rune) bool {
	return character >= '1' && character <= '9' ||
		character == '\u00b9' ||
		character == '\u00b2' ||
		character == '\u00b3'
}

func loadDuration(getter Getter, key string, fallback, minimum, maximum time.Duration) (time.Duration, error) {
	value, present, err := readValue(getter, key)
	if err != nil || !present {
		return fallback, err
	}
	parsed, parseErr := time.ParseDuration(value)
	if parseErr != nil || parsed < minimum || parsed > maximum {
		return 0, invalid(key, "is outside the allowed range")
	}
	return parsed, nil
}

func loadInt(getter Getter, key string, fallback, minimum, maximum int) (int, error) {
	value, present, err := readValue(getter, key)
	if err != nil || !present {
		return fallback, err
	}
	parsed, parseErr := strconv.ParseInt(value, 10, 32)
	if parseErr != nil || int(parsed) < minimum || int(parsed) > maximum {
		return 0, invalid(key, "is outside the allowed range")
	}
	return int(parsed), nil
}

func loadInt64(getter Getter, key string, fallback int64, minimum, maximum int64) (int64, error) {
	value, present, err := readValue(getter, key)
	if err != nil || !present {
		return fallback, err
	}
	parsed, parseErr := strconv.ParseInt(value, 10, 64)
	if parseErr != nil || parsed < minimum || parsed > maximum {
		return 0, invalid(key, "is outside the allowed range")
	}
	return parsed, nil
}

func loadFloat(getter Getter, key string, fallback, minimum, maximum float64) (float64, error) {
	value, present, err := readValue(getter, key)
	if err != nil || !present {
		return fallback, err
	}
	parsed, parseErr := strconv.ParseFloat(value, 64)
	if parseErr != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < minimum || parsed > maximum {
		return 0, invalid(key, "is outside the allowed range")
	}
	return parsed, nil
}

func readValue(getter Getter, key string) (string, bool, error) {
	value, present := getter.LookupEnv(key)
	if !present {
		return "", false, nil
	}
	if value == "" || value != strings.TrimSpace(value) {
		return "", false, invalid(key, "is blank or not canonical")
	}
	return value, true, nil
}

func parseAddress(value string) (string, uint16, error) {
	host, portText, err := net.SplitHostPort(value)
	if err != nil || !validHost(host) || !canonicalPort(portText) {
		return "", 0, ErrInvalid
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return "", 0, ErrInvalid
	}
	return host, uint16(port), nil
}

func parsePublicOrigin(value string) (url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" || parsed.RawPath != "" || parsed.ForceQuery || parsed.RawQuery != "" || parsed.Fragment != "" {
		return url.URL{}, ErrInvalid
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return url.URL{}, ErrInvalid
	}
	host := parsed.Hostname()
	if !validHost(host) {
		return url.URL{}, ErrInvalid
	}
	port := parsed.Port()
	if port != "" {
		if !canonicalPort(port) {
			return url.URL{}, ErrInvalid
		}
		number, parseErr := strconv.ParseUint(port, 10, 16)
		if parseErr != nil || number == 0 || parsed.Scheme == "http" && number == 80 || parsed.Scheme == "https" && number == 443 {
			return url.URL{}, ErrInvalid
		}
	}

	canonicalHost := host
	if strings.Contains(host, ":") {
		canonicalHost = "[" + host + "]"
	}
	if port != "" {
		canonicalHost += ":" + port
	}
	if parsed.Host != canonicalHost || parsed.String() != value {
		return url.URL{}, ErrInvalid
	}
	return *parsed, nil
}

func validHost(host string) bool {
	if host == "" || host != strings.ToLower(host) {
		return false
	}
	if address, err := netip.ParseAddr(host); err == nil {
		return address.Zone() == "" && address.String() == host
	}
	if len(host) > 253 || strings.HasSuffix(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if character < 'a' || character > 'z' {
				if character < '0' || character > '9' {
					if character != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}

func canonicalPort(value string) bool {
	if value == "" || len(value) > 5 || len(value) > 1 && value[0] == '0' {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validateProduction(result Config) error {
	if result.Environment != EnvironmentProduction {
		return nil
	}
	listenerHost, _, err := parseAddress(result.HTTPAddress)
	if err != nil {
		return invalid(httpAddressKey, "is invalid")
	}
	remoteListener := !isLoopbackHost(listenerHost)
	remoteOrigin := !isLoopbackHost(result.PublicOrigin.Hostname())
	if !remoteListener && !remoteOrigin {
		return nil
	}
	if result.PublicOrigin.Scheme != "https" {
		return invalid(publicOriginKey, "must use HTTPS for non-loopback production")
	}
	if len(result.TrustedProxyPrefixes) == 0 {
		return invalid(trustedProxyPrefixesKey, "is required for non-loopback production")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}

func invalid(field, reason string) error {
	return &FieldError{Field: field, Reason: reason}
}
