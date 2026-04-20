package getter

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetHTMLMeta(t *testing.T) {
	withTestHTTPClient(t, func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "example.com", request.URL.Hostname())

		return newTestResponse(request, "text/html; charset=UTF-8", `
			<html>
				<head>
					<title>ignored title</title>
					<meta property="og:title" content="The SQL Review Tool for Developers" />
					<meta property="og:description" content="Reviewing SQL can be somewhat tedious, yet is essential to keep your database fleet reliable. At Bytebase, we are building a developer-first SQL review tool to empower the DevOps system." />
					<meta property="og:image" content="https://example.com/static/blog/sql-review-tool-for-devs/dev-fighting-dba.webp" />
				</head>
				<body></body>
			</html>
		`), nil
	})

	tests := []struct {
		urlStr   string
		htmlMeta HTMLMeta
	}{
		{
			urlStr: "https://example.com/blog/sql-review-tool-for-devs",
			htmlMeta: HTMLMeta{
				Title:       "The SQL Review Tool for Developers",
				Description: "Reviewing SQL can be somewhat tedious, yet is essential to keep your database fleet reliable. At Bytebase, we are building a developer-first SQL review tool to empower the DevOps system.",
				Image:       "https://example.com/static/blog/sql-review-tool-for-devs/dev-fighting-dba.webp",
			},
		},
	}
	for _, test := range tests {
		metadata, err := GetHTMLMeta(test.urlStr)
		require.NoError(t, err)
		require.Equal(t, test.htmlMeta, *metadata)
	}
}
