package httpError

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

type Message struct {
	w       http.ResponseWriter
	context map[string]string
}

type ErrorBody struct {
	Message string            `json:"message"`
	Context map[string]string `json:"context"`
}

// Http create a *Message object
func Http(w http.ResponseWriter, r *http.Request, code int) *Message {
	ErrorResponse := &Message{w: w, context: make(map[string]string)}
	ErrorResponse.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	ErrorResponse.w.Header().Set("X-Content-Type-Options", "nosniff")
	ErrorResponse.w.WriteHeader(code)

	requestId := middleware.GetReqID(r.Context())
	if requestId != "" {
		ErrorResponse.context["requestId"] = requestId
	}

	return ErrorResponse
}

// Msg sends the *Message with msg added as the message field if not empty.
//
// NOTICE: once this method is called, the *Message should be disposed.
// Calling Msg twice can have unexpected result.
func (e *Message) Msg(msg string) {
	if e == nil {
		return
	}
	e.msg(msg)
}

// Send is equivalent to calling Msg("").
//
// NOTICE: once this method is called, the *Message should be disposed.
func (e *Message) Send() {
	if e == nil {
		return
	}
	e.msg("")
}

// Msgf sends the event with formatted msg added as the message field if not empty.
//
// NOTICE: once this method is called, the *Message should be disposed.
// Calling Msgf twice can have unexpected result.
func (e *Message) Msgf(format string, v ...interface{}) {
	if e == nil {
		return
	}
	e.msg(fmt.Sprintf(format, v...))
}

// MsgFunc sends the event with function returning string added as the message field if not empty.
//
// NOTICE: once this method is called, the *Message should be disposed.
// Calling MsgFunc twice can have unexpected result.
func (e *Message) MsgFunc(createMsg func() string) {
	if e == nil {
		return
	}
	e.msg(createMsg())
}

func (e *Message) msg(msg string) {
	// build response body
	body := ErrorBody{
		Message: msg,
		Context: e.context,
	}

	bodyStr, _ := json.Marshal(body)
	_, err := e.w.Write(bodyStr)
	if err != nil {
		log.Err(err).Msgf("error writing response")
		return
	}
}

// Str adds the field key with val as a string to the *Message context.
func (e *Message) Str(key, val string) *Message {
	if e == nil {
		return e
	}
	e.context[key] = val
	return e
}

// Any adds the field key with val as a string to the *Message context.
func (e *Message) Any(key string, val interface{}) *Message {
	if e == nil {
		return e
	}

	valStr, _ := json.Marshal(val)
	e.context[key] = string(valStr)
	return e
}
