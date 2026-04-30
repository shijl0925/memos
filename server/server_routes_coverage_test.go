package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	authpkg "github.com/usememos/memos/server/auth"
)

type routeTestClient struct {
	t       *testing.T
	ts      *authTestServer
	cookies map[string]*http.Cookie
}

func newRouteTestClient(t *testing.T, ts *authTestServer) *routeTestClient {
	t.Helper()
	return &routeTestClient{t: t, ts: ts, cookies: map[string]*http.Cookie{}}
}

func (c *routeTestClient) do(method, target string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	c.t.Helper()

	req := httptest.NewRequest(method, target, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	rec := c.ts.serve(req)
	for _, cookie := range rec.Result().Cookies() {
		c.cookies[cookie.Name] = cookie
	}
	return rec
}

func (c *routeTestClient) json(method, target string, payload any) *httptest.ResponseRecorder {
	c.t.Helper()

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		require.NoError(c.t, err)
		body = bytes.NewReader(data)
	}
	return c.do(method, target, body, "application/json")
}

func (c *routeTestClient) upload(target, fieldName, filename string, content []byte) *httptest.ResponseRecorder {
	c.t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(c.t, err)
	_, err = part.Write(content)
	require.NoError(c.t, err)
	require.NoError(c.t, writer.Close())

	rec := c.do(http.MethodPost, target, &body, writer.FormDataContentType())
	return rec
}

func decodeResponseData(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), rec.Body.String())
	if out != nil {
		require.NoError(t, json.Unmarshal(envelope.Data, out), rec.Body.String())
	}
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	require.Equal(t, status, rec.Code, rec.Body.String())
}

func TestServerRoutesCoverage(t *testing.T) {
	ts := newSQLiteAuthTestServer(t)
	client := newRouteTestClient(t, ts)

	var host api.User
	rec := client.json(http.MethodPost, "/api/auth/signup", api.SignUp{
		Username: "hostuser",
		Password: "hostpassword",
	})
	requireStatus(t, rec, http.StatusOK)
	decodeResponseData(t, rec, &host)

	rec = client.json(http.MethodGet, "/api/ping", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/status", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodPost, "/api/system/setting", api.SystemSettingUpsert{
		Name:  api.SystemSettingAdditionalStyleName,
		Value: `"body { color: red; }"`,
	})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/system/setting", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodPost, "/api/system/vacuum", nil)
	requireStatus(t, rec, http.StatusOK)

	var member api.User
	rec = client.json(http.MethodPost, "/api/user", api.UserCreate{
		Username: "member",
		Password: "memberpassword",
		Role:     api.NormalUser,
		Nickname: "Member",
		Email:    "member@example.com",
	})
	requireStatus(t, rec, http.StatusOK)
	decodeResponseData(t, rec, &member)

	rec = client.json(http.MethodGet, "/api/user", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/user/me", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/user/1", nil)
	requireStatus(t, rec, http.StatusOK)

	nickname := "Updated Host"
	rec = client.json(http.MethodPatch, "/api/user/1", api.UserPatch{Nickname: &nickname})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodPost, "/api/user/setting", api.UserSettingUpsert{
		Key:   api.UserSettingLocaleKey,
		Value: `"en"`,
	})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodPost, "/api/tag", api.TagUpsert{Name: "work"})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/tag", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/tag/suggestion", nil)
	requireStatus(t, rec, http.StatusOK)

	var resource api.Resource
	rec = client.upload("/api/resource/blob", "file", "note.txt", []byte("resource body"))
	requireStatus(t, rec, http.StatusOK)
	decodeResponseData(t, rec, &resource)

	var externalResource api.Resource
	rec = client.json(http.MethodPost, "/api/resource", api.ResourceCreate{
		Filename:     "link.txt",
		ExternalLink: "https://example.com/file.txt",
		Type:         "text/plain",
	})
	requireStatus(t, rec, http.StatusOK)
	decodeResponseData(t, rec, &externalResource)

	rec = client.json(http.MethodGet, "/api/resource?limit=10&offset=0", nil)
	requireStatus(t, rec, http.StatusOK)

	renamedResource := "renamed.txt"
	rec = client.json(http.MethodPatch, "/api/resource/"+itoa(resource.ID), api.ResourcePatch{Filename: &renamedResource})
	requireStatus(t, rec, http.StatusOK)

	var memo api.Memo
	rec = client.json(http.MethodPost, "/api/memo", api.MemoCreate{
		Content:        "# First memo\nbody #work",
		Visibility:     api.Public,
		ResourceIDList: []int{resource.ID},
	})
	requireStatus(t, rec, http.StatusOK)
	decodeResponseData(t, rec, &memo)

	updatedContent := "# Updated memo\nbody #work"
	rec = client.json(http.MethodPatch, "/api/memo/"+itoa(memo.ID), api.MemoPatch{Content: &updatedContent})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/memo?creatorId=1&rowStatus=NORMAL&pinned=false&tag=work&visibility=PUBLIC&limit=10&offset=0", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/memo?shortcut=content.contains(%22Updated%22)", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/memo/"+itoa(memo.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodPost, "/api/memo/"+itoa(memo.ID)+"/organizer", api.MemoOrganizerUpsert{Pinned: true})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodPost, "/api/memo/"+itoa(memo.ID)+"/resource", api.MemoResourceUpsert{ResourceID: externalResource.ID})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/memo/"+itoa(memo.ID)+"/resource", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/memo/stats?creatorId=1", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/memo/all?pinned=true&tag=work&text=Updated&visibility=PUBLIC&from=0&to=4102444800&limit=10&offset=0", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/o/r/"+itoa(resource.ID), nil)
	requireStatus(t, rec, http.StatusOK)
	require.NotEmpty(t, rec.Body.String())

	rec = client.json(http.MethodGet, "/o/r/"+itoa(resource.ID)+"/public-id", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/o/r/"+itoa(resource.ID)+"/public-id/"+resource.Filename, nil)
	requireStatus(t, rec, http.StatusOK)

	var shortcut api.Shortcut
	rec = client.json(http.MethodPost, "/api/shortcut", api.ShortcutCreate{
		Title:   "Public work",
		Payload: `content contains "Updated"`,
	})
	requireStatus(t, rec, http.StatusOK)
	decodeResponseData(t, rec, &shortcut)

	shortcutTitle := "Updated shortcut"
	rec = client.json(http.MethodPatch, "/api/shortcut/"+itoa(shortcut.ID), api.ShortcutPatch{Title: &shortcutTitle})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/shortcut", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/shortcut/"+itoa(shortcut.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodDelete, "/api/shortcut/"+itoa(shortcut.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	var storage api.Storage
	rec = client.json(http.MethodPost, "/api/storage", api.StorageCreate{
		Name: "s3",
		Type: api.StorageS3,
		Config: &api.StorageConfig{S3Config: &api.StorageS3Config{
			EndPoint: "https://s3.example.com",
			Region:   "us-east-1",
			Bucket:   "bucket",
		}},
	})
	requireStatus(t, rec, http.StatusOK)
	decodeResponseData(t, rec, &storage)

	storageName := "s3-renamed"
	rec = client.json(http.MethodPatch, "/api/storage/"+itoa(storage.ID), api.StoragePatch{Name: &storageName, Type: api.StorageS3, Config: storage.Config})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/storage", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodDelete, "/api/storage/"+itoa(storage.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	var idp api.IdentityProvider
	idpConfig := &api.IdentityProviderConfig{OAuth2Config: &api.IdentityProviderOAuth2Config{
		ClientID:     "client",
		ClientSecret: "secret",
		AuthURL:      "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		UserInfoURL:  "https://example.com/user",
		Scopes:       []string{"openid"},
		FieldMapping: &api.FieldMapping{
			Identifier:  "sub",
			DisplayName: "name",
			Email:       "email",
		},
	}}
	rec = client.json(http.MethodPost, "/api/idp", api.IdentityProviderCreate{
		Name:   "oidc",
		Type:   api.IdentityProviderOAuth2,
		Config: idpConfig,
	})
	requireStatus(t, rec, http.StatusOK)
	decodeResponseData(t, rec, &idp)

	idpName := "oidc-renamed"
	rec = client.json(http.MethodPatch, "/api/idp/"+itoa(idp.ID), api.IdentityProviderPatch{
		Type:   api.IdentityProviderOAuth2,
		Name:   &idpName,
		Config: idpConfig,
	})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/idp", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/idp/"+itoa(idp.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodDelete, "/api/idp/"+itoa(idp.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/openai/enabled", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodPost, "/api/openai/chat-completion", []any{})
	requireStatus(t, rec, http.StatusBadRequest)

	rec = client.json(http.MethodGet, "/explore/rss.xml", nil)
	requireStatus(t, rec, http.StatusOK)
	require.Contains(t, rec.Body.String(), "Updated memo")

	rec = client.json(http.MethodGet, "/u/1/rss.xml", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/o/get/httpmeta", nil)
	requireStatus(t, rec, http.StatusBadRequest)

	rec = client.json(http.MethodGet, "/o/get/image", nil)
	requireStatus(t, rec, http.StatusBadRequest)

	rec = client.json(http.MethodPost, "/api/tag/delete", api.TagDelete{Name: "work"})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodDelete, "/api/memo/"+itoa(memo.ID)+"/resource/"+itoa(externalResource.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodDelete, "/api/memo/"+itoa(memo.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodDelete, "/api/resource/"+itoa(resource.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodDelete, "/api/resource/"+itoa(externalResource.ID), nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodDelete, "/api/user/"+itoa(member.ID), nil)
	requireStatus(t, rec, http.StatusOK)
}

func TestRSSHelpersCoverage(t *testing.T) {
	profile := &api.CustomizedProfile{Name: "memos", Description: "desc"}
	memos := make([]*api.Memo, MaxRSSItemCount+1)
	for i := range memos {
		memos[i] = &api.Memo{ID: i + 1, Content: "# Title\nDescription", CreatedTs: int64(i)}
	}

	rss, err := generateRSSFromMemoList(memos, "https://example.com", profile)
	require.NoError(t, err)
	require.Contains(t, rss, "Title")
	require.NotContains(t, rss, "/m/101")

	require.Equal(t, "Title", getRSSItemTitle("# Title\nDescription"))
	require.Equal(t, "Description", getRSSItemDescription("# Title\nDescription"))
	require.Equal(t, "", getRSSItemDescription("# Title"))
	require.False(t, isTitleDefined("plain memo"))

	longTitle := string(bytes.Repeat([]byte("a"), MaxRSSItemTitleLength+1))
	require.Equal(t, longTitle[:MaxRSSItemTitleLength]+"...", getRSSItemTitle(longTitle))
	require.Equal(t, longTitle, getRSSItemDescription(longTitle))
}

func TestServerRouteErrorCoverage(t *testing.T) {
	ts := newSQLiteAuthTestServer(t)
	client := newRouteTestClient(t, ts)

	rec := client.json(http.MethodPost, "/api/auth/signup", api.SignUp{
		Username: "hostuser",
		Password: "hostpassword",
	})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/status", nil)
	requireStatus(t, rec, http.StatusOK)

	badJSON := bytes.NewBufferString("{")
	for _, test := range []struct {
		method      string
		target      string
		body        io.Reader
		contentType string
		want        int
	}{
		{http.MethodPost, "/api/auth/signin", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/auth/signin/sso", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/auth/signup", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/system/setting", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/user", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/user/not-a-number", bytes.NewBufferString("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/user/1", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodDelete, "/api/user/not-a-number", nil, "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/memo", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/memo/not-a-number", bytes.NewBufferString("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/memo/1", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodGet, "/api/memo/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodPost, "/api/memo/not-a-number/organizer", bytes.NewBufferString("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/memo/1/organizer", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/memo/not-a-number/resource", bytes.NewBufferString("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/memo/1/resource", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodGet, "/api/memo/not-a-number/resource", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/api/memo/stats", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/api/memo/all?shortcut=unsupported", nil, "", http.StatusBadRequest},
		{http.MethodDelete, "/api/memo/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodDelete, "/api/memo/not-a-number/resource/1", nil, "", http.StatusBadRequest},
		{http.MethodDelete, "/api/memo/1/resource/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodPost, "/api/resource", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/resource/blob", badJSON, "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/resource/not-a-number", bytes.NewBufferString("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/resource/1", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodDelete, "/api/resource/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodPost, "/api/tag", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/tag/delete", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/tag/delete", bytes.NewBufferString(`{"name":"missing"}`), "application/json", http.StatusNotFound},
		{http.MethodPost, "/api/shortcut", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/shortcut/not-a-number", bytes.NewBufferString("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/shortcut/1", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodGet, "/api/shortcut/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodDelete, "/api/shortcut/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodPost, "/api/storage", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/storage/not-a-number", bytes.NewBufferString("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/storage/1", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodDelete, "/api/storage/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodDelete, "/api/storage/999999", nil, "", http.StatusNotFound},
		{http.MethodPost, "/api/idp", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/idp/not-a-number", bytes.NewBufferString("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPatch, "/api/idp/1", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodGet, "/api/idp/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodDelete, "/api/idp/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodDelete, "/api/idp/999999", nil, "", http.StatusNotFound},
		{http.MethodPost, "/api/openai/chat-completion", bytes.NewBufferString("{"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/api/openai/chat-completion", bytes.NewBufferString(`[{"role":"user","content":"hello"}]`), "application/json", http.StatusBadRequest},
		{http.MethodGet, "/u/not-a-number/rss.xml", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/o/r/not-a-number", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/o/r/999999", nil, "", http.StatusNotFound},
		{http.MethodGet, "/o/r/not-a-number/public", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/o/r/not-a-number/public/file.txt", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/o/get/httpmeta?url=:%2f%2f", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/o/get/httpmeta?url=http://127.0.0.1:1", nil, "", http.StatusNotAcceptable},
		{http.MethodGet, "/o/get/image?url=:%2f%2f", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/o/get/image?url=http://127.0.0.1:1/image.png", nil, "", http.StatusBadRequest},
	} {
		rec := client.do(test.method, test.target, test.body, test.contentType)
		requireStatus(t, rec, test.want)
	}
}

func TestAdditionalRouteBranchesCoverage(t *testing.T) {
	ctx := context.Background()
	ts := newSQLiteAuthTestServer(t)
	client := newRouteTestClient(t, ts)

	rec := client.json(http.MethodPost, "/api/auth/signup", api.SignUp{
		Username: "hostuser",
		Password: "hostpassword",
	})
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodGet, "/api/status", nil)
	requireStatus(t, rec, http.StatusOK)

	rec = client.json(http.MethodPost, "/api/auth/signin/sso", api.SSOSignIn{IdentityProviderID: 999999})
	requireStatus(t, rec, http.StatusNotFound)

	rec = client.json(http.MethodPost, "/api/resource", api.ResourceCreate{
		Filename:     "bad-link.txt",
		ExternalLink: "ftp://example.com/file.txt",
		Type:         "text/plain",
	})
	requireStatus(t, rec, http.StatusBadRequest)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.Close())
	rec = client.do(http.MethodPost, "/api/resource/blob", &body, writer.FormDataContentType())
	requireStatus(t, rec, http.StatusBadRequest)

	audioPath := filepath.Join(t.TempDir(), "sound.mp3")
	require.NoError(t, os.WriteFile(audioPath, []byte("audio bytes"), 0644))
	resource, err := ts.Store.CreateResource(ctx, &api.ResourceCreate{
		CreatorID:    1,
		Filename:     "sound.mp3",
		InternalPath: audioPath,
		Type:         "audio/mpeg",
		Size:         int64(len("audio bytes")),
	})
	require.NoError(t, err)
	rec = client.json(http.MethodGet, "/o/r/"+itoa(resource.ID), nil)
	requireStatus(t, rec, http.StatusOK)
	require.Contains(t, rec.Body.String(), "audio bytes")

	idpConfig := &api.IdentityProviderConfig{OAuth2Config: &api.IdentityProviderOAuth2Config{
		ClientID:     "client",
		ClientSecret: "secret",
		AuthURL:      "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		UserInfoURL:  "https://example.com/user",
		FieldMapping: &api.FieldMapping{Identifier: "sub"},
	}}
	idp, err := ts.Service.CreateIdentityProvider(ctx, 1, &api.IdentityProviderCreate{
		Name:   "direct-oidc",
		Type:   api.IdentityProviderOAuth2,
		Config: idpConfig,
	})
	require.NoError(t, err)
	group := newCapturedGroup()
	ts.Server.registerIdentityProviderRoutes(group)
	directCtx := &stubContext{
		request: httptest.NewRequest(http.MethodGet, "/idp/"+itoa(idp.ID), nil),
		values:  map[string]any{getUserIDContextKey(): 1},
		params:  map[string]string{"idpId": itoa(idp.ID)},
	}
	require.NoError(t, group.get["/idp/:idpId"](directCtx))
	require.Equal(t, http.StatusOK, directCtx.status)
}

func TestGetterPublicRoutesServeLocalHTTP(t *testing.T) {
	ts := newSQLiteAuthTestServer(t)
	client := newRouteTestClient(t, ts)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><title>Local title</title><meta property="description" content="Local desc"><meta property="og:image" content="local.png"></head><body></body></html>`))
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	rec := client.json(http.MethodGet, "/o/get/httpmeta?url="+url.QueryEscape(upstream.URL+"/page"), nil)
	requireStatus(t, rec, http.StatusOK)
	require.Contains(t, rec.Body.String(), "Local title")

	rec = client.json(http.MethodGet, "/o/get/image?url="+url.QueryEscape(upstream.URL+"/image.png"), nil)
	requireStatus(t, rec, http.StatusOK)
	require.Equal(t, "image/png", rec.Header().Get(headerContentType))
	require.Equal(t, "png", rec.Body.String())
}

func TestServerStartCreatesActivityAndStartsApp(t *testing.T) {
	ts := newSQLiteAuthTestServer(t)
	app := &fakeApp{}
	ts.Server.app = app

	require.NoError(t, ts.Server.Start(context.Background()))
	require.Equal(t, ":0", app.startAddress)
}

func TestGinUseSessionAndStaticFiles(t *testing.T) {
	app := newTestGinApp(t)
	app.UseSession("secret")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("index"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("file"), 0644))

	app.UseStatic(StaticFileServerConfig{
		HTML5:      true,
		Filesystem: http.Dir(dir),
		Skipper:    DefaultAPIRequestSkipper,
	})
	app.UseStatic(StaticFileServerConfig{
		PathPrefix: "/assets",
		Filesystem: http.Dir(dir),
		Middlewares: []MiddlewareFunc{
			func(next HandlerFunc) HandlerFunc {
				return func(c Context) error {
					c.Header("X-Test-Middleware", "hit")
					return next(c)
				}
			},
		},
	})

	rec := httptest.NewRecorder()
	app.app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/file.txt", nil))
	requireStatus(t, rec, http.StatusOK)
	require.Equal(t, "file", rec.Body.String())

	rec = httptest.NewRecorder()
	app.app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	requireStatus(t, rec, http.StatusOK)
	require.Equal(t, "index", rec.Body.String())

	rec = httptest.NewRecorder()
	app.app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	requireStatus(t, rec, http.StatusNotFound)

	rec = httptest.NewRecorder()
	app.app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/file.txt", nil))
	requireStatus(t, rec, http.StatusOK)
	require.Equal(t, "hit", rec.Header().Get("X-Test-Middleware"))
	require.Equal(t, "file", rec.Body.String())

	app.Group("").GET("/status-only", func(c Context) error {
		c.Status(http.StatusNoContent)
		return nil
	})
	rec = httptest.NewRecorder()
	app.app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status-only", nil))
	requireStatus(t, rec, http.StatusNoContent)
}

func TestHTTPAndJWTAdaptersCoverage(t *testing.T) {
	err := internalError("boom", io.ErrUnexpectedEOF)
	require.EqualError(t, err, "boom")

	var httpErr *httpError
	require.True(t, unwrapHTTPError(err, &httpErr))
	require.Equal(t, http.StatusInternalServerError, httpErr.code)
	require.False(t, unwrapHTTPError(io.ErrUnexpectedEOF, &httpErr))

	ctx := &stubContext{request: httptest.NewRequest(http.MethodGet, "/api/test", nil)}
	require.True(t, DefaultAPIRequestSkipper(ctx))
	require.Equal(t, "192.0.2.1:1234", getClientIP(ctx))

	ctx.request.Header.Set(headerXForwardedFor, "198.51.100.1")
	require.Equal(t, "198.51.100.1", getClientIP(ctx))

	ctx.request.Header.Set(headerXRealIP, "203.0.113.1")
	require.Equal(t, "203.0.113.1", getClientIP(ctx))

	token, err := extractTokenFromHeader(&stubContext{request: httptest.NewRequest(http.MethodGet, "/", nil)})
	require.NoError(t, err)
	require.Empty(t, token)

	authReq := httptest.NewRequest(http.MethodGet, "/", nil)
	authReq.Header.Set("Authorization", "Bearer token")
	token, err = extractTokenFromHeader(&stubContext{request: authReq})
	require.NoError(t, err)
	require.Equal(t, "token", token)

	authReq.Header.Set("Authorization", "Basic token")
	_, err = extractTokenFromHeader(&stubContext{request: authReq})
	require.Error(t, err)

	require.False(t, audienceContains(nil, "missing"))

	header := http.Header{}
	appendVaryHeader(header, "Accept-Encoding")
	appendVaryHeader(header, "Origin")
	require.Equal(t, []string{"Accept-Encoding", "Origin"}, header.Values("Vary"))

	require.Equal(t, "https", requestScheme(httptest.NewRequest(http.MethodGet, "https://example.com", nil)))
	protoReq := httptest.NewRequest(http.MethodGet, "/", nil)
	protoReq.Header.Set("X-Forwarded-Proto", "https")
	require.Equal(t, "https", requestScheme(protoReq))
	schemeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	schemeReq.URL.Scheme = "memos"
	require.Equal(t, "memos", requestScheme(schemeReq))

	require.Equal(t, http.StatusUnauthorized, convertServiceError(common.Errorf(common.NotAuthorized, io.ErrUnexpectedEOF)).(*httpError).code)
	require.Equal(t, http.StatusNotFound, convertServiceError(common.Errorf(common.NotFound, io.ErrUnexpectedEOF)).(*httpError).code)
	require.Equal(t, http.StatusBadRequest, convertServiceError(common.Errorf(common.Invalid, io.ErrUnexpectedEOF)).(*httpError).code)
	require.Equal(t, http.StatusConflict, convertServiceError(common.Errorf(common.Conflict, io.ErrUnexpectedEOF)).(*httpError).code)
	require.Equal(t, http.StatusInternalServerError, convertServiceError(io.ErrUnexpectedEOF).(*httpError).code)
}

func TestJWTMiddlewareCoverage(t *testing.T) {
	ctx := context.Background()
	ts := newSQLiteAuthTestServer(t)
	client := newRouteTestClient(t, ts)

	rec := client.json(http.MethodPost, "/api/auth/signup", api.SignUp{
		Username: "hostuser",
		Password: "hostpassword",
	})
	requireStatus(t, rec, http.StatusOK)

	secret, err := ts.Service.GetSystemSecretSession(ctx)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/user/me", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec = ts.serve(req)
	requireStatus(t, rec, http.StatusUnauthorized)

	token, err := authpkg.GenerateAccessToken("ghost", 999999, secret)
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodGet, "/api/user/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = ts.serve(req)
	requireStatus(t, rec, http.StatusInternalServerError)

	malformedIDClaims := &Claims{
		Name: "ghost",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "not-a-number",
			Audience:  jwt.ClaimStrings{authpkg.AccessTokenAudienceName},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	malformedIDToken := jwt.NewWithClaims(jwt.SigningMethodHS256, malformedIDClaims)
	malformedIDToken.Header["kid"] = "v1"
	token, err = malformedIDToken.SignedString([]byte(secret))
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodGet, "/api/user/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = ts.serve(req)
	requireStatus(t, rec, http.StatusUnauthorized)

	rowStatus := api.Archived
	_, err = ts.Store.PatchUser(ctx, &api.UserPatch{ID: 1, RowStatus: &rowStatus})
	require.NoError(t, err)
	rec = client.json(http.MethodGet, "/api/user/me", nil)
	requireStatus(t, rec, http.StatusForbidden)
}

type fakeApp struct {
	startAddress string
}

func (*fakeApp) Group(string) Group                 { return &fakeGroup{} }
func (*fakeApp) UseLogger(string)                   {}
func (*fakeApp) UseGzip()                           {}
func (*fakeApp) UseCSRF(string, func(Context) bool) {}
func (*fakeApp) UseCORS()                           {}
func (*fakeApp) UseSecure(SecureConfig)             {}
func (*fakeApp) UseTimeout(time.Duration, string)   {}
func (*fakeApp) UseSession(string)                  {}
func (*fakeApp) UseStatic(StaticFileServerConfig)   {}
func (a *fakeApp) Start(address string) error       { a.startAddress = address; return nil }
func (*fakeApp) Shutdown(context.Context) error     { return nil }
func (*fakeGroup) GET(string, HandlerFunc)          {}
func (*fakeGroup) POST(string, HandlerFunc)         {}
func (*fakeGroup) PATCH(string, HandlerFunc)        {}
func (*fakeGroup) DELETE(string, HandlerFunc)       {}
func (*fakeGroup) Use(...MiddlewareFunc)            {}

type fakeGroup struct{}

type capturedGroup struct {
	get    map[string]HandlerFunc
	post   map[string]HandlerFunc
	patch  map[string]HandlerFunc
	delete map[string]HandlerFunc
}

func newCapturedGroup() *capturedGroup {
	return &capturedGroup{
		get:    map[string]HandlerFunc{},
		post:   map[string]HandlerFunc{},
		patch:  map[string]HandlerFunc{},
		delete: map[string]HandlerFunc{},
	}
}

func (g *capturedGroup) GET(path string, handler HandlerFunc)    { g.get[path] = handler }
func (g *capturedGroup) POST(path string, handler HandlerFunc)   { g.post[path] = handler }
func (g *capturedGroup) PATCH(path string, handler HandlerFunc)  { g.patch[path] = handler }
func (g *capturedGroup) DELETE(path string, handler HandlerFunc) { g.delete[path] = handler }
func (*capturedGroup) Use(...MiddlewareFunc)                     {}

type stubContext struct {
	request *http.Request
	values  map[string]any
	params  map[string]string
	writer  http.ResponseWriter
	status  int
}

func (c *stubContext) Request() *http.Request { return c.request }
func (c *stubContext) Writer() http.ResponseWriter {
	if c.writer != nil {
		return c.writer
	}
	return httptest.NewRecorder()
}
func (*stubContext) Cookie(string) (*http.Cookie, error) { return nil, http.ErrNoCookie }
func (*stubContext) SetCookie(*http.Cookie)              {}
func (c *stubContext) JSON(code int, _ any) error        { c.status = code; return nil }
func (*stubContext) String(int, string) error            { return nil }
func (*stubContext) Stream(int, string, io.Reader) error { return nil }
func (c *stubContext) Status(code int)                   { c.status = code }
func (c *stubContext) Header(key, value string) {
	if c.writer != nil {
		c.writer.Header().Set(key, value)
	}
}
func (c *stubContext) Path() string                  { return c.request.URL.Path }
func (c *stubContext) Param(name string) string      { return c.params[name] }
func (c *stubContext) QueryParam(name string) string { return c.request.URL.Query().Get(name) }
func (c *stubContext) Set(key string, value any) {
	if c.values == nil {
		c.values = map[string]any{}
	}
	c.values[key] = value
}
func (c *stubContext) Get(key string) any { return c.values[key] }
func (*stubContext) FormFile(string) (*multipart.FileHeader, error) {
	return nil, http.ErrMissingFile
}
func (c *stubContext) Scheme() string { return requestScheme(c.request) }

func itoa(v int) string {
	return strconv.Itoa(v)
}
