package authentication

import "net/http"

type AuthType struct {
	Name       string
	Detection  func(w http.ResponseWriter, r *http.Request) bool
	Validation func(w http.ResponseWriter, r *http.Request) (*http.Request, error)
}
