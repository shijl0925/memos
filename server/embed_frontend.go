package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist
var embeddedFiles embed.FS

func getFileSystem(path string) http.FileSystem {
	fs, err := fs.Sub(embeddedFiles, path)
	if err != nil {
		panic(err)
	}

	return http.FS(fs)
}

func embedFrontend(app App) {
	app.UseStatic(StaticFileServerConfig{
		Skipper:    DefaultAPIRequestSkipper,
		HTML5:      true,
		Filesystem: getFileSystem("dist"),
	})

	app.UseStatic(StaticFileServerConfig{
		PathPrefix: "/assets",
		HTML5:      true,
		Filesystem: getFileSystem("dist/assets"),
		Middlewares: []MiddlewareFunc{
			func(next HandlerFunc) HandlerFunc {
				return func(c Context) error {
					c.Header(headerCacheControl, "max-age=31536000, immutable")
					return next(c)
				}
			},
		},
	})
}
