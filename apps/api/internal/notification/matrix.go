package notification

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const MatrixAdapter = "matrix"

type MatrixTarget struct {
	Origin      string `json:"-"`
	RoomID      string `json:"-"`
	AccessToken []byte `json:"-"`
}

func (MatrixTarget) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[redacted Matrix target]"))
}

type MatrixDelivery struct {
	ID      string
	Payload []byte
}
type MatrixResult struct{ Status int }
type MatrixTargetResolver interface {
	ResolveMatrixTarget(context.Context, string) (MatrixTarget, error)
}
type MatrixAttemptSender interface {
	Send(context.Context, MatrixTarget, MatrixDelivery) (MatrixResult, error)
}

func MatrixAttempt(targets MatrixTargetResolver, sender MatrixAttemptSender) DeliveryAttempt {
	if targets == nil || sender == nil {
		return nil
	}
	return func(ctx context.Context, task DeliveryTask) (int, error) {
		target, err := targets.ResolveMatrixTarget(ctx, task.DestinationReferenceID)
		if err != nil {
			return 0, deliveryFailure("notification.target_unavailable", false)
		}
		defer clear(target.AccessToken)
		result, err := sender.Send(ctx, target, MatrixDelivery{ID: task.ID, Payload: task.Payload})
		return result.Status, err
	}
}

type MatrixSenderOptions = WebhookSenderOptions

type MatrixSender struct {
	outbound outboundOptions
	rootCAs  *x509.CertPool
}

func NewMatrixSender(options MatrixSenderOptions) (*MatrixSender, error) {
	outbound := outboundOptions{resolver: options.Resolver, dialer: options.Dialer, timeout: options.Timeout, maximumResponseBytes: options.MaximumResponseBytes}
	if !validOutboundOptions(outbound) {
		return nil, ErrDeliveryUnavailable
	}
	return &MatrixSender{outbound: outbound, rootCAs: options.RootCAs}, nil
}

func (sender *MatrixSender) Send(ctx context.Context, target MatrixTarget, delivery MatrixDelivery) (MatrixResult, error) {
	if sender == nil || ctx == nil || ctx.Err() != nil || !validDeliveryID(delivery.ID) || !validMatrixPayload(delivery.Payload) || !validMatrixRoomID(target.RoomID) || !validMatrixToken(target.AccessToken) {
		return MatrixResult{}, deliveryFailure("notification.invalid_request", false)
	}
	origin, err := parseMatrixOrigin(target.Origin)
	if err != nil {
		return MatrixResult{}, deliveryFailure("notification.destination_rejected", false)
	}
	endpoint, err := url.Parse(origin.String() + "/_matrix/client/v3/rooms/" + url.PathEscape(target.RoomID) + "/send/m.room.message/" + delivery.ID)
	if err != nil || endpoint.Scheme != origin.Scheme || endpoint.Host != origin.Host {
		return MatrixResult{}, deliveryFailure("notification.destination_rejected", false)
	}
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer "+string(target.AccessToken))
	options := sender.outbound
	options.rootCAs = sender.rootCAs
	status, body, err := sendOutbound(ctx, options, endpoint, http.MethodPut, delivery.Payload, headers)
	if err != nil {
		return MatrixResult{Status: status}, err
	}
	var response struct {
		EventID string `json:"event_id"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &response) != nil || len(response.EventID) < 2 || len(response.EventID) > 255 || response.EventID[0] != '$' {
		return MatrixResult{Status: status}, uncertainDeliveryFailure("notification.invalid_response")
	}
	return MatrixResult{Status: status}, nil
}

type matrixPayload struct {
	MsgType string `json:"msgtype"`
	Body    string `json:"body"`
}

// RenderMatrixPayload has no provider text or credentials. Persist its bytes once.
func RenderMatrixPayload(item domain.Notification) ([]byte, error) {
	if item.Validate() != nil || item.Synthetic {
		return nil, ErrDeliveryUnavailable
	}
	body := "Stackplore " + string(item.Severity) + ": " + string(item.SummaryCode) + "\nNotification: " + string(item.ID) + "\nObserved: " + item.CreatedAt.Format(time.RFC3339Nano)
	return json.Marshal(matrixPayload{MsgType: "m.notice", Body: body})
}

func validMatrixPayload(payload []byte) bool {
	if len(payload) < 2 || len(payload) > maximumWebhookPayload {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var value matrixPayload
	if decoder.Decode(&value) != nil || value.MsgType != "m.notice" || len(value.Body) < 1 || len(value.Body) > 4096 {
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF && !strings.ContainsAny(value.Body, "\x00\r")
}

func parseMatrixOrigin(raw string) (*url.URL, error) {
	origin, err := parseDestination(raw)
	if err != nil || origin.Path != "" || origin.RawPath != "" || origin.RawQuery != "" || origin.ForceQuery || origin.Port() == "443" {
		return nil, ErrDeliveryUnavailable
	}
	return origin, nil
}

func validMatrixRoomID(room string) bool {
	if len(room) < 2 || len(room) > 255 || room[0] != '!' {
		return false
	}
	for _, char := range room[1:] {
		if char < 33 || char > 126 || strings.ContainsRune("/\\?#%", char) {
			return false
		}
	}
	return true
}

func validMatrixToken(token []byte) bool {
	if len(token) < 1 || len(token) > 4096 {
		return false
	}
	padding := false
	characters := 0
	for _, char := range token {
		if char == '=' {
			if characters == 0 {
				return false
			}
			padding = true
			continue
		}
		if padding || !(asciiLetter(char) || char >= '0' && char <= '9' || strings.ContainsRune("-._~+/", rune(char))) {
			return false
		}
		characters++
	}
	return true
}

type StaticMatrixTargetResolver struct {
	mu                      sync.RWMutex
	reference, origin, room string
	token                   []byte
	closed                  bool
}

func NewStaticMatrixTargetResolver(reference, origin, room string, token []byte) (*StaticMatrixTargetResolver, error) {
	if !validDeliveryID(reference) || !validMatrixRoomID(room) || !validMatrixToken(token) {
		return nil, ErrDeliveryUnavailable
	}
	if _, err := parseMatrixOrigin(origin); err != nil {
		return nil, ErrDeliveryUnavailable
	}
	return &StaticMatrixTargetResolver{reference: reference, origin: origin, room: room, token: append([]byte(nil), token...)}, nil
}

func (resolver *StaticMatrixTargetResolver) ResolveMatrixTarget(ctx context.Context, reference string) (MatrixTarget, error) {
	if resolver == nil || ctx == nil || ctx.Err() != nil {
		return MatrixTarget{}, ErrDeliveryUnavailable
	}
	resolver.mu.RLock()
	defer resolver.mu.RUnlock()
	if resolver.closed || reference != resolver.reference {
		return MatrixTarget{}, ErrDeliveryUnavailable
	}
	return MatrixTarget{Origin: resolver.origin, RoomID: resolver.room, AccessToken: append([]byte(nil), resolver.token...)}, nil
}

func (resolver *StaticMatrixTargetResolver) Close() {
	if resolver == nil {
		return
	}
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	clear(resolver.token)
	resolver.token = nil
	resolver.origin = ""
	resolver.room = ""
	resolver.closed = true
}
