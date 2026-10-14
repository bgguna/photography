package main

import (
	"database/sql"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// dropTable simulates a failing database query for one feature. Sessions and
// users are left alone, so the admin stays logged in while the feature errors.
func dropTable(t *testing.T, database *sql.DB, name string) {
	t.Helper()
	if _, err := database.Exec("DROP TABLE " + name); err != nil {
		t.Fatalf("drop %s: %v", name, err)
	}
}

func postForm(r http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAdminRoutes_BadIDs(t *testing.T) {
	a, _ := newAdminClient(t)

	for _, tc := range []struct{ method, path string }{
		{"GET", "/admin/photos/abc/thumb"},
		{"GET", "/admin/photos/abc/web"},
		{"GET", "/admin/photos/abc/original"},
		{"DELETE", "/admin/photos/abc"},
		{"POST", "/admin/photos/abc/visibility"},
		{"POST", "/admin/photos/abc/move"},
		{"POST", "/admin/messages/abc/read"},
		{"POST", "/admin/messages/abc/archive"},
		{"DELETE", "/admin/messages/abc"},
	} {
		var w *httptest.ResponseRecorder
		if tc.method == "GET" {
			w = a.get(tc.path)
		} else {
			w = a.send(tc.method, tc.path, nil, true)
		}
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404", tc.method, tc.path, w.Code)
		}
	}
}

func TestAdminRoutes_PhotoDatabaseFailures(t *testing.T) {
	a, database := newAdminClient(t)
	dropTable(t, database, "photos")

	if w := a.get("/admin"); w.Code != http.StatusOK {
		t.Errorf("dashboard = %d, want 200 with the error logged", w.Code)
	}
	if w := a.get("/admin/photos"); w.Code != http.StatusOK {
		t.Errorf("photos list = %d, want 200 with the error logged", w.Code)
	}
	if w := a.send("POST", "/admin/photos/1/visibility", nil, true); w.Code != http.StatusNotFound {
		t.Errorf("visibility = %d, want 404", w.Code)
	}
	if w := a.send("POST", "/admin/photos/1/move", url.Values{"direction": {"up"}}, true); w.Code != http.StatusBadRequest {
		t.Errorf("move = %d, want 400", w.Code)
	}
	if w := a.send("DELETE", "/admin/photos/1", nil, true); w.Code != http.StatusInternalServerError {
		t.Errorf("delete = %d, want 500", w.Code)
	}
	if w := a.get("/admin/photos/1/thumb"); w.Code != http.StatusNotFound {
		t.Errorf("admin thumb = %d, want 404", w.Code)
	}
}

func TestAdminRoutes_MessageDatabaseFailures(t *testing.T) {
	a, database := newAdminClient(t)
	dropTable(t, database, "contact_messages")

	if w := a.get("/admin/messages"); w.Code != http.StatusOK {
		t.Errorf("messages list = %d, want 200 with the error logged", w.Code)
	}
	for _, path := range []string{"/admin/messages/1/read", "/admin/messages/1/archive"} {
		if w := a.send("POST", path, nil, true); w.Code != http.StatusInternalServerError {
			t.Errorf("POST %s = %d, want 500", path, w.Code)
		}
	}
	if w := a.send("DELETE", "/admin/messages/1", nil, true); w.Code != http.StatusInternalServerError {
		t.Errorf("delete message = %d, want 500", w.Code)
	}
}

func TestAdminRoutes_AboutDatabaseFailures(t *testing.T) {
	a, database := newAdminClient(t)
	dropTable(t, database, "about_page")

	if w := a.get("/admin/about"); w.Code != http.StatusOK {
		t.Errorf("about form = %d, want 200 with the error logged", w.Code)
	}
	if w := a.send("POST", "/admin/about", url.Values{"body": {"Hello"}}, true); w.Code != http.StatusInternalServerError {
		t.Errorf("save about = %d, want 500", w.Code)
	}

	pub := httptest.NewRecorder()
	a.router.ServeHTTP(pub, httptest.NewRequest("GET", "/about", nil))
	if pub.Code != http.StatusInternalServerError {
		t.Errorf("public about = %d, want 500", pub.Code)
	}
}

func TestPublicRoutes_DatabaseFailures(t *testing.T) {
	t.Setenv("PHOTO_STORAGE_PATH", t.TempDir())
	database := openTestDB(t)
	defer database.Close()
	router := setupRouter(database)

	dropTable(t, database, "photos")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("home with photos table missing = %d, want 500", w.Code)
	}
}

func TestPublicRoutes_BadPhotoIDs(t *testing.T) {
	t.Setenv("PHOTO_STORAGE_PATH", t.TempDir())
	database := openTestDB(t)
	defer database.Close()
	router := setupRouter(database)

	for _, path := range []string{"/photos/abc/thumb", "/photos/abc/web"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, w.Code)
		}
	}
}

func TestPublicContact_RateLimited(t *testing.T) {
	t.Setenv("PHOTO_STORAGE_PATH", t.TempDir())
	database := openTestDB(t)
	defer database.Close()
	router := setupRouter(database)

	form := url.Values{"name": {"A"}, "email": {"a@b.co"}, "message": {"hi"}}
	for i := 1; i <= 5; i++ {
		if w := postForm(router, "/contact", form); w.Code != http.StatusOK {
			t.Fatalf("submission %d = %d, want 200", i, w.Code)
		}
	}
	if w := postForm(router, "/contact", form); w.Code != http.StatusTooManyRequests {
		t.Errorf("submission 6 = %d, want 429", w.Code)
	}
}

func TestAdminLogin_RateLimited(t *testing.T) {
	t.Setenv("PHOTO_STORAGE_PATH", t.TempDir())
	database := openTestDB(t)
	defer database.Close()
	router := setupRouter(database)

	bad := url.Values{"email": {"nobody@example.com"}, "password": {"wrong"}}
	for i := 1; i <= 5; i++ {
		w := postForm(router, "/admin/login", bad)
		if loc := w.Header().Get("Location"); loc != "/admin/login?error=invalid+credentials" {
			t.Fatalf("attempt %d redirected to %q", i, loc)
		}
	}
	w := postForm(router, "/admin/login", bad)
	if loc := w.Header().Get("Location"); loc != "/admin/login?error=too+many+login+attempts" {
		t.Errorf("attempt 6 redirected to %q, want the rate-limit message", loc)
	}
}

func TestAdminLogin_MissingFields(t *testing.T) {
	t.Setenv("PHOTO_STORAGE_PATH", t.TempDir())
	database := openTestDB(t)
	defer database.Close()
	router := setupRouter(database)

	w := postForm(router, "/admin/login", url.Values{"email": {"a@b.co"}})
	if loc := w.Header().Get("Location"); loc != "/admin/login?error=email+and+password+required" {
		t.Errorf("redirected to %q", loc)
	}
}

func TestRender_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	render(c, nil, http.StatusOK, "anything", nil)
	if c.Writer.Status() != http.StatusInternalServerError {
		t.Errorf("nil template: status %d, want 500", c.Writer.Status())
	}

	tmpl := template.Must(template.New("page").Parse("ok"))
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	render(c, tmpl, http.StatusOK, "missing-template", nil)
	if c.Writer.Status() != http.StatusInternalServerError {
		t.Errorf("unknown template: status %d, want 500", c.Writer.Status())
	}
}

func TestLoadTemplates_MissingDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	if _, err := loadTemplates(); err == nil {
		t.Error("loadTemplates succeeded without the template directory")
	}
}
