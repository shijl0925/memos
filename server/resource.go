package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/plugin/storage/s3"
)

const (
	maxFileSize = 32 << 20
)

var fileKeyPattern = regexp.MustCompile(`\{[a-z]{1,9}\}`)

func (s *Server) registerResourceRoutes(g Group) {
	g.POST("/resource", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		resourceCreate := &api.ResourceCreate{}
		if err := json.NewDecoder(c.Request().Body).Decode(resourceCreate); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post resource request", err)
		}

		resourceCreate.CreatorID = userID
		if resourceCreate.ExternalLink != "" && !strings.HasPrefix(resourceCreate.ExternalLink, "http") {
			return newHTTPError(http.StatusBadRequest, "Invalid external link")
		}
		if resourceCreate.Visibility == "" {
			userResourceVisibilitySetting, err := s.Store.FindUserSetting(ctx, &api.UserSettingFind{
				UserID: userID,
				Key:    api.UserSettingResourceVisibilityKey,
			})
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user setting", err)
			}

			if userResourceVisibilitySetting != nil {
				resourceVisibility := api.Private
				if err := json.Unmarshal([]byte(userResourceVisibilitySetting.Value), &resourceVisibility); err != nil {
					return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal user setting value", err)
				}
				resourceCreate.Visibility = resourceVisibility
			} else {
				resourceCreate.Visibility = api.Private
			}
		}

		resource, err := s.Store.CreateResource(ctx, resourceCreate)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create resource", err)
		}
		if err := s.createResourceCreateActivity(c, resource); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create activity", err)
		}
		return c.JSON(http.StatusOK, composeResponse(resource))
	})

	g.POST("/resource/blob", func(c Context) error {
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

		filename := file.Filename
		filetype := file.Header.Get("Content-Type")
		size := file.Size
		src, err := file.Open()
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to open file", err)
		}
		defer src.Close()

		systemSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingStorageServiceIDName})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find storage", err)
		}
		storageServiceID := 0
		if systemSetting != nil {
			if err := json.Unmarshal([]byte(systemSetting.Value), &storageServiceID); err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal storage service id", err)
			}
		}

		var resourceCreate *api.ResourceCreate
		if storageServiceID == 0 {
			fileBytes, err := io.ReadAll(src)
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to read file", err)
			}
			resourceCreate = &api.ResourceCreate{
				CreatorID: userID,
				Filename:  filename,
				Type:      filetype,
				Size:      size,
				Blob:      fileBytes,
			}
		} else {
			storage, err := s.Store.FindStorage(ctx, &api.StorageFind{ID: &storageServiceID})
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find storage", err)
			}

			if storage.Type == api.StorageS3 {
				s3Config := storage.Config.S3Config
				t := time.Now()
				var s3FileKey string
				if s3Config.Path == "" {
					s3FileKey = filename
				} else {
					s3FileKey = fileKeyPattern.ReplaceAllStringFunc(s3Config.Path, func(s string) string {
						switch s {
						case "{filename}":
							return filename
						case "{filetype}":
							return filetype
						case "{timestamp}":
							return fmt.Sprintf("%d", t.Unix())
						case "{year}":
							return fmt.Sprintf("%d", t.Year())
						case "{month}":
							return fmt.Sprintf("%02d", t.Month())
						case "{day}":
							return fmt.Sprintf("%02d", t.Day())
						case "{hour}":
							return fmt.Sprintf("%02d", t.Hour())
						case "{minute}":
							return fmt.Sprintf("%02d", t.Minute())
						case "{second}":
							return fmt.Sprintf("%02d", t.Second())
						}
						return s
					})
					if !strings.Contains(s3Config.Path, "{filename}") {
						s3FileKey = path.Join(s3FileKey, filename)
					}
				}

				s3client, err := s3.NewClient(ctx, &s3.Config{
					AccessKey: s3Config.AccessKey,
					SecretKey: s3Config.SecretKey,
					EndPoint:  s3Config.EndPoint,
					Region:    s3Config.Region,
					Bucket:    s3Config.Bucket,
					URLPrefix: s3Config.URLPrefix,
				})
				if err != nil {
					return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to new s3 client", err)
				}

				link, err := s3client.UploadFile(ctx, s3FileKey, filetype, src)
				if err != nil {
					return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to upload via s3 client", err)
				}
				resourceCreate = &api.ResourceCreate{
					CreatorID:    userID,
					Filename:     filename,
					Type:         filetype,
					ExternalLink: link,
				}
			} else {
				return newHTTPError(http.StatusInternalServerError, "Unsupported storage type")
			}
		}

		if resourceCreate.Visibility == "" {
			userResourceVisibilitySetting, err := s.Store.FindUserSetting(ctx, &api.UserSettingFind{
				UserID: userID,
				Key:    api.UserSettingResourceVisibilityKey,
			})
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user setting", err)
			}

			if userResourceVisibilitySetting != nil {
				resourceVisibility := api.Private
				if err := json.Unmarshal([]byte(userResourceVisibilitySetting.Value), &resourceVisibility); err != nil {
					return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal user setting value", err)
				}
				resourceCreate.Visibility = resourceVisibility
			} else {
				resourceCreate.Visibility = api.Private
			}
		}

		resource, err := s.Store.CreateResource(ctx, resourceCreate)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create resource", err)
		}
		if err := s.createResourceCreateActivity(c, resource); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to create activity", err)
		}
		return c.JSON(http.StatusOK, composeResponse(resource))
	})

	g.GET("/resource", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}
		list, err := s.Store.FindResourceList(ctx, &api.ResourceFind{CreatorID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch resource list", err)
		}

		for _, resource := range list {
			memoResourceList, err := s.Store.FindMemoResourceList(ctx, &api.MemoResourceFind{ResourceID: &resource.ID})
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find memo resource list", err)
			}
			resource.LinkedMemoAmount = len(memoResourceList)
		}
		return c.JSON(http.StatusOK, composeResponse(list))
	})

	g.GET("/resource/:resourceId", func(c Context) error {
		ctx := c.Request().Context()
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}
		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{
			ID:        &resourceID,
			CreatorID: &userID,
			GetBlob:   true,
		})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch resource", err)
		}
		return c.JSON(http.StatusOK, composeResponse(resource))
	})

	g.GET("/resource/:resourceId/blob", func(c Context) error {
		ctx := c.Request().Context()
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}
		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{
			ID:        &resourceID,
			CreatorID: &userID,
			GetBlob:   true,
		})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to fetch resource", err)
		}
		return c.Stream(http.StatusOK, resource.Type, bytes.NewReader(resource.Blob))
	})

	g.PATCH("/resource/:resourceId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{ID: &resourceID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find resource", err)
		}
		if resource.CreatorID != userID {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		currentTs := time.Now().Unix()
		resourcePatch := &api.ResourcePatch{UpdatedTs: &currentTs}
		if err := json.NewDecoder(c.Request().Body).Decode(resourcePatch); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted patch resource request", err)
		}

		resourcePatch.ID = resourceID
		resource, err = s.Store.PatchResource(ctx, resourcePatch)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to patch resource", err)
		}
		return c.JSON(http.StatusOK, composeResponse(resource))
	})

	g.DELETE("/resource/:resourceId", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}

		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{ID: &resourceID, CreatorID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find resource", err)
		}
		if resource.CreatorID != userID {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		resourceDelete := &api.ResourceDelete{ID: resourceID}
		if err := s.Store.DeleteResource(ctx, resourceDelete); err != nil {
			if common.ErrorCode(err) == common.NotFound {
				return newHTTPError(http.StatusNotFound, fmt.Sprintf("Resource ID not found: %d", resourceID))
			}
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to delete resource", err)
		}
		return c.JSON(http.StatusOK, true)
	})
}

func (s *Server) registerResourcePublicRoutes(g Group) {
	g.GET("/r/:resourceId/:filename", func(c Context) error {
		ctx := c.Request().Context()
		resourceID, err := strconv.Atoi(c.Param("resourceId"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("ID is not a number: %s", c.Param("resourceId")), err)
		}
		filename, err := url.QueryUnescape(c.Param("filename"))
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, fmt.Sprintf("filename is invalid: %s", c.Param("filename")), err)
		}
		resource, err := s.Store.FindResource(ctx, &api.ResourceFind{
			ID:       &resourceID,
			Filename: &filename,
			GetBlob:  true,
		})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, fmt.Sprintf("Failed to find resource by ID: %v", resourceID), err)
		}

		c.Writer().Header().Set(headerCacheControl, "max-age=31536000, immutable")
		c.Writer().Header().Set(headerContentSecurityPolicy, "default-src 'self'")
		resourceType := strings.ToLower(resource.Type)
		if strings.HasPrefix(resourceType, "text") {
			resourceType = mimeTextPlain
		} else if strings.HasPrefix(resourceType, "video") || strings.HasPrefix(resourceType, "audio") {
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
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: resource.CreatorID,
		Type:      api.ActivityResourceCreate,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return err
}
