package errors

import "net/http"

type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e AppError) Error() string {
	return e.Message
}

func New(code, message string, status int) AppError {
	return AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
	}
}

var (
	ErrInternal     = New("INTERNAL_ERROR", "internal server error", http.StatusInternalServerError)
	ErrNotFound     = New("NOT_FOUND", "resource not found", http.StatusNotFound)
	ErrBadRequest   = New("BAD_REQUEST", "bad request", http.StatusBadRequest)
	ErrUnauthorized = New("UNAUTHORIZED", "authentication required", http.StatusUnauthorized)
	ErrForbidden    = New("FORBIDDEN", "permission denied", http.StatusForbidden)
	ErrBadMethod    = New("METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
)
