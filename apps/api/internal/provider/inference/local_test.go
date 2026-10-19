package inference

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

const fixtureAlias = "sha256-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const fixtureBuild = "b11729 fixture"
const fixtureTemplate = "fixture role and content template"

type protocolFixture struct {
	completion atomic.Int32
	changed    atomic.Bool
	mutate     func(http.ResponseWriter, *http.Request) bool
}

func (fixture *protocolFixture) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if fixture.mutate != nil && fixture.mutate(response, request) {
		return
	}
	response.Header().Set("Content-Type", "application/json")
	switch request.URL.Path {
	case "/v1/models":
		alias := fixtureAlias
		if fixture.changed.Load() {
			alias = "sha256-" + strings.Repeat("b", 64)
		}
		_ = json.NewEncoder(response).Encode(map[string]any{"data": []map[string]string{{"id": alias}}, "object": "list"})
	case "/props":
		_ = json.NewEncoder(response).Encode(map[string]any{"build_info": fixtureBuild, "chat_template": fixtureTemplate, "model_path": "private unused fixture path", "extra": true})
	case "/apply-template":
		var body struct {
			Messages []message `json:"messages"`
		}
		if json.NewDecoder(request.Body).Decode(&body) != nil || len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[1].Role != "user" {
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(response).Encode(map[string]string{"prompt": "exact formatted fixture prompt"})
	case "/tokenize":
		var body struct {
			Content      string `json:"content"`
			AddSpecial   bool   `json:"add_special"`
			ParseSpecial bool   `json:"parse_special"`
			WithPieces   bool   `json:"with_pieces"`
		}
		if json.NewDecoder(request.Body).Decode(&body) != nil || body.Content != "exact formatted fixture prompt" || !body.AddSpecial || !body.ParseSpecial || body.WithPieces {
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = response.Write([]byte(`{"tokens":[1,2,3]}`))
	case "/completion":
		fixture.completion.Add(1)
		var body struct {
			Prompt         []int32         `json:"prompt"`
			Schema         json.RawMessage `json:"json_schema"`
			Predict        int             `json:"n_predict"`
			Temperature    float64         `json:"temperature"`
			Stream         bool            `json:"stream"`
			Cache          bool            `json:"cache_prompt"`
			ResponseFields []string        `json:"response_fields"`
		}
		if json.NewDecoder(request.Body).Decode(&body) != nil || len(body.Prompt) != 3 || body.Prompt[0] != 1 || body.Predict < 1 || body.Predict > 4096 || body.Stream || body.Cache || body.Temperature != 0 || !json.Valid(body.Schema) || len(body.ResponseFields) != 6 {
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(response).Encode(map[string]any{"content": `{"sections":[{"kind":"gaps","text":"Insufficient evidence.","citations":[]}]}`, "tokens_predicted": 20, "tokens_evaluated": 3, "truncated": false, "stop_type": "eos", "model": fixtureAlias})
	default:
		response.WriteHeader(http.StatusNotFound)
	}
}

func fixtureProfile(origin string) Profile {
	digest := sha256.Sum256([]byte(fixtureTemplate))
	return Profile{ID: "local_fixture", Origin: origin, ApprovedAddresses: []netip.Addr{netip.MustParseAddr("127.0.0.1")}, ExpectedAlias: fixtureAlias, ExpectedBuild: fixtureBuild, ExpectedTemplateSHA256: hex.EncodeToString(digest[:]), Generation: 1}
}

func fixtureLocal(t *testing.T, fixture *protocolFixture, options Options) (*Local, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(fixture)
	t.Cleanup(server.Close)
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	options.RootCAs = pool
	local, err := NewLocal(fixtureProfile(server.URL), options)
	if err != nil {
		t.Fatal("create local protocol fixture")
	}
	return local, server
}

func testRequest() port.Request {
	return port.Request{Locale: "en", Question: "Explain the evidence.", OutputLimit: 128}
}

func TestLocalProtocolCountsBeforeExactTokenDispatch(t *testing.T) {
	t.Parallel()
	fixture := &protocolFixture{}
	local, _ := fixtureLocal(t, fixture, Options{})
	gateway, err := port.New(local, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := gateway.Prepare(context.Background(), testRequest())
	if err != nil || prepared.InputTokens() != 3 || fixture.completion.Load() != 0 {
		t.Fatal("counting failed or invoked generation")
	}
	result, err := gateway.Dispatch(context.Background(), prepared)
	if err != nil || result.InputTokens != 3 || result.OutputTokens != 20 || len(result.Answer.Sections) != 1 || fixture.completion.Load() != 1 {
		t.Fatal("exact token generation failed")
	}
	if _, err := gateway.Dispatch(context.Background(), prepared); err != port.ErrPrepared || fixture.completion.Load() != 1 {
		t.Fatal("generation repeated")
	}
}

func TestLocalRejectsIdentityChangeBeforeDispatch(t *testing.T) {
	t.Parallel()
	fixture := &protocolFixture{}
	local, _ := fixtureLocal(t, fixture, Options{})
	input, err := local.Count(context.Background(), testRequest())
	if err != nil {
		t.Fatal(err)
	}
	fixture.changed.Store(true)
	if _, err := local.Generate(context.Background(), input, 128); err != port.ErrIdentity || fixture.completion.Load() != 0 {
		t.Fatal("changed model identity reached generation")
	}
}

func TestLocalRejectsMalformedUnknownAndOversizedProtocol(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path, body string
	}{
		{"/v1/models", `{"data":[]}`},
		{"/v1/models", `{"data":[{"id":"wrong"}]}`},
		{"/v1/models", `{"data":[{"id":"wrong"},{"id":"wrong"}]}`},
		{"/props", `{"build_info":"wrong","chat_template":"fixture"}`},
		{"/props", `{"build_info":"b11729 fixture","chat_template":"changed"}`},
		{"/apply-template", `{"prompt":"fixture","extra":true}`},
		{"/apply-template", `{"prompt":""}`},
		{"/apply-template", `{"prompt":` + strconvQuote(strings.Repeat("x", port.MaximumInputBytes+1)) + `}`},
		{"/tokenize", `{"tokens":[1],"extra":true}`},
		{"/tokenize", `{"tokens":[-1]}`},
		{"/tokenize", `{"tokens":[]}`},
		{"/tokenize", `{"tokens":[2147483648]}`},
		{"/tokenize", `{"tokens":[1]} {}`},
		{"/tokenize", strings.Repeat("x", maximumWireBytes+1)},
		{"/completion", `{"content":"{}","tokens_predicted":2,"tokens_evaluated":3,"truncated":false,"stop_type":"eos","model":"wrong"}`},
		{"/completion", `{"content":"{}","tokens_predicted":2,"tokens_evaluated":3,"truncated":false,"stop_type":"eos","model":"` + fixtureAlias + `","extra":true}`},
	}
	for index, test := range cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			fixture := &protocolFixture{mutate: func(response http.ResponseWriter, request *http.Request) bool {
				if request.URL.Path != test.path {
					return false
				}
				_, _ = response.Write([]byte(test.body))
				return true
			}}
			local, _ := fixtureLocal(t, fixture, Options{})
			input, err := local.Count(context.Background(), testRequest())
			if test.path == "/completion" && err == nil {
				_, err = local.Generate(context.Background(), input, 128)
			}
			if err == nil {
				t.Fatal("invalid protocol response accepted")
			}
		})
	}
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestLocalCancellationStopsOneActualDispatch(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	stopped := make(chan struct{})
	var calls atomic.Int32
	fixture := &protocolFixture{mutate: func(response http.ResponseWriter, request *http.Request) bool {
		if request.URL.Path != "/completion" {
			return false
		}
		calls.Add(1)
		_, _ = io.Copy(io.Discard, request.Body)
		close(started)
		select {
		case <-request.Context().Done():
		case <-time.After(3 * time.Second):
		}
		close(stopped)
		return true
	}}
	local, _ := fixtureLocal(t, fixture, Options{Timeout: time.Second})
	gateway, err := port.New(local, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := gateway.Prepare(context.Background(), testRequest())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := gateway.Dispatch(ctx, prepared); done <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("fixture never received inference")
	}
	cancel()
	select {
	case err := <-done:
		if err != port.ErrUncertain {
			t.Fatal("cancel after dispatch did not remain uncertain")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("inference ignored cancellation")
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("receiver context was not cancelled")
	}
	if calls.Load() != 1 {
		t.Fatal("cancelled inference retried")
	}
}

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (resolve resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return resolve(ctx, network, host)
}

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (dial dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dial(ctx, network, address)
}

func TestLocalRechecksAndPinsDNSWithoutFollowingRedirect(t *testing.T) {
	t.Parallel()
	var resolutions, connections atomic.Int32
	fixture := &protocolFixture{}
	local, server := fixtureLocal(t, fixture, Options{})
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	profile := fixtureProfile("https://example.com:" + parsed.Port())
	local, err = NewLocal(profile, Options{RootCAs: local.rootCAs,
		Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
			if resolutions.Add(1) > 4 {
				return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
			}
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		}),
		Dialer: dialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
			connections.Add(1)
			if address != server.Listener.Addr().String() {
				return nil, errors.New("unapproved dial")
			}
			return (&net.Dialer{}).DialContext(ctx, network, address)
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	input, err := local.Count(context.Background(), testRequest())
	if err != nil || resolutions.Load() != 4 || connections.Load() != 4 {
		t.Fatal("count did not pin each checked request")
	}
	if _, err := local.Generate(context.Background(), input, 128); err == nil || connections.Load() != 4 || fixture.completion.Load() != 0 {
		t.Fatal("DNS change reached an unapproved destination")
	}
	var followed atomic.Int32
	other := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed.Add(1) }))
	defer other.Close()
	redirect := &protocolFixture{mutate: func(response http.ResponseWriter, request *http.Request) bool {
		http.Redirect(response, request, other.URL, http.StatusTemporaryRedirect)
		return true
	}}
	blocked, _ := fixtureLocal(t, redirect, Options{})
	if _, err := blocked.Count(context.Background(), testRequest()); err == nil || followed.Load() != 0 {
		t.Fatal("redirect reached a second destination")
	}
}

func TestLocalProfileBlocksRemoteAndUnsafeOrigins(t *testing.T) {
	t.Parallel()
	for _, origin := range []string{"https://fixture.test/path", "https://fixture.test?x=1", "https://fixture.test#x", "https://user@fixture.test", "ftp://fixture.test", "http://fixture.test:0", "http://fixture.test:001", "https://fixture.test/%2fpath", "https://fixture.test?"} {
		if _, err := NewLocal(fixtureProfile(origin), Options{}); err == nil {
			t.Fatal("unsafe profile origin accepted")
		}
	}
	for _, value := range []string{"8.8.8.8", "169.254.169.254", "224.0.0.1", "0.0.0.0", "fe80::1", "ff02::1", "::", "::ffff:169.254.169.254", "100.100.100.200", "168.63.129.16", "fd00:ec2::254"} {
		profile := fixtureProfile("https://fixture.test")
		profile.ApprovedAddresses = []netip.Addr{netip.MustParseAddr(value)}
		if _, err := NewLocal(profile, Options{}); err == nil {
			t.Fatal("nonlocal or special address accepted")
		}
	}
	profile := fixtureProfile("http://fixture.test")
	profile.ApprovedAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.1")}
	if _, err := NewLocal(profile, Options{}); err == nil {
		t.Fatal("private nonloopback HTTP accepted")
	}
	profile.Origin = "https://fixture.test"
	if _, err := NewLocal(profile, Options{}); err != nil {
		t.Fatal("approved private TLS rejected")
	}
	for _, protected := range []any{profile} {
		body, err := json.Marshal(protected)
		if err != nil || string(body) != "{}" || !strings.HasPrefix(fmt.Sprintf("%#v", protected), "[protected ") {
			t.Fatal("profile leaked into formatting")
		}
	}
}

func TestLocalRejectsTLSMismatchAndEncodedBody(t *testing.T) {
	t.Parallel()
	local, server := fixtureLocal(t, &protocolFixture{}, Options{})
	parsed, _ := url.Parse(server.URL)
	wrong, err := NewLocal(fixtureProfile("https://wrong.example.test:"+parsed.Port()), Options{RootCAs: local.rootCAs, Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrong.Count(context.Background(), testRequest()); err == nil {
		t.Fatal("TLS hostname mismatch accepted")
	}
	encoded, _ := fixtureLocal(t, &protocolFixture{mutate: func(response http.ResponseWriter, request *http.Request) bool {
		response.Header().Set("Content-Encoding", "gzip")
		_, _ = response.Write([]byte(`{}`))
		return true
	}}, Options{})
	if _, err := encoded.Count(context.Background(), testRequest()); err == nil {
		t.Fatal("unapproved content encoding accepted")
	}
}

func TestLocalDeadlineDNSBoundsAndNoAmbientProxy(t *testing.T) {
	var proxied atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		proxied.Add(1)
		response.WriteHeader(http.StatusForbidden)
	}))
	defer proxy.Close()
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")
	local, server := fixtureLocal(t, &protocolFixture{}, Options{})
	parsed, _ := url.Parse(server.URL)
	profile := fixtureProfile("https://example.com:" + parsed.Port())
	options := Options{RootCAs: local.rootCAs, Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})}
	checked, err := NewLocal(profile, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checked.Count(context.Background(), testRequest()); err != nil || proxied.Load() != 0 {
		t.Fatal("ambient proxy used")
	}
	for _, answer := range [][]netip.Addr{nil, make([]netip.Addr, 17), {netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("10.0.0.1")}} {
		options.Resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) { return answer, nil })
		checked, err = NewLocal(profile, options)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := checked.Count(context.Background(), testRequest()); err == nil {
			t.Fatal("invalid or mixed DNS set accepted")
		}
	}
	options.Timeout = 20 * time.Millisecond
	options.Resolver = resolverFunc(func(ctx context.Context, _, _ string) ([]netip.Addr, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	checked, err = NewLocal(profile, options)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := checked.Count(context.Background(), testRequest()); err == nil || time.Since(started) > time.Second {
		t.Fatal("DNS did not honor deadline")
	}
}

func TestLocalLoopbackHTTPAndProfileConfigurationBounds(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(&protocolFixture{})
	defer server.Close()
	local, err := NewLocal(fixtureProfile(server.URL), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := local.Count(context.Background(), testRequest()); err != nil {
		t.Fatal("explicit loopback HTTP rejected")
	}
	for _, change := range []func(*Profile){
		func(profile *Profile) { profile.ID = "" },
		func(profile *Profile) { profile.Generation = 0 },
		func(profile *Profile) { profile.ExpectedAlias = "unbound" },
		func(profile *Profile) { profile.ExpectedTemplateSHA256 = strings.Repeat("A", 64) },
		func(profile *Profile) { profile.ExpectedTemplateSHA256 = strings.Repeat("g", 64) },
		func(profile *Profile) { profile.ExpectedBuild = "bad\nvalue" },
		func(profile *Profile) { profile.ApprovedAddresses = nil },
		func(profile *Profile) {
			profile.ApprovedAddresses = append(profile.ApprovedAddresses, profile.ApprovedAddresses[0])
		},
	} {
		profile := fixtureProfile(server.URL)
		change(&profile)
		if _, err := NewLocal(profile, Options{}); err == nil {
			t.Fatal("invalid operator profile accepted")
		}
	}
	for _, duration := range []time.Duration{-time.Second, time.Microsecond, port.MaximumCallDuration + time.Second} {
		if _, err := NewLocal(fixtureProfile(server.URL), Options{Timeout: duration}); err == nil {
			t.Fatal("unbounded call duration accepted")
		}
	}
	if _, err := local.exchange(context.Background(), "/arbitrary", nil); err != port.ErrInvalid {
		t.Fatal("arbitrary endpoint allowed")
	}
	if _, err := local.exchange(context.Background(), "/tokenize", make([]byte, maximumWireBytes+1)); err != port.ErrInputLimit {
		t.Fatal("oversized request accepted")
	}
	if _, err := local.Count(nil, testRequest()); err != port.ErrInvalid {
		t.Fatal("nil context accepted")
	}
}

func TestLocalDoesNotMutateBorrowedResolverAnswers(t *testing.T) {
	t.Parallel()
	shared := []netip.Addr{netip.MustParseAddr("::ffff:127.0.0.1")}
	local, err := NewLocal(fixtureProfile("https://example.test"), Options{Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return shared, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	checked, err := local.resolve(context.Background(), "example.test")
	if err != nil || len(checked) != 1 || checked[0] != netip.MustParseAddr("127.0.0.1") || shared[0] != netip.MustParseAddr("::ffff:127.0.0.1") {
		t.Fatal("resolver answer was modified or not normalized")
	}
	checked[0] = netip.MustParseAddr("10.0.0.1")
	if shared[0] != netip.MustParseAddr("::ffff:127.0.0.1") {
		t.Fatal("checked addresses retained a mutable borrowed slice")
	}
}
