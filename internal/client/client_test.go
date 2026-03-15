package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ajesus37/hCTF2/internal/client"
)

func TestDoSetsAuthCookie(t *testing.T) {
	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie("auth_token")
		if c != nil {
			gotCookie = c.Value
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := client.New(srv.URL, "mytoken")
	req, _ := http.NewRequest("GET", srv.URL+"/api/challenges", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotCookie != "mytoken" {
		t.Errorf("expected cookie auth_token=mytoken, got %q", gotCookie)
	}
}
