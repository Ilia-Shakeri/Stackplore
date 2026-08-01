package domain

import "fmt"

type ProviderErrorCode string

const (
	ProviderErrorInvalidRequest   ProviderErrorCode = "invalid_request"
	ProviderErrorCanceled         ProviderErrorCode = "canceled"
	ProviderErrorDeadlineExceeded ProviderErrorCode = "deadline_exceeded"
	ProviderErrorUnavailable      ProviderErrorCode = "unavailable"
	ProviderErrorPermissionDenied ProviderErrorCode = "permission_denied"
	ProviderErrorUnsupported      ProviderErrorCode = "unsupported"
	ProviderErrorRateLimited      ProviderErrorCode = "rate_limited"
	ProviderErrorInternal         ProviderErrorCode = "internal"
)

func (code ProviderErrorCode) Validate() error {
	switch code {
	case ProviderErrorInvalidRequest, ProviderErrorCanceled, ProviderErrorDeadlineExceeded, ProviderErrorUnavailable, ProviderErrorPermissionDenied, ProviderErrorUnsupported, ProviderErrorRateLimited, ProviderErrorInternal:
		return nil
	default:
		return fmt.Errorf("unknown provider error code %q", code)
	}
}

type ProviderOperation string

func (operation ProviderOperation) Validate() error {
	return validateNamespacedCode("provider operation", string(operation))
}

type ProviderError struct {
	Operation ProviderOperation
	Code      ProviderErrorCode
	Retryable bool
	cause     error
}

func NewProviderError(operation ProviderOperation, code ProviderErrorCode, retryable bool, cause error) (*ProviderError, error) {
	result := &ProviderError{Operation: operation, Code: code, Retryable: retryable, cause: cause}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}

func (providerError *ProviderError) Validate() error {
	if providerError == nil {
		return fmt.Errorf("provider error must not be nil")
	}
	if err := providerError.Operation.Validate(); err != nil {
		return err
	}
	if err := providerError.Code.Validate(); err != nil {
		return err
	}
	if providerError.cause == nil {
		return fmt.Errorf("provider error cause must not be nil")
	}
	if providerError.Retryable && (providerError.Code == ProviderErrorInvalidRequest || providerError.Code == ProviderErrorCanceled || providerError.Code == ProviderErrorPermissionDenied || providerError.Code == ProviderErrorUnsupported) {
		return fmt.Errorf("provider error code %q cannot be retryable", providerError.Code)
	}
	return nil
}

func (providerError *ProviderError) Error() string {
	if providerError == nil {
		return "provider operation failed"
	}
	return fmt.Sprintf("provider operation %q failed with %q", providerError.Operation, providerError.Code)
}

func (providerError *ProviderError) Unwrap() error {
	if providerError == nil {
		return nil
	}
	return providerError.cause
}
