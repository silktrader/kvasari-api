package json_utilities

import (
	"encoding/json"
	"errors"
	"github.com/silktrader/kvasari/pkg/ntime"
	"net/http"
)

var errEncoding = errors.New("error while encoding response")

type httpError struct {
	Error     string
	Timestamp ntime.NTime
}

func newHttpError(err error) *httpError {
	return &httpError{err.Error(), ntime.Now()}
}

type httpMessage struct {
	Message   string
	Timestamp ntime.NTime
}

func newHttpMessage(message string) *httpMessage {
	return &httpMessage{message, ntime.Now()}
}

// Created encodes a JSON object in a 201 created response.
func Created(writer http.ResponseWriter, payload interface{}) {
	encodeJSON(writer, http.StatusCreated, payload)
}

// Ok encodes a JSON object in a 200 OK response.
func Ok(writer http.ResponseWriter, payload interface{}) {
	encodeJSON(writer, http.StatusOK, payload)
}

// NoContent sets the appropriate headers of a 204 no content response.
func NoContent(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusNoContent)
}

// NotFound encodes a JSON object containing a timestamp and a message in a 404 not found response.
func NotFound(writer http.ResponseWriter, message string) {
	encodeJSON(writer, http.StatusNotFound, newHttpMessage(message))
}

// BadRequest sets the appropriate headers of a 400 bad request response.
func BadRequest(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusBadRequest)
}

// Forbidden sets the appropriate headers of a 403 forbidden response.
func Forbidden(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusForbidden)
}

// BadRequestWithMessage encodes a JSON object containing a timestamp and a message in a 400 bad request response.
func BadRequestWithMessage(writer http.ResponseWriter, message string) {
	encodeJSON(writer, http.StatusBadRequest, newHttpMessage(message))
}

// InternalServerError encodes a JSON object containing a timestamp and an error message in a 500 server error response.
func InternalServerError(writer http.ResponseWriter, err error) {
	encodeJSON(writer, http.StatusInternalServerError, newHttpError(err))
}

// ValidationError encodes a JSON object containing a timestamp and a message in a 400 bad request response.
func ValidationError(writer http.ResponseWriter, err error) {
	encodeJSON(writer, http.StatusBadRequest, newHttpMessage(err.Error()))
}

func encodeJSON(writer http.ResponseWriter, status int, payload interface{}) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(writer).Encode(newHttpError(errEncoding))
	}
}

// DecodeValidate performs validation checks on any data argument that implements `Validate()`.
func DecodeValidate[T Validator](request *http.Request) (data T, err error) {
	if err = json.NewDecoder(request.Body).Decode(&data); err != nil {
		return data, err
	}
	return data, data.Validate()
}

type Validator interface {
	Validate() error
}
