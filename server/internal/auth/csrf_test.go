package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func csrfRouter(key []byte) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFMiddleware(key))
	r.GET("/t", func(c *gin.Context) { c.String(http.StatusOK, CSRFTokenFromContext(c)) })
	r.POST("/t", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.DELETE("/t", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func doReq(r *gin.Engine, method string, sessionID string, header, form string) *httptest.ResponseRecorder {
	var req *http.Request
	if form != "" {
		req = httptest.NewRequest(method, "/t", strings.NewReader(url.Values{csrfFormField: {form}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, "/t", nil)
	}
	if sessionID != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	}
	if header != "" {
		req.Header.Set(csrfHeaderName, header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCSRFToken_DeterministicAndSessionBound(t *testing.T) {
	key := NewCSRFKey()
	if CSRFToken(key, "a") != CSRFToken(key, "a") {
		t.Error("token should be deterministic")
	}
	if CSRFToken(key, "a") == CSRFToken(key, "b") {
		t.Error("tokens for different sessions must differ")
	}
	if CSRFToken(key, "a") == CSRFToken(NewCSRFKey(), "a") {
		t.Error("tokens for different keys must differ")
	}
}

func TestCSRFMiddleware_GETExposesToken(t *testing.T) {
	key := NewCSRFKey()
	w := doReq(csrfRouter(key), "GET", "sess1", "", "")
	if w.Code != 200 || w.Body.String() != CSRFToken(key, "sess1") {
		t.Errorf("got %d %q", w.Code, w.Body.String())
	}
}

func TestCSRFMiddleware_RejectsMissingOrWrongToken(t *testing.T) {
	key := NewCSRFKey()
	r := csrfRouter(key)
	for _, m := range []string{"POST", "DELETE"} {
		if w := doReq(r, m, "sess1", "", ""); w.Code != http.StatusForbidden {
			t.Errorf("%s without token: got %d", m, w.Code)
		}
		if w := doReq(r, m, "sess1", "bogus", ""); w.Code != http.StatusForbidden {
			t.Errorf("%s with bad token: got %d", m, w.Code)
		}
		other := CSRFToken(key, "sess2")
		if w := doReq(r, m, "sess1", other, ""); w.Code != http.StatusForbidden {
			t.Errorf("%s with other session's token: got %d", m, w.Code)
		}
	}
}

func TestCSRFMiddleware_AcceptsHeaderAndFormToken(t *testing.T) {
	key := NewCSRFKey()
	r := csrfRouter(key)
	tok := CSRFToken(key, "sess1")
	if w := doReq(r, "POST", "sess1", tok, ""); w.Code != 200 {
		t.Errorf("header token: got %d", w.Code)
	}
	if w := doReq(r, "POST", "sess1", "", tok); w.Code != 200 {
		t.Errorf("form token: got %d", w.Code)
	}
	if w := doReq(r, "DELETE", "sess1", tok, ""); w.Code != 200 {
		t.Errorf("delete with header token: got %d", w.Code)
	}
}

func TestCSRFMiddleware_NoSessionCookie(t *testing.T) {
	if w := doReq(csrfRouter(NewCSRFKey()), "GET", "", "", ""); w.Code != http.StatusForbidden {
		t.Errorf("got %d", w.Code)
	}
}
