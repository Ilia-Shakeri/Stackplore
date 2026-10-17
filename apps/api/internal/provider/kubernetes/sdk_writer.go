package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

const operationReceiptAnnotation = "stackplore.io/operation-id"
const restartReceiptAnnotation = "stackplore.io/restart-request"

type WriterOptions struct {
	Client          *http.Client
	BearerToken     string
	TokenReader     func(context.Context) (string, error)
	Namespaces      []string
	MaximumResponse int64
	Now             func() time.Time
}

// SDKWriter owns an independent operator credential and a narrow transport.
type SDKWriter struct {
	client     *http.Client
	apps       appsv1.AppsV1Interface
	namespaces []string
}

func NewSDKWriter(options WriterOptions) (*SDKWriter, error) {
	if options.Client == nil || !validBearerToken(options.BearerToken) || len(options.Namespaces) < 1 || len(options.Namespaces) > maximumNamespaces || options.MaximumResponse < minimumAPIResponseBytes || options.MaximumResponse > maximumAPIResponseBytes {
		return nil, ErrUnavailable
	}
	namespaces := slices.Clone(options.Namespaces)
	slices.Sort(namespaces)
	for i, namespace := range namespaces {
		if len(validation.IsDNS1123Label(namespace)) != 0 || i > 0 && namespace == namespaces[i-1] {
			return nil, ErrPermissionDenied
		}
	}
	client := *options.Client
	client.Timeout = minPositiveDuration(client.Timeout, 15*time.Second)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return ErrPermissionDenied }
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	}
	if transport, ok := base.(*http.Transport); ok {
		clone := transport.Clone()
		clone.Proxy = nil
		clone.DisableCompression = true
		clone.ResponseHeaderTimeout = minPositiveDuration(clone.ResponseHeaderTimeout, client.Timeout)
		clone.MaxResponseHeaderBytes = 64 * 1024
		base = clone
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	client.Transport = &boundedTransport{base: base, tokens: &bearerSource{token: options.BearerToken, read: options.TokenReader, now: now, expiresAt: now().Add(time.Minute)}, namespaces: namespaces, maximum: options.MaximumResponse, writer: true}
	config := &rest.Config{Host: kubernetesAPIBase, Timeout: client.Timeout, ContentConfig: rest.ContentConfig{ContentType: "application/json", AcceptContentTypes: "application/json"}, RateLimiter: flowcontrol.NewTokenBucketRateLimiter(5, 5), UserAgent: "stackplore-guarded-writer", WarningHandlerWithContext: rest.NoWarnings{}}
	apps, err := appsv1.NewForConfigAndClient(config, &client)
	if err != nil {
		return nil, ErrUnavailable
	}
	return &SDKWriter{client: &client, apps: apps, namespaces: namespaces}, nil
}

func (writer *SDKWriter) Close() error {
	if writer != nil && writer.client != nil {
		writer.client.CloseIdleConnections()
		if transport, ok := writer.client.Transport.(*boundedTransport); ok {
			transport.tokens.close()
		}
	}
	return nil
}

type writerObject struct {
	Metadata metav1.ObjectMeta `json:"metadata"`
	Spec     struct {
		Replicas *int32 `json:"replicas"`
		Template struct {
			Metadata struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
		} `json:"template"`
	} `json:"spec"`
}

func (writer *SDKWriter) read(ctx context.Context, operation WorkloadOperation) (writerObject, error) {
	if writer == nil || ctx == nil || ctx.Err() != nil || writer.apps == nil || !slices.Contains(writer.namespaces, operation.Namespace) || !safeKubernetesPathSegment(operation.Name) || !safeKubernetesPathSegment(operation.ExpectedUID) || domain.NativeResourceID(operation.ResourceVersion).Validate() != nil || domain.RequestID(operation.OperationID).Validate() != nil {
		return writerObject{}, ErrPermissionDenied
	}
	resource := writerResource(operation.Kind)
	if resource == "" {
		return writerObject{}, ErrUnsupported
	}
	data, err := writer.apps.RESTClient().Get().Namespace(operation.Namespace).Resource(resource).Name(operation.Name).MaxRetries(0).Do(ctx).Raw()
	if err != nil {
		return writerObject{}, mapAPIError(err)
	}
	var object writerObject
	if json.Unmarshal(data, &object) != nil || string(object.Metadata.UID) != operation.ExpectedUID || object.Metadata.Name != operation.Name || object.Metadata.Namespace != operation.Namespace || domain.NativeResourceID(object.Metadata.ResourceVersion).Validate() != nil {
		return writerObject{}, ErrVersionConflict
	}
	return object, nil
}

func writerResource(kind string) string {
	switch kind {
	case "Deployment":
		return "deployments"
	case "StatefulSet":
		return "statefulsets"
	case "DaemonSet":
		return "daemonsets"
	default:
		return ""
	}
}

func (writer *SDKWriter) PreviewWorkload(ctx context.Context, operation WorkloadOperation) error {
	operation.OperationID = "operation_preview"
	object, err := writer.read(ctx, operation)
	if err != nil {
		return err
	}
	if object.Metadata.ResourceVersion != operation.ResourceVersion {
		return ErrVersionConflict
	}
	return nil
}

func (writer *SDKWriter) RestartWorkload(ctx context.Context, operation WorkloadOperation) (string, error) {
	if operation.RequestedReplicas != nil {
		return "", ErrUnsupported
	}
	return writer.patch(ctx, operation, false)
}
func (writer *SDKWriter) ScaleWorkload(ctx context.Context, operation WorkloadOperation) (string, error) {
	if operation.Kind == "DaemonSet" || operation.RequestedReplicas == nil || *operation.RequestedReplicas < 0 || *operation.RequestedReplicas > domain.MaxOperationReplicas {
		return "", ErrUnsupported
	}
	return writer.patch(ctx, operation, true)
}

func (writer *SDKWriter) patch(ctx context.Context, operation WorkloadOperation, scale bool) (string, error) {
	before, err := writer.read(ctx, operation)
	if err != nil {
		return "", err
	}
	if before.Metadata.ResourceVersion != operation.ResourceVersion {
		return "", ErrVersionConflict
	}
	// Both mutations put the durable receipt on the base object in the same CAS.
	body := struct {
		Metadata struct {
			UID             string            `json:"uid"`
			ResourceVersion string            `json:"resourceVersion"`
			Annotations     map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Replicas *int `json:"replicas,omitempty"`
			Template *struct {
				Metadata struct {
					Annotations map[string]string `json:"annotations"`
				} `json:"metadata"`
			} `json:"template,omitempty"`
		} `json:"spec"`
	}{}
	body.Metadata.UID = operation.ExpectedUID
	body.Metadata.ResourceVersion = operation.ResourceVersion
	body.Metadata.Annotations = map[string]string{operationReceiptAnnotation: operation.OperationID}
	if scale {
		body.Spec.Replicas = operation.RequestedReplicas
	} else {
		body.Spec.Template = &struct {
			Metadata struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
		}{}
		body.Spec.Template.Metadata.Annotations = map[string]string{restartReceiptAnnotation: operation.OperationID}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", ErrUnavailable
	}
	data, err := writer.apps.RESTClient().Patch(types.MergePatchType).Namespace(operation.Namespace).Resource(writerResource(operation.Kind)).Name(operation.Name).Body(payload).MaxRetries(0).Do(ctx).Raw()
	if err != nil {
		if apierrors.IsConflict(err) {
			return "", ErrVersionConflict
		}
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
			return "", ErrPermissionDenied
		}
		if apierrors.IsNotFound(err) || apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) || apierrors.IsMethodNotSupported(err) {
			return "", ErrUnavailable
		}
		return "", domain.ErrOperationOutcomeUncertain
	}
	var result writerObject
	if json.Unmarshal(data, &result) != nil || string(result.Metadata.UID) != operation.ExpectedUID || result.Metadata.Name != operation.Name || result.Metadata.Namespace != operation.Namespace || result.Metadata.Annotations[operationReceiptAnnotation] != operation.OperationID || domain.NativeResourceID(result.Metadata.ResourceVersion).Validate() != nil || result.Metadata.ResourceVersion == operation.ResourceVersion {
		return "", domain.ErrOperationOutcomeUncertain
	}
	if scale && (result.Spec.Replicas == nil || int(*result.Spec.Replicas) != *operation.RequestedReplicas) || !scale && result.Spec.Template.Metadata.Annotations[restartReceiptAnnotation] != operation.OperationID {
		return "", domain.ErrOperationOutcomeUncertain
	}
	return result.Metadata.ResourceVersion, nil
}

func (writer *SDKWriter) ReconcileWorkload(ctx context.Context, operation WorkloadOperation) (string, bool, error) {
	object, err := writer.read(ctx, operation)
	if errors.Is(err, ErrVersionConflict) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if object.Metadata.Annotations[operationReceiptAnnotation] != operation.OperationID {
		return "", false, nil
	}
	return object.Metadata.ResourceVersion, true, nil
}

var _ OperationCollector = (*SDKWriter)(nil)
