package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func (s *Server) WithHandlerFunc(path string, handler http.HandlerFunc, methods ...string) *Server {
	s.Router.HandleFunc(path, handler).Methods(methods...)
	return s
}

type OciErrors struct {
	Errors []OciError `json:"errors"`
}

type OciError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details"`
}

func (e OciError) Error() string {
	return e.Message
}

var (
	ERROR_BLOB_UNKNOWN = OciError{
		Code:    "code-1",
		Message: "BLOB_UNKNOWN",
		Details: "blob unknown to registry",
	}
	ERROR_BLOB_UPLOAD_INVALID = OciError{
		Code:    "code-2",
		Message: "BLOB_UPLOAD_INVALID",
		Details: "blob upload invalid",
	}
	ERROR_BLOB_UPLOAD_UNKNOWN = OciError{
		Code:    "code-3",
		Message: "BLOB_UPLOAD_UNKNOWN",
		Details: "blob upload unknown to registry",
	}
	ERROR_DIGEST_INVALID = OciError{
		Code:    "code-4",
		Message: "DIGEST_INVALID",
		Details: "provided digest did not match uploaded content",
	}
	ERROR_MANIFEST_BLOB_UNKNOWN = OciError{
		Code:    "code-5",
		Message: "MANIFEST_BLOB_UNKNOWN",
		Details: "manifest references a manifest or blob unknown to registry",
	}
	ERROR_MANIFEST_INVALID = OciError{
		Code:    "code-6",
		Message: "MANIFEST_INVALID",
		Details: "manifest invalid",
	}
	ERROR_MANIFEST_UNKNOWN = OciError{
		Code:    "code-7",
		Message: "MANIFEST_UNKNOWN",
		Details: "manifest unknown to registry",
	}
	ERROR_NAME_INVALID = OciError{
		Code:    "code-8",
		Message: "NAME_INVALID",
		Details: "invalid repository name",
	}
	ERROR_NAME_UNKNOWN = OciError{
		Code:    "code-9",
		Message: "NAME_UNKNOWN",
		Details: "repository name not known to registry",
	}
	ERROR_SIZE_INVALID = OciError{
		Code:    "code-10",
		Message: "SIZE_INVALID",
		Details: "provided length did not match content length",
	}
	ERROR_UNAUTHORIZED = OciError{
		Code:    "code-11",
		Message: "UNAUTHORIZED",
		Details: "authentication required",
	}
	ERROR_DENIED = OciError{
		Code:    "code-12",
		Message: "DENIED",
		Details: "requested access to the resource is denied",
	}
	ERROR_UNSUPPORTED = OciError{
		Code:    "code-13",
		Message: "UNSUPPORTED",
		Details: "the operation is unsupported",
	}
	ERROR_TOOMANYREQUESTS = OciError{
		Code:    "code-14",
		Message: "TOOMANYREQUESTS",
		Details: "too many requests",
	}
)

func WriteErrors(w http.ResponseWriter, errors ...OciError) error {
	errs := OciErrors{
		Errors: errors,
	}
	// TODO: Write the appropriate status code
	w.WriteHeader(400)

	log.Println(errors[0].Message)

	err := json.NewEncoder(w).Encode(errs)
	w.Header().Set("application/json", "application/json")
	return err
}

func Error(w http.ResponseWriter, message string, code int) {
	log.Println(message)
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(OciError{
		Code:    fmt.Sprintf("code-%d", code),
		Message: message,
		Details: message,
	})
}

func Warning(w http.ResponseWriter, code int, message string) {
	w.Header().Add("Warning", fmt.Sprintf(`%d - "%s"`, code, message))
}

func GetScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	// Optional: "X-Forwarded-Proto" für Reverse Proxies prüfen
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}
