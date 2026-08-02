package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"unicode/utf8"
)

const (
	maximumJSONDepth = 16
	maximumJSONKeys  = 64
)

var (
	errJSONBodyInvalid  = errors.New("JSON request body is invalid")
	errJSONBodyTooLarge = errors.New("JSON request body is too large")
	errJSONMediaType    = errors.New("JSON request media type is invalid")
)

func decodeJSONBody(r *http.Request, maximumBytes int64, target any) error {
	if r == nil || target == nil || maximumBytes < 2 || r.Body == nil {
		return errJSONBodyInvalid
	}
	contentTypes := r.Header.Values("Content-Type")
	if len(contentTypes) != 1 {
		return errJSONMediaType
	}
	mediaType, parameters, err := mime.ParseMediaType(contentTypes[0])
	if err != nil || mediaType != "application/json" || len(parameters) != 0 {
		return errJSONMediaType
	}
	if r.ContentLength > maximumBytes {
		return errJSONBodyTooLarge
	}

	contents, err := io.ReadAll(io.LimitReader(r.Body, maximumBytes+1))
	if err != nil {
		var maximumError *http.MaxBytesError
		if errors.As(err, &maximumError) {
			return errJSONBodyTooLarge
		}
		return errJSONBodyInvalid
	}
	if int64(len(contents)) > maximumBytes {
		return errJSONBodyTooLarge
	}
	defer clear(contents)
	if len(contents) == 0 || !utf8.Valid(contents) || duplicateJSONKey(contents) != nil {
		return errJSONBodyInvalid
	}

	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errJSONBodyInvalid
	}
	if err := requireJSONEnd(decoder); err != nil {
		return errJSONBodyInvalid
	}
	return nil
}

func duplicateJSONKey(contents []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	if err := inspectJSONValue(decoder, 0); err != nil {
		return err
	}
	return requireJSONEnd(decoder)
}

func inspectJSONValue(decoder *json.Decoder, depth int) error {
	if depth > maximumJSONDepth {
		return errJSONBodyInvalid
	}
	token, err := decoder.Token()
	if err != nil {
		return errJSONBodyInvalid
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}

	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok || len(keys) >= maximumJSONKeys {
				return errJSONBodyInvalid
			}
			if _, exists := keys[key]; exists {
				return errJSONBodyInvalid
			}
			keys[key] = struct{}{}
			if err := inspectJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errJSONBodyInvalid
		}
		return nil
	case '[':
		for decoder.More() {
			if err := inspectJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errJSONBodyInvalid
		}
		return nil
	default:
		return errJSONBodyInvalid
	}
}

func requireJSONEnd(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errJSONBodyInvalid
	}
	return nil
}
