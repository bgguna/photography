package main

import (
	"bytes"
	"database/sql"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bgguna/photography/internal/auth"
)

type adminClient struct {
	t      *testing.T
	router *gin.Engine
	cookie *http.Cookie
	csrf   string
}

func (a *adminClient) do(req *http.Request) *httptest.ResponseRecorder {
	if a.cookie != nil {
		req.AddCookie(a.cookie)
	}
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	return w
}

func (a *adminClient) get(path string) *httptest.ResponseRecorder {
	return a.do(httptest.NewRequest("GET", path, nil))
}

func (a *adminClient) send(method, path string, form url.Values, withToken bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	if withToken {
		req.Header.Set("X-CSRF-Token", a.csrf)
	}
	return a.do(req)
}

func (a *adminClient) upload(withToken bool) *httptest.ResponseRecorder {
	img := image.NewRGBA(image.Rect(0, 0, 600, 400))
	for x := 0; x < 600; x++ {
		for y := 0; y < 400; y++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 90, 255})
		}
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		a.t.Fatal(err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="t.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, _ := mw.CreatePart(h)
	part.Write(jpg.Bytes())
	mw.Close()

	req := httptest.NewRequest("POST", "/admin/photos", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("HX-Request", "true")
	if withToken {
		req.Header.Set("X-CSRF-Token", a.csrf)
	}
	return a.do(req)
}

func newAdminClient(t *testing.T) (*adminClient, *sql.DB) {
	t.Helper()
	t.Setenv("PHOTO_STORAGE_PATH", t.TempDir())
	t.Setenv("SESSION_SECRET", "test-secret")
	database := openTestDB(t)
	t.Cleanup(func() { database.Close() })

	if _, err := auth.NewAuthService(database).CreateUser("admin@example.com", "correct-horse-battery", "admin"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	a := &adminClient{t: t, router: setupRouter(database)}

	req := httptest.NewRequest("POST", "/admin/login", strings.NewReader(url.Values{
		"email": {"admin@example.com"}, "password": {"correct-horse-battery"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/admin" {
		t.Fatalf("login failed: %d %s", w.Code, w.Header().Get("Location"))
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "photography_session" {
			a.cookie = c
		}
	}
	if a.cookie == nil {
		t.Fatal("no session cookie")
	}

	page := a.get("/admin/photos")
	m := regexp.MustCompile(`X-CSRF-Token": "([0-9a-f]+)"`).FindStringSubmatch(page.Body.String())
	if page.Code != 200 || m == nil {
		t.Fatalf("photos page: %d, token found: %v", page.Code, m != nil)
	}
	a.csrf = m[1]
	return a, database
}

func publicStatus(a *adminClient, path string) int {
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, httptest.NewRequest("GET", path, nil)) // no cookie: logged-out visitor
	return w.Code
}

func TestAdminFlow_CSRFRejectsMutations(t *testing.T) {
	a, db := newAdminClient(t)

	if w := a.upload(false); w.Code != http.StatusForbidden {
		t.Errorf("upload without token: %d", w.Code)
	}
	if w := a.upload(true); w.Code != 200 {
		t.Fatalf("upload with token: %d %s", w.Code, w.Body.String())
	}

	for _, r := range []struct{ method, path string }{
		{"POST", "/admin/photos/1/visibility"},
		{"POST", "/admin/photos/1/move"},
		{"DELETE", "/admin/photos/1"},
		{"POST", "/admin/messages/1/read"},
		{"POST", "/admin/messages/1/archive"},
		{"DELETE", "/admin/messages/1"},
		{"POST", "/admin/logout"},
	} {
		if w := a.send(r.method, r.path, nil, false); w.Code != http.StatusForbidden {
			t.Errorf("%s %s without token: %d, want 403", r.method, r.path, w.Code)
		}
		if w := a.send(r.method, r.path, nil, false); w.Code == 200 {
			t.Errorf("%s %s should not succeed", r.method, r.path)
		}
	}

	// nothing was changed by the rejected requests
	var public, count int
	db.QueryRow("SELECT is_public, (SELECT COUNT(*) FROM photos) FROM photos WHERE id = 1").Scan(&public, &count)
	if public != 1 || count != 1 {
		t.Errorf("rejected requests mutated state: is_public=%d count=%d", public, count)
	}

	// a token from a different session/key is also rejected
	a.csrf = auth.CSRFToken(auth.NewCSRFKey(), a.cookie.Value)
	if w := a.send("POST", "/admin/photos/1/visibility", nil, true); w.Code != http.StatusForbidden {
		t.Errorf("forged token: %d", w.Code)
	}
}

func TestAdminFlow_PhotoLifecycle(t *testing.T) {
	a, _ := newAdminClient(t)

	// upload two photos; each upload returns the full grid
	if w := a.upload(true); w.Code != 200 || !strings.Contains(w.Body.String(), `id="photo-1"`) {
		t.Fatalf("upload 1: %d %s", w.Code, w.Body.String())
	}
	w := a.upload(true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `id="photo-2"`) {
		t.Fatalf("upload 2: %d %s", w.Code, w.Body.String())
	}
	if publicStatus(a, "/photos/1/thumb") != 200 || publicStatus(a, "/photos/1/web") != 200 {
		t.Error("new photo should be public")
	}

	// hide -> public 404 for logged-out visitors, admin still sees it
	w = a.send("POST", "/admin/photos/1/visibility", nil, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Hidden") || !strings.Contains(w.Body.String(), ">Publish<") {
		t.Errorf("hide response: %d %s", w.Code, w.Body.String())
	}
	if publicStatus(a, "/photos/1/thumb") != 404 || publicStatus(a, "/photos/1/web") != 404 {
		t.Error("hidden photo must 404 publicly")
	}
	if a.get("/admin/photos/1/thumb").Code != 200 {
		t.Error("admin should still see hidden photo")
	}
	if strings.Contains(publicHome(a), "/photos/1/thumb") {
		t.Error("hidden photo must not appear in public gallery")
	}

	// publish again
	w = a.send("POST", "/admin/photos/1/visibility", nil, true)
	if w.Code != 200 || strings.Contains(w.Body.String(), "Hidden") || !strings.Contains(w.Body.String(), ">Hide<") {
		t.Errorf("publish response: %d %s", w.Code, w.Body.String())
	}
	if publicStatus(a, "/photos/1/thumb") != 200 || !strings.Contains(publicHome(a), "/photos/1/thumb") {
		t.Error("published photo should be visible again")
	}

	// reorder: photo 2 up puts it before photo 1 in admin and public
	w = a.send("POST", "/admin/photos/2/move", url.Values{"direction": {"up"}}, true)
	body := w.Body.String()
	if w.Code != 200 || strings.Index(body, `id="photo-2"`) > strings.Index(body, `id="photo-1"`) {
		t.Errorf("move up: %d, order wrong:\n%s", w.Code, body)
	}
	home := publicHome(a)
	if strings.Index(home, "/photos/2/thumb") > strings.Index(home, "/photos/1/thumb") {
		t.Error("public gallery should reflect new order")
	}
	if w := a.send("POST", "/admin/photos/2/move", url.Values{"direction": {"bogus"}}, true); w.Code != http.StatusBadRequest {
		t.Errorf("bogus direction: %d", w.Code)
	}

	// delete removes from admin and public
	w = a.send("DELETE", "/admin/photos/1", nil, true)
	if w.Code != 200 || strings.Contains(w.Body.String(), `id="photo-1"`) || !strings.Contains(w.Body.String(), `id="photo-2"`) {
		t.Errorf("delete: %d %s", w.Code, w.Body.String())
	}
	if publicStatus(a, "/photos/1/thumb") != 404 || a.get("/admin/photos/1/thumb").Code != 404 {
		t.Error("deleted photo should 404 everywhere")
	}

	// bad upload surfaces an error status
	if w := a.send("POST", "/admin/photos", nil, true); w.Code != http.StatusBadRequest {
		t.Errorf("upload without file: %d", w.Code)
	}
}

func publicHome(a *adminClient) string {
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	return w.Body.String()
}

func TestAdminFlow_Messages(t *testing.T) {
	a, db := newAdminClient(t)

	// submit through the public contact form
	form := func(v url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/contact", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		a.router.ServeHTTP(w, req)
		return w
	}
	if w := form(url.Values{"name": {"<script>alert(1)</script>"}, "email": {"v@example.com"}, "message": {"hi <b>there</b>"}}); w.Code != 200 {
		t.Fatalf("contact submit: %d %s", w.Code, w.Body.String())
	}
	if w := form(url.Values{"name": {"bot"}, "message": {"spam"}, "website": {"http://spam"}}); w.Code != 200 {
		t.Errorf("honeypot should look successful: %d", w.Code)
	}
	var n int
	db.QueryRow("SELECT COUNT(*) FROM contact_messages").Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 stored message (honeypot discarded), got %d", n)
	}

	// admin sees it, with user content HTML-escaped
	w := a.get("/admin/messages")
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "1 unread") {
		t.Fatalf("messages page: %d", w.Code)
	}
	if strings.Contains(body, "<script>alert(1)") || strings.Contains(body, "<b>there</b>") {
		t.Error("message content must be HTML-escaped")
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("escaped name not found")
	}

	// mark read -> card re-rendered without the unread state/button
	w = a.send("POST", "/admin/messages/1/read", nil, true)
	if w.Code != 200 || strings.Contains(w.Body.String(), "unread") || strings.Contains(w.Body.String(), "Mark as read") || !strings.Contains(w.Body.String(), `id="msg-1"`) {
		t.Errorf("read: %d %s", w.Code, w.Body.String())
	}

	// archive
	w = a.send("POST", "/admin/messages/1/archive", nil, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "(archived)") || strings.Contains(w.Body.String(), ">Archive<") {
		t.Errorf("archive: %d %s", w.Code, w.Body.String())
	}

	// delete -> empty body so htmx removes the card
	w = a.send("DELETE", "/admin/messages/1", nil, true)
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Errorf("delete: %d %q", w.Code, w.Body.String())
	}
	db.QueryRow("SELECT COUNT(*) FROM contact_messages").Scan(&n)
	if n != 0 {
		t.Error("message not deleted")
	}
}

func TestAdminFlow_PagesRender(t *testing.T) {
	a, _ := newAdminClient(t)
	for _, p := range []string{"/admin", "/admin/photos", "/admin/messages"} {
		w := a.get(p)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `hx-headers`) || !strings.Contains(w.Body.String(), "Photography Admin") {
			t.Errorf("%s: %d", p, w.Code)
		}
	}

	// logout with token ends the session
	if w := a.send("POST", "/admin/logout", nil, true); w.Code != http.StatusFound {
		t.Errorf("logout: %d", w.Code)
	}
	if w := a.get("/admin"); w.Code != http.StatusFound {
		t.Errorf("after logout /admin: %d, want redirect", w.Code)
	}
}
