package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	metric "github.com/usememos/memos/plugin/metrics"
)

const (
	// The max file size is 32MB.
	maxFileSize = 32 << 20
)

func (s *Server) registerResourceRoutes(g Group) {
	g.POST("/resource", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}

		resourceCreate := &api.ResourceCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(resourceCreate); err != nil {
			return badRequestError("Malformatted post resource request", err)
		}

		resourceCreate.CreatorID = userID
		resource, err := s.Store.CreateResource(ctx, resourceCreate)
		if err != nil {
			return internalError("Failed to create resource", err)
		}
		if err := s.createResourceCreateActivity(c, resource); err != nil {
			return internalError("Failed to create activity", err)
		}

		if err := writeJSON(c, resource); err != nil {
			return internalError("Failed to encode resource response", err)
		}
		return nil
	})

	g.POST("/resource/blob", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}

		if err := c.Request().ParseMultipartForm(maxFileSize); err != nil {
			return badRequestError("Upload file overload max size", err)
		}

		file, err := c.FormFile("file")
		if err != nil {
			return internalError("Failed to get uploading file", err)
		}
		if file == nil {
			return badRequestError("Upload file not found", nil)
		}

		filename := file.Filename
		filetype := file.Header.Get("Content-Type")
		size := file.Size
		src, err := file.Open()
		if err != nil {
			return internalError("Failed to open file", err)
		}
		defer src.Close()

		fileBytes, err := io.ReadAll(src)
		if err != nil {
			return internalError("Failed to read file", err)
		}

		resourceCreate := &api.ResourceCreate{
			CreatorID: userID,
			Filename:  filename,
			Type:      filetype,
			Size:      size,
			Blob:      fileBytes,
		}
		resource, err := s.Store.CreateResource(ctx, resourceCreate)
		if err != nil {
			return internalError("Failed to create resource", err)
		}
		if err := s.createResourceCreateActivity(c, resource); err != nil {
			return internalError("Failed to create activity", err)
		}

		if err := writeJSON(c, resource); err != nil {
			return internalError("Failed to encode resource response", err)
		}
		return nil
	})

	g.GET("/resource", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		resourceFind := &api.ResourceFind{
			CreatorID: &userID,
		}
		list, err := s.Store.FindResourceList(ctx, resourceFind)
		if err != nil {
			return internalError("Failed to fetch resource list", err)
		}

		for _, resource := range list {
			memoResourceList, err := s.Store.FindMemoResourceList(ctx, &api.MemoResourceFind{
				ResourceID: &resource.ID,
			})
			if err != nil {
				return internalError("Failed to find memo resource list", err)
			}
			resource.LinkedMemoAmount = len(memoResourceList)
		}

		if err := writeJSON(c, list); err != nil {
			return internalError("Failed to encode resource list response", err)
		}
		return nil
	})

	g.GET("/resource/:resourceId", func(c Context) error {
		ctx := c.Request().Context()
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		resourceFind := &api.ResourceFind{
			ID:        &resourceID,
			CreatorID: &userID,
			GetBlob:   true,
		}
		resource, err := s.Store.FindResource(ctx, resourceFind)
		if err != nil {
			return internalError("Failed to fetch resource", err)
		}

		if err := writeJSON(c, resource); err != nil {
			return internalError("Failed to encode resource response", err)
		}
		return nil
	})

	g.GET("/resource/:resourceId/blob", func(c Context) error {
		ctx := c.Request().Context()
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		resourceFind := &api.ResourceFind{
			ID:        &resourceID,
			CreatorID: &userID,
			GetBlob:   true,
		}
		resource, err := s.Store.FindResource(ctx, resourceFind)
		if err != nil {
			return internalError("Failed to fetch resource", err)
		}

		c.Status(http.StatusOK)
		c.Header(headerContentType, resource.Type)
		c.Header(headerContentSecurityPolicy, "default-src 'self'")
		if _, err := c.Writer().Write(resource.Blob); err != nil {
			return internalError("Failed to write resource blob", err)
		}
		return nil
	})

	g.PATCH("/resource/:resourceId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}

		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		resourceFind := &api.ResourceFind{
			ID: &resourceID,
		}
		resource, err := s.Store.FindResource(ctx, resourceFind)
		if err != nil {
			return internalError("Failed to find resource", err)
		}
		if resource.CreatorID != userID {
			return unauthorizedError("Unauthorized")
		}

		currentTs := time.Now().Unix()
		resourcePatch := &api.ResourcePatch{
			UpdatedTs: &currentTs,
		}
		if err := json.NewDecoder(c.Request().Body).Decode(resourcePatch); err != nil {
			return badRequestError("Malformatted patch resource request", err)
		}

		resourcePatch.ID = resourceID
		resource, err = s.Store.PatchResource(ctx, resourcePatch)
		if err != nil {
			return internalError("Failed to patch resource", err)
		}

		if err := writeJSON(c, resource); err != nil {
			return internalError("Failed to encode resource response", err)
		}
		return nil
	})

	g.DELETE("/resource/:resourceId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}

		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{
			ID:        &resourceID,
			CreatorID: &userID,
		})
		if err != nil {
			return internalError("Failed to find resource", err)
		}
		if resource.CreatorID != userID {
			return unauthorizedError("Unauthorized")
		}

		resourceDelete := &api.ResourceDelete{
			ID: resourceID,
		}
		if err := s.Store.DeleteResource(ctx, resourceDelete); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return notFoundError(fmt.Sprintf("Resource ID not found: %d", resourceID), nil)
			}
			return internalError("Failed to delete resource", err)
		}

		return c.JSON(http.StatusOK, true)
	})
}

func (s *Server) registerResourcePublicRoutes(g Group) {
	g.GET("/r/:resourceId/:filename", func(c Context) error {
		ctx := c.Request().Context()
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return badRequestError(fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}
		filename, err := url.QueryUnescape(c.Param("filename"))
		if err != nil {
			return badRequestError(fmt.Sprintf("filename is invalid: %s", c.Param("filename")), err)
		}
		resourceFind := &api.ResourceFind{
			ID:       &resourceID,
			Filename: &filename,
			GetBlob:  true,
		}
		resource, err := s.Store.FindResource(ctx, resourceFind)
		if err != nil {
			return internalError(fmt.Sprintf("Failed to fetch resource ID: %v", resourceID), err)
		}

		resourceType := strings.ToLower(resource.Type)
		if strings.HasPrefix(resourceType, "text") || (strings.HasPrefix(resourceType, "application") && resourceType != "application/pdf") {
			resourceType = mimeTextPlain
		}
		c.Writer().Header().Set(headerCacheControl, "max-age=31536000, immutable")
		c.Writer().Header().Set(headerContentSecurityPolicy, "default-src 'self'")
		if strings.HasPrefix(resourceType, "video") || strings.HasPrefix(resourceType, "audio") {
			http.ServeContent(c.Writer(), c.Request(), resource.Filename, time.Unix(resource.UpdatedTs, 0), bytes.NewReader(resource.Blob))
			return nil
		}
		return c.Stream(http.StatusOK, resourceType, bytes.NewReader(resource.Blob))
	})
}

func (s *Server) createResourceCreateActivity(c Context, resource *api.Resource) error {
	ctx := c.Request().Context()
	payload := api.ActivityResourceCreatePayload{
		Filename: resource.Filename,
		Type:     resource.Type,
		Size:     resource.Size,
	}
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: resource.CreatorID,
		Type:      api.ActivityResourceCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadStr),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	s.Collector.Collect(ctx, &metric.Metric{
		Name: string(activity.Type),
	})
	return err
}
