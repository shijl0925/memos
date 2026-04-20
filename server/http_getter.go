package server

import (
	"fmt"
	"net/http"
	"net/url"

	getter "github.com/usememos/memos/plugin/http-getter"
)

func registerGetterPublicRoutes(g Group) {
	g.GET("/get/httpmeta", func(c Context) error {
		urlStr := c.QueryParam("url")
		if urlStr == "" {
			return newHTTPError(http.StatusBadRequest, "Missing website url")
		}
		if _, err := url.Parse(urlStr); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Wrong url", err)
		}

		htmlMeta, err := getter.GetHTMLMeta(urlStr)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusNotAcceptable, fmt.Sprintf("Failed to get website meta with url: %s", urlStr), err)
		}
		return c.JSON(http.StatusOK, composeResponse(htmlMeta))
	})

	g.GET("/get/image", func(c Context) error {
		urlStr := c.QueryParam("url")
		if urlStr == "" {
			return newHTTPError(http.StatusBadRequest, "Missing image url")
		}
		if _, err := url.Parse(urlStr); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Wrong url", err)
		}

		image, err := getter.GetImage(urlStr)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("Failed to get image url: %s", urlStr), err)
		}

		c.Status(http.StatusOK)
		c.Header(headerContentType, image.Mediatype)
		c.Header(headerCacheControl, "max-age=31536000, immutable")
		if _, err := c.Writer().Write(image.Blob); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to write image blob", err)
		}
		return nil
	})
}
