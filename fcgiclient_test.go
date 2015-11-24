package fcgiclient

import (
	"net/http"
	"testing"
)

func TestHeaders2Env(t *testing.T) {
	header := make(http.Header)
	header.Add("Host", "google.com")
	header.Add("user-agent", "Chrome")

	env := headers2env(header)

	value, ok := env["HTTP_HOST"]
	if !ok {
		t.Error("Host header does not appear in the env")
	} else {
		if value != "google.com" {
			t.Error("Value of http host header is wrong")
		}
	}
	value, ok = env["HTTP_USER_AGENT"]
	if !ok {
		t.Error("Host User Agent does not appear in the env")
	} else {
		if value != "Chrome" {
			t.Error("Value of http host header is wrong")
		}
	}

}
