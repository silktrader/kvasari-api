package json_utilities

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

var errEncoding = errors.New("error while encoding response")

type httpError struct {
	Error     string
	Timestamp time.Time
}

func newHttpError(err error) *httpError {
	return &httpError{err.Error(), time.Now()}
}

type httpMessage struct {
	Message   string
	Timestamp time.Time
}

func newHttpMessage(message string) *httpMessage {
	return &httpMessage{message, time.Now()}
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

func NotFound(writer http.ResponseWriter, message string) {
	encodeJSON(writer, http.StatusNotFound, newHttpMessage(message))
}

func BadRequest(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusBadRequest)
}

func Unauthorised(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusUnauthorized)
}

func BadRequestWithMessage(writer http.ResponseWriter, message string) {
	encodeJSON(writer, http.StatusBadRequest, newHttpMessage(message))
}

func InternalServerError(writer http.ResponseWriter, err error) {
	encoderJSONError(writer, http.StatusInternalServerError, err)
}

func ValidationError(writer http.ResponseWriter, err error) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(writer).Encode(newHttpError(err))
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

func encoderJSONError(writer http.ResponseWriter, status int, err error) {
	// tk log errors
	encodeJSON(writer, status, newHttpError(err))
}

func DecodeValidate[T Validator](request *http.Request) (data T, err error) {
	if err = json.NewDecoder(request.Body).Decode(&data); err != nil {
		return data, err
	}
	return data, data.Validate()
}

type Validator interface {
	Validate() error
}
