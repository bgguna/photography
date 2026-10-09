package contact

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/bgguna/photography/internal/db"
	"github.com/gin-gonic/gin"
)

func setup(t *testing.T) (*gin.Engine, func() int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	schema, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/msgs", GetMessages(d))
	r.POST("/msgs", HandleNewMsg(d))
	r.POST("/form", HandleNewMsgForm(d))
	r.GET("/closed", func(c *gin.Context) { d.Close(); GetMessages(d)(c) })
	count := func() int {
		var n int
		d.QueryRow("SELECT COUNT(*) FROM contact_messages").Scan(&n)
		return n
	}
	return r, count
}

func do(r http.Handler, method, path, ct, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestJSONHandlers(t *testing.T) {
	r, count := setup(t)

	if w := do(r, "POST", "/msgs", "application/json", "{bad"); w.Code != 400 {
		t.Errorf("bad json = %d", w.Code)
	}
	w := do(r, "POST", "/msgs", "application/json", `{"name":"A","email":"a@b.co","message":"hi"}`)
	if w.Code != 200 || count() != 1 {
		t.Fatalf("post = %d count=%d", w.Code, count())
	}
	w = do(r, "GET", "/msgs", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"A"`) {
		t.Errorf("get = %d %s", w.Code, w.Body.String())
	}
	if w := do(r, "GET", "/closed", "", ""); w.Code != 500 {
		t.Errorf("closed db = %d", w.Code)
	}
	if w := do(r, "POST", "/msgs", "application/json", `{"name":"A","message":"hi"}`); w.Code != 500 {
		t.Errorf("post on closed db = %d", w.Code)
	}
}

func TestFormHandler(t *testing.T) {
	r, count := setup(t)
	const ct = "application/x-www-form-urlencoded"
	post := func(v url.Values) int { return do(r, "POST", "/form", ct, v.Encode()).Code }

	if c := post(url.Values{"website": {"spam"}, "name": {"a"}, "message": {"m"}}); c != 200 || count() != 0 {
		t.Errorf("honeypot = %d count=%d", c, count())
	}
	if c := post(url.Values{"message": {"m"}}); c != 400 {
		t.Errorf("no name = %d", c)
	}
	if c := post(url.Values{"name": {"a"}}); c != 400 {
		t.Errorf("no message = %d", c)
	}
	if c := post(url.Values{"name": {"a"}, "message": {"m"}, "email": {"nope"}}); c != 400 {
		t.Errorf("bad email = %d", c)
	}
	if c := post(url.Values{"name": {"a"}, "message": {"m"}, "email": {"a@b.co"}}); c != 200 {
		t.Errorf("valid = %d", c)
	}
	if c := post(url.Values{"name": {"a"}, "message": {"m"}}); c != 200 || count() != 2 {
		t.Errorf("no email = %d count=%d", c, count())
	}
	// Close DB via the /closed route, then expect a storage failure.
	do(r, "GET", "/closed", "", "")
	if c := post(url.Values{"name": {"a"}, "message": {"m"}}); c != 500 {
		t.Errorf("closed db = %d", c)
	}
}

func TestIsValidEmail(t *testing.T) {
	for e, want := range map[string]bool{"a@b.co": true, "a.b+c@d-e.org": true, "a@b": false, "@b.co": false, "a b@c.co": false} {
		if got := isValidEmail(e); got != want {
			t.Errorf("isValidEmail(%q) = %v", e, got)
		}
	}
}
