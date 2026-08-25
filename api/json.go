package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxRequestBody = 1 << 20

type errorBody struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var syntax *json.SyntaxError
		var typeError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntax):
			return fmt.Errorf("malformed JSON at byte %d", syntax.Offset)
		case errors.As(err, &typeError):
			return fmt.Errorf("field %s has invalid type", typeError.Field)
		case errors.Is(err, io.EOF):
			return fmt.Errorf("request body is required")
		case err.Error() == "http: request body too large":
			return fmt.Errorf("request body exceeds %d bytes", maxRequestBody)
		default:
			return err
		}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request body must contain one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, err error) {
	requestID, _ := r.Context().Value(requestIDKey{}).(string)
	writeJSON(w, status, errorBody{Error: apiError{Code: code, Message: err.Error(), RequestID: requestID}})
}
