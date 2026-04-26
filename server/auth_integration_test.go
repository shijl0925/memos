package server

// Integration tests that exercise the full HTTP auth flow (sign-up, sign-in,
// sign-out) against each supported database backend.
//
// SQLite runs unconditionally using a temporary directory.
// MySQL / PostgreSQL run only when the corresponding environment variable is
// set to a valid DSN:
//
//	MEMOS_TEST_MYSQL_DSN    e.g. "root:password@tcp(localhost:3306)/memos_test"
//	MEMOS_TEST_POSTGRES_DSN e.g. "postgres://user:password@localhost:5432/memos_test?sslmode=disable"
//
// The scenario covered by runAuthFlow:
//  1. GET  /api/status         → host is nil (fresh DB, Sign-In button would not appear)
//  2. POST /api/auth/signup    → first user becomes HOST
//  3. GET  /api/status         → host is now set (Sign-In button would appear)
//  4. POST /api/auth/signin    → correct credentials succeed
//  5. POST /api/auth/signin    → wrong password returns 401
//  6. POST /api/auth/signin    → unknown user returns 401
//  7. POST /api/auth/signup    → duplicate username returns 409
//  8. POST /api/auth/signout   → succeeds

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/server/profile"
	"github.com/usememos/memos/server/version"
)

const (
	envMySQLDSN    = "MEMOS_TEST_MYSQL_DSN"
	envPostgresDSN = "MEMOS_TEST_POSTGRES_DSN"
)

// authTestServer is a thin test helper around *Server that provides
// convenience methods for making HTTP requests and parsing responses.
type authTestServer struct {
	*Server
	handler http.Handler
}

// newSQLiteAuthTestServer creates a Server backed by a fresh temporary
// SQLite database and returns an authTestServer wrapping it.
func newSQLiteAuthTestServer(t *testing.T) *authTestServer {
	t.Helper()
	dir := t.TempDir()
	mode := "prod"
	prof := &profile.Profile{
		Mode:    mode,
		Port:    0,
		Data:    dir,
		DSN:     fmt.Sprintf("%s/memos_%s.db", dir, mode),
		Version: version.GetCurrentVersion(mode),
	}
	return buildAuthTestServer(t, prof)
}

// buildAuthTestServer creates a Server from prof, registers a cleanup hook,
// and returns an authTestServer wrapping it.
func buildAuthTestServer(t *testing.T, prof *profile.Profile) *authTestServer {
	t.Helper()
	ctx := context.Background()
	s, err := NewServer(ctx, prof)
	require.NoError(t, err)
	t.Cleanup(func() { s.Shutdown(context.Background()) })

	ga, ok := s.app.(*ginApp)
	require.True(t, ok, "underlying app must be *ginApp")
	return &authTestServer{Server: s, handler: ga.server.Handler}
}

// cleanNonSQLiteDB deletes all rows from auth-related tables so that tests
// running against a persistent MySQL or PostgreSQL database start clean.
func cleanNonSQLiteDB(t *testing.T, driver, dsn string) {
	t.Helper()
	db, err := sql.Open(driver, dsn)
	require.NoError(t, err, "failed to open %s for cleanup", driver)
	defer db.Close()

	ctx := context.Background()

	// Tables without reserved-word names can be cleaned with a plain DELETE.
	for _, tbl := range []string{
		"memo_relation", "memo_resource", "memo_organizer",
		"resource", "tag", "shortcut", "user_setting",
		"activity", "idp", "memo", "system_setting",
	} {
		if _, err := db.ExecContext(ctx, "DELETE FROM "+tbl); err != nil {
			t.Logf("cleanup warning: DELETE FROM %s: %v", tbl, err)
		}
	}

	// "user" is a reserved word in MySQL and PostgreSQL and needs quoting.
	switch driver {
	case "mysql":
		if _, err := db.ExecContext(ctx, "DELETE FROM `user`"); err != nil {
			t.Logf("cleanup warning: DELETE FROM `user`: %v", err)
		}
	case "postgres":
		if _, err := db.ExecContext(ctx, `DELETE FROM "user"`); err != nil {
			t.Logf(`cleanup warning: DELETE FROM "user": %v`, err)
		}
	}
}

// ---------- Low-level HTTP helpers ----------

func (ts *authTestServer) serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)
	return rec
}

// ---------- Domain-level helpers ----------

func (ts *authTestServer) getStatus(t *testing.T) *api.SystemStatus {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := ts.serve(req)
	require.Equal(t, http.StatusOK, rec.Code, "GET /api/status: unexpected status")

	var resp struct {
		Data *api.SystemStatus `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data
}

func (ts *authTestServer) signUp(t *testing.T, username, password string) *api.User {
	t.Helper()
	return ts.signUpExpect(t, username, password, http.StatusOK)
}

func (ts *authTestServer) signUpExpect(t *testing.T, username, password string, wantStatus int) *api.User {
	t.Helper()
	payload, _ := json.Marshal(api.SignUp{Username: username, Password: password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := ts.serve(req)
	require.Equal(t, wantStatus, rec.Code,
		"POST /api/auth/signup (%s): unexpected status; body: %s", username, rec.Body.String())
	if wantStatus != http.StatusOK {
		return nil
	}
	var resp struct {
		Data *api.User `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data
}

func (ts *authTestServer) signIn(t *testing.T, username, password string) *api.User {
	t.Helper()
	return ts.signInExpect(t, username, password, http.StatusOK)
}

func (ts *authTestServer) signInExpect(t *testing.T, username, password string, wantStatus int) *api.User {
	t.Helper()
	payload, _ := json.Marshal(api.SignIn{Username: username, Password: password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signin", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := ts.serve(req)
	require.Equal(t, wantStatus, rec.Code,
		"POST /api/auth/signin (%s): unexpected status; body: %s", username, rec.Body.String())
	if wantStatus != http.StatusOK {
		return nil
	}
	var resp struct {
		Data *api.User `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data
}

func (ts *authTestServer) signOut(t *testing.T) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signout", nil)
	rec := ts.serve(req)
	require.Equal(t, http.StatusOK, rec.Code, "POST /api/auth/signout: unexpected status")
}

func (ts *authTestServer) csrfCookie(t *testing.T) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := ts.serve(req)
	require.Equal(t, http.StatusOK, rec.Code)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "_csrf" {
			return cookie
		}
	}
	t.Fatal("missing csrf cookie")
	return nil
}

// ---------- Shared auth-flow scenario ----------

// runAuthFlow exercises the complete sign-up / sign-in lifecycle and asserts
// that the host field in /api/status is populated after the first sign-up
// (which is what the frontend uses to decide whether to show the Sign-In button).
func runAuthFlow(t *testing.T, ts *authTestServer) {
	t.Helper()
	ctx := context.Background()

	// 1. Fresh database → no host → frontend would show Sign-Up only.
	status := ts.getStatus(t)
	require.Nil(t, status.Host,
		"expected nil host in a fresh database (Sign-In button should not be visible yet)")

	// 2. First sign-up creates the HOST user.
	user := ts.signUp(t, "alice", "alicepassword123")
	require.Equal(t, "alice", user.Username)

	// 3. After sign-up, /api/status must expose the host so the frontend
	//    can show the Sign-In button (reproduces the reported missing Sign-In bug).
	status = ts.getStatus(t)
	require.NotNil(t, status.Host,
		"expected host to be set after first sign-up (Sign-In button must be visible)")
	require.Equal(t, "alice", status.Host.Username)

	// 4. Sign-In with correct credentials succeeds.
	user = ts.signIn(t, "alice", "alicepassword123")
	require.Equal(t, "alice", user.Username)

	// 5. Sign-In with a wrong password → 401.
	ts.signInExpect(t, "alice", "wrong-password", http.StatusUnauthorized)

	// 6. Sign-In with an unknown username → 401.
	ts.signInExpect(t, "nobody", "anything", http.StatusUnauthorized)

	// 7. Sign-Up with a duplicate username → 409 Conflict.
	//    allowSignUp must be enabled first; otherwise the signup guard returns
	//    401 before even reaching the uniqueness check.
	_, err := ts.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
		Name:  api.SystemSettingAllowSignUpName,
		Value: "true",
	})
	require.NoError(t, err, "enabling allowSignUp")
	ts.signUpExpect(t, "alice", "anotherpassword123", http.StatusConflict)

	// 8. Sign-Out succeeds.
	ts.signOut(t)
}

// ---------- Per-driver test entry-points ----------

func TestAuthFlow_SQLite(t *testing.T) {
	ts := newSQLiteAuthTestServer(t)
	runAuthFlow(t, ts)
}

func TestOpenIDOnlyAuthenticatesMemoCreate(t *testing.T) {
	ts := newSQLiteAuthTestServer(t)
	user := ts.signUp(t, "alice", "alicepassword123")

	payload, _ := json.Marshal(api.MemoCreate{Content: "open api memo"})
	req := httptest.NewRequest(http.MethodPost, "/api/memo?openId="+user.OpenID, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := ts.serve(req)
	require.Equal(t, http.StatusOK, rec.Code, "POST /api/memo with openId should remain supported: %s", rec.Body.String())

	csrfCookie := ts.csrfCookie(t)
	nickname := "hacked"
	patchPayload, _ := json.Marshal(api.UserPatch{Nickname: &nickname})
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/user/%d?openId=%s", user.ID, user.OpenID), bytes.NewReader(patchPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, csrfCookie.Value)
	req.AddCookie(csrfCookie)
	rec = ts.serve(req)
	require.Equal(t, http.StatusUnauthorized, rec.Code, "openId must not authenticate arbitrary APIs")
}

func TestPrivateMemoResourcesRequireVisibility(t *testing.T) {
	ctx := context.Background()
	ts := newSQLiteAuthTestServer(t)
	user := ts.signUp(t, "alice", "alicepassword123")
	resource, err := ts.Service.CreateResource(ctx, user.ID, &api.ResourceCreate{
		Filename: "secret.txt",
		Type:     "text/plain",
		Blob:     []byte("secret"),
	})
	require.NoError(t, err)
	memo, err := ts.Service.CreateMemo(ctx, user.ID, &api.MemoCreate{
		Content:        "private memo",
		Visibility:     api.Private,
		ResourceIDList: []int{resource.ID},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/memo/%d/resource", memo.ID), nil)
	rec := ts.serve(req)
	require.Equal(t, http.StatusUnauthorized, rec.Code, "private memo resources must not be public")

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/o/r/%d", resource.ID), nil)
	rec = ts.serve(req)
	require.Equal(t, http.StatusUnauthorized, rec.Code, "private resource blob must not be public")
}

func TestAuthFlow_MySQL(t *testing.T) {
	dsn := os.Getenv(envMySQLDSN)
	if dsn == "" {
		t.Skipf("skipping MySQL integration test: set %s to a valid DSN to enable", envMySQLDSN)
	}

	cleanNonSQLiteDB(t, "mysql", dsn)

	prof := &profile.Profile{
		Mode:    "prod",
		DSN:     dsn,
		Driver:  "mysql",
		Version: version.GetCurrentVersion("prod"),
	}
	ts := buildAuthTestServer(t, prof)
	runAuthFlow(t, ts)
}

func TestAuthFlow_PostgreSQL(t *testing.T) {
	dsn := os.Getenv(envPostgresDSN)
	if dsn == "" {
		t.Skipf("skipping PostgreSQL integration test: set %s to a valid DSN to enable", envPostgresDSN)
	}

	cleanNonSQLiteDB(t, "postgres", dsn)

	prof := &profile.Profile{
		Mode:    "prod",
		DSN:     dsn,
		Driver:  "postgres",
		Version: version.GetCurrentVersion("prod"),
	}
	ts := buildAuthTestServer(t, prof)
	runAuthFlow(t, ts)
}
