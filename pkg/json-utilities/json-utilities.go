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

func Forbidden(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusForbidden)
}

func BadRequestWithMessage(writer http.ResponseWriter, message string) {
	encodeJSON(writer, http.StatusBadRequest, newHttpMessage(message))
}

func InternalServerError(writer http.ResponseWriter, err error) {
	encoderJSONError(writer, http.StatusInternalServerError, err)
}

// ValidationError encodes a 400 BadRequest response with a JSON object, containing a timestamp and an error message.
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
