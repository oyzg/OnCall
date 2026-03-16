package response

import (
	"encoding/json"
	"net/http"
)

type Envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data,omitempty"`
}

func JSON(w http.ResponseWriter, status int, payload Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func Success(w http.ResponseWriter, status int, requestID string, data any) {
	JSON(w, status, Envelope{
		Code:      "OK",
		Message:   "success",
		RequestID: requestID,
		Data:      data,
	})
}

func Failure(w http.ResponseWriter, status int, requestID, code, message string) {
	JSON(w, status, Envelope{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	})
}
