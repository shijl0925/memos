package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	ninja "github.com/shijl0925/gin-ninja"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

const (
	maxFileSize = 32 << 20
)

func (s *Server) registerResourceRoutes(r *ninja.Router) {
	ninja.Post(r, "/resource", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		resourceCreate := &api.ResourceCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(resourceCreate); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post resource request", err)
		}

		resource, err := s.Service.CreateResource(ctx, userID, resourceCreate)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(resource))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Post(r, "/resource/blob", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		if err := c.Request().ParseMultipartForm(maxFileSize); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Upload file overload max size", err)
		}

		file, err := c.FormFile("file")
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to get uploading file", err)
		}
		if file == nil {
			return newHTTPError(http.StatusBadRequest, "Upload file not found")
		}

		sourceFile, err := file.Open()
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to open file", err)
		}
		defer sourceFile.Close()

		resource, err := s.Service.CreateResourceFromBlob(ctx, userID, sourceFile, file)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(resource))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Get(r, "/resource", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		resourceFind := &api.ResourceFind{CreatorID: &userID}
		if limit, err := strconv.Atoi(c.QueryParam("limit")); err == nil {
			resourceFind.Limit = &limit
		}
		if offset, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
			resourceFind.Offset = &offset
		}

		list, err := s.Store.FindResourceList(ctx, resourceFind)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch resource list", err)
		}
		return c.JSON(http.StatusOK, composeResponse(list))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Patch(r, "/resource/:resourceId", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		resourcePatch := &api.ResourcePatch{}
		if err := json.NewDecoder(c.Request().Body).Decode(resourcePatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch resource request", err)
		}

		resource, err := s.Service.UpdateResource(ctx, userID, resourceID, resourcePatch)
		if err != nil {
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, composeResponse(resource))
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Delete(r, "/resource/:resourceId", adaptNinjaVoidHandler(func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		if err := s.Service.DeleteResource(ctx, userID, resourceID); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Resource ID not found: %d", resourceID))
			}
			return convertServiceError(err)
		}
		return c.JSON(http.StatusOK, true)
	}), ninja.ExcludeFromDocs())
}

func (s *Server) registerResourcePublicRoutes(r *ninja.Router) {
	// Helper that streams a resource blob to the client.
	serveResource := func(c Context, resource *api.Resource) error {
		ctx := c.Request().Context()
		var currentUserID *int
		if id, ok := c.Get(getUserIDContextKey()).(int); ok {
			currentUserID = &id
		}
		if err := s.Service.CanAccessResource(ctx, currentUserID, resource.ID); err != nil {
			return convertServiceError(err)
		}

		blob := resource.Blob
		if resource.InternalPath != "" {
			src, err := os.Open(resource.InternalPath)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to open the local resource: %s", resource.InternalPath), err)
			}
			defer src.Close()
			blob, err = io.ReadAll(src)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to read the local resource: %s", resource.InternalPath), err)
			}
		}
		c.Writer().Header().Set(headerCacheControl, "max-age=31536000, immutable")
		c.Writer().Header().Set(headerContentSecurityPolicy, "default-src 'self'")
		resourceType := strings.ToLower(resource.Type)
		if strings.HasPrefix(resourceType, "text") {
			resourceType = mimeTextPlain
		} else if strings.HasPrefix(resourceType, "video") || strings.HasPrefix(resourceType, "audio") {
			http.ServeContent(c.Writer(), c.Request(), resource.Filename, time.Unix(resource.UpdatedTs, 0), bytes.NewReader(blob))
			return nil
		}
		return c.Stream(http.StatusOK, resourceType, bytes.NewReader(blob))
	}

	// Primary route: /o/r/:resourceId (used by the v0.14.4 frontend)
	ninja.Get(r, "/r/:resourceId", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}
		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{ID: &resourceID, GetBlob: true})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to find resource by ID: %v", resourceID), err)
		}
		if resource == nil {
			return newHTTPError(http.StatusNotFound, fmt.Sprintf("Resource not found: %d", resourceID))
		}
		return serveResource(c, resource)
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	// Legacy routes kept for backward compatibility with old URLs that include publicId and filename.
	ninja.Get(r, "/r/:resourceId/:publicId", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}
		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{ID: &resourceID, GetBlob: true})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to find resource by ID: %v", resourceID), err)
		}
		if resource == nil {
			return newHTTPError(http.StatusNotFound, fmt.Sprintf("Resource not found: %d", resourceID))
		}
		return serveResource(c, resource)
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())

	ninja.Get(r, "/r/:resourceId/:publicId/:filename", adaptNinjaHandler(func(c Context) error {
		ctx := c.Request().Context()
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}
		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{ID: &resourceID, GetBlob: true})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to find resource by ID: %v", resourceID), err)
		}
		if resource == nil {
			return newHTTPError(http.StatusNotFound, fmt.Sprintf("Resource not found: %d", resourceID))
		}
		return serveResource(c, resource)
	}), ninja.SuccessStatus(http.StatusOK), ninja.ExcludeFromDocs())
}
