package getter

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TestGetHTMLMeta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte(`
			<html>
				<head>
					<title>fallback title</title>
					<meta name="description" property="description" content="fallback description">
					<meta property="og:title" content="Open Graph title">
					<meta property="og:description" content="Open Graph description">
					<meta property="og:image" content="https://example.com/image.png">
				</head>
				<body>ignored</body>
			</html>
		`))
		require.NoError(t, err)
	}))
	defer server.Close()

	metadata, err := GetHTMLMeta(server.URL)
	require.NoError(t, err)
	require.Equal(t, HTMLMeta{
		Title:       "Open Graph title",
		Description: "Open Graph description",
		Image:       "https://example.com/image.png",
	}, *metadata)
}

func TestGetHTMLMetaRejectsNonHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"title":"not html"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	_, err := GetHTMLMeta(server.URL)
	require.Error(t, err)
}

func TestGetHTMLMetaRejectsInvalidURL(t *testing.T) {
	_, err := GetHTMLMeta("://bad-url")
	require.Error(t, err)
}

func TestExtractHTMLMetaStopsAtBody(t *testing.T) {
	metadata := extractHTMLMeta(strings.NewReader(`
		<html>
			<head><title>head title</title></head>
			<body>
				<meta property="og:title" content="body title">
			</body>
		</html>
	`))
	require.Equal(t, "head title", metadata.Title)
}

func TestExtractMetaProperty(t *testing.T) {
	token := html.Token{
		DataAtom: atom.Meta,
		Data:     "meta",
		Attr: []html.Attribute{
			{Key: "content", Val: "value"},
			{Key: "property", Val: "og:title"},
		},
	}

	content, ok := extractMetaProperty(token, "og:title")
	require.True(t, ok)
	require.Equal(t, "value", content)

	content, ok = extractMetaProperty(token, "og:description")
	require.False(t, ok)
	require.Equal(t, "value", content)
}
