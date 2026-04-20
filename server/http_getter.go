package server

import (
	"fmt"
	"net/http"
	"net/url"

	getter "github.com/usememos/memos/plugin/http_getter"
)

func registerGetterPublicRoutes(g Group) {
	g.GET("/get/httpmeta", func(c Context) error {
		urlStr := c.QueryParam("url")
		if urlStr == "" {
			return badRequestError("Missing website url", nil)
		}
		if _, err := url.Parse(urlStr); err != nil {
			return badRequestError("Wrong url", err)
		}

		htmlMeta, err := getter.GetHTMLMeta(urlStr)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusNotAcceptable, fmt.Sprintf("Failed to get website meta with url: %s", urlStr), err)
		}

		if err := writeJSON(c, htmlMeta); err != nil {
			return internalError("Failed to encode website HTML meta", err)
		}
		return nil
	})

	g.GET("/get/image", func(c Context) error {
		urlStr := c.QueryParam("url")
		if urlStr == "" {
			return badRequestError("Missing image url", nil)
		}
		if _, err := url.Parse(urlStr); err != nil {
			return badRequestError("Wrong url", err)
		}

		image, err := getter.GetImage(urlStr)
		if err != nil {
			return badRequestError(fmt.Sprintf("Failed to get image url: %s", urlStr), err)
		}

		c.Status(http.StatusOK)
		c.Header(headerContentType, image.Mediatype)
		c.Header(headerCacheControl, "max-age=31536000, immutable")
		if _, err := c.Writer().Write(image.Blob); err != nil {
			return internalError("Failed to write image blob", err)
		}
		return nil
	})
}
