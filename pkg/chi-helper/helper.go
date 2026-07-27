package helper

import (
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/super-phenix/superphenix/pkg/chi-helper/model"

	"github.com/rs/zerolog/log"
)

// Content-Type MIME of the most common data formats.
const (
	MIMEJSON              = "application/json"
	MIMEHTML              = "text/html"
	MIMEXML               = "application/xml"
	MIMEXML2              = "text/xml"
	MIMEPlain             = "text/plain"
	MIMEPOSTForm          = "application/x-www-form-urlencoded"
	MIMEMultipartPOSTForm = "multipart/form-data"
	MIMEPROTOBUF          = "application/x-protobuf"
	MIMEMSGPACK           = "application/x-msgpack"
	MIMEMSGPACK2          = "application/msgpack"
	MIMEYAML              = "application/x-yaml"
	MIMEYAML2             = "application/yaml"
	MIMETOML              = "application/toml"
)

/************************************/
/******** RESPONSE RENDERING ********/
/************************************/

// bodyAllowedForStatus is a copy of http.bodyAllowedForStatus non-exported function.
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent:
		return false
	case status == http.StatusNotModified:
		return false
	}
	return true
}

// Header is an intelligent shortcut for c.Writer.Header().Set(key, value).
// It writes a header in the response.
// If value == "", this method removes the header `c.Writer.Header().Del(key)`
func Header(w http.ResponseWriter, key, value string) {
	if value == "" {
		w.Header().Del(key)
		return
	}
	w.Header().Set(key, value)
}

// GetHeader returns value from request headers.
func GetHeader(r http.Request, key string) string {
	return r.Header.Get(key)
}

// GetRawData returns stream data.
func GetRawData(r http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, errors.New("cannot read nil body")
	}
	return io.ReadAll(r.Body)
}

// SetCookie adds a Set-Cookie header to the ResponseWriter's headers.
// The provided cookie must have a valid Name. Invalid cookies may be
// silently dropped.
func SetCookie(w http.ResponseWriter, name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

// Cookie returns the named cookie provided in the request or
// ErrNoCookie if not found. And return the named cookie is unescaped.
// If multiple cookies match the given name, only one cookie will
// be returned.
func Cookie(r http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err
	}
	val, _ := url.QueryUnescape(cookie.Value)
	return val, nil
}

// Render writes the response headers and calls render.Render to render data.
func Render(w http.ResponseWriter, code int, r model.Render) {
	if !bodyAllowedForStatus(code) {
		r.WriteContentType(w)
		model.Status(w, code)
		return
	}

	if err := r.Render(w, code); err != nil {
		// Pushing error to c.Errors
		log.Error().AnErr("Failed to render response", err).Send()
	}
}

// String writes the given string into the response body.
func String(w http.ResponseWriter, code int, format string, values ...any) {
	Render(w, code, model.String{Format: format, Data: values})
}

// Data writes some data into the body stream and updates the HTTP code.
func Data(w http.ResponseWriter, code int, contentType string, data []byte) {
	Render(w, code, model.Data{
		ContentType: contentType,
		Data:        data,
	})
}

// Query returns the keyed url query value if it exists,
// otherwise it returns an empty string `("")`.
// It is shortcut for `c.Request.URL.Query().Get(key)`
//
//	    GET /path?id=1234&name=Manu&value=
//		   c.Query("id") == "1234"
//		   c.Query("name") == "Manu"
//		   c.Query("value") == ""
//		   c.Query("wtf") == ""
func Query(r *http.Request, key string) (value string) {
	value, _ = GetQuery(r, key)
	return
}

// DefaultQuery returns the keyed url query value if it exists,
// otherwise it returns the specified defaultValue string.
// See: Query() and GetQuery() for further information.
//
//	GET /?name=Manu&lastname=
//	c.DefaultQuery("name", "unknown") == "Manu"
//	c.DefaultQuery("id", "none") == "none"
//	c.DefaultQuery("lastname", "none") == ""
func DefaultQuery(r *http.Request, key, defaultValue string) string {
	if value, ok := GetQuery(r, key); ok {
		return value
	}
	return defaultValue
}

func GetQuery(r *http.Request, key string) (string, bool) {
	if values, ok := GetQueryArray(r, key); ok {
		return values[0], ok
	}
	return "", false
}

// QueryArray returns a slice of strings for a given query key.
// The length of the slice depends on the number of params with the given key.
func QueryArray(r *http.Request, key string) (values []string) {
	values, _ = GetQueryArray(r, key)
	return
}

// GetQueryArray returns a slice of strings for a given query key, plus
// a boolean value whether at least one value exists for the given key.
func GetQueryArray(r *http.Request, key string) (values []string, ok bool) {
	query := urlQuery(r)
	values, ok = query[key]
	return
}

func urlQuery(r *http.Request) url.Values {
	if r != nil {
		return r.URL.Query()
	} else {
		return url.Values{}
	}
}
