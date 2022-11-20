package json_utilities

import (
	"encoding/json"
	"net/http"
	"time"
)

type httpError struct {
	Error     string
	Timestamp time.Time
}

func Created(writer http.ResponseWriter, payload interface{}) {
	encodeJSON(writer, http.StatusCreated, payload)
}

func Ok(writer http.ResponseWriter, payload interface{}) {
	encodeJSON(writer, http.StatusOK, payload)
}

func NoContent(writer http.ResponseWriter) {
	// no content type header needed
	writer.WriteHeader(http.StatusNoContent)
}

func BadRequest(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusBadRequest)
}

func InternalServerError(writer http.ResponseWriter, err error) {
	encoderJSONError(writer, http.StatusInternalServerError, err)
}

func ValidationError(writer http.ResponseWriter, err error) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(writer).Encode(struct {
		Error     error
		Timestamp time.Time
	}{
		err, time.Now(),
	})
}

func encodeJSON(writer http.ResponseWriter, status int, payload interface{}) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(writer).Encode(httpError{"Error while encoding payload", time.Now()})
	}
}

func encoderJSONError(writer http.ResponseWriter, status int, err error) {
	// tk log errors
	encodeJSON(writer, status, httpError{err.Error(), time.Now()})
}

func DecodeValidate(r *http.Request, data Validator) error {
	if err := json.NewDecoder(r.Body).Decode(data); err != nil {
		return err
	}
	return data.Validate()
}

type Validator interface {
	Validate() error
}
