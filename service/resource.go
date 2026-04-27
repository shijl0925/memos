package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/plugin/storage/s3"
)

// CreateResource persists a resource record (external link or metadata only).
func (s *Service) CreateResource(ctx context.Context, userID int, create *api.ResourceCreate) (*api.Resource, error) {
	if create.ExternalLink != "" && !strings.HasPrefix(create.ExternalLink, "http") {
		return nil, common.Errorf(common.Invalid, fmt.Errorf("invalid external link"))
	}
	create.CreatorID = userID
	resource, err := s.Store.CreateResource(ctx, create)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	if err := s.createResourceCreateActivity(ctx, resource); err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}
	return resource, nil
}

// CreateResourceFromBlob handles file upload, selects the storage backend,
// writes the file and persists the resource record.
func (s *Service) CreateResourceFromBlob(ctx context.Context, userID int, file multipart.File, header *multipart.FileHeader) (*api.Resource, error) {
	filename, err := sanitizeUploadFilename(header.Filename)
	if err != nil {
		return nil, common.Errorf(common.Invalid, err)
	}
	filetype := header.Header.Get("Content-Type")
	size := header.Size

	systemSettingStorageServiceID, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingStorageServiceIDName})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return nil, fmt.Errorf("failed to find storage setting: %w", err)
	}

	storageServiceID := api.DatabaseStorage
	if systemSettingStorageServiceID != nil {
		if err := json.Unmarshal([]byte(systemSettingStorageServiceID.Value), &storageServiceID); err != nil {
			return nil, fmt.Errorf("failed to unmarshal storage service id: %w", err)
		}
	}

	var create *api.ResourceCreate
	if storageServiceID == api.DatabaseStorage {
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		create = &api.ResourceCreate{
			CreatorID: userID,
			Filename:  filename,
			Type:      filetype,
			Size:      size,
			Blob:      fileBytes,
		}
	} else if storageServiceID == api.LocalStorage {
		systemSettingLocalStoragePath, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingLocalStoragePathName})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return nil, fmt.Errorf("failed to find local storage path setting: %w", err)
		}
		localStoragePath := ""
		if systemSettingLocalStoragePath != nil {
			if err := json.Unmarshal([]byte(systemSettingLocalStoragePath.Value), &localStoragePath); err != nil {
				return nil, fmt.Errorf("failed to unmarshal local storage path: %w", err)
			}
		}

		templatePath := localStoragePath
		if !strings.Contains(templatePath, "{filename}") {
			templatePath = path.Join(templatePath, "{filename}")
		}
		resolvedTemplate := replacePathTemplate(templatePath, filename)
		resolvedPath := filepath.Join(s.Profile.Data, filepath.FromSlash(resolvedTemplate))
		dataPath, err := filepath.Abs(s.Profile.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve data directory: %w", err)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve file path: %w", err)
		}
		relPath, err := filepath.Rel(dataPath, resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to validate file path: %w", err)
		}
		if isPathTraversal(relPath) {
			return nil, common.Errorf(common.Invalid, fmt.Errorf("invalid upload path"))
		}
		dir, filename := filepath.Split(resolvedPath)
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
		dst, err := os.Create(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %w", err)
		}
		defer dst.Close()
		if _, err = io.Copy(dst, file); err != nil {
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}
		create = &api.ResourceCreate{
			CreatorID:    userID,
			Filename:     filename,
			Type:         filetype,
			Size:         size,
			InternalPath: resolvedPath,
		}
	} else {
		storage, err := s.Store.FindStorage(ctx, &api.StorageFind{ID: &storageServiceID})
		if err != nil {
			return nil, fmt.Errorf("failed to find storage: %w", err)
		}
		if storage.Type != api.StorageS3 {
			return nil, common.Errorf(common.Invalid, fmt.Errorf("unsupported storage type"))
		}

		s3Config := storage.Config.S3Config
		s3Client, err := s3.NewClient(ctx, &s3.Config{
			AccessKey: s3Config.AccessKey,
			SecretKey: s3Config.SecretKey,
			EndPoint:  s3Config.EndPoint,
			Region:    s3Config.Region,
			Bucket:    s3Config.Bucket,
			URLPrefix: s3Config.URLPrefix,
			URLSuffix: s3Config.URLSuffix,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create s3 client: %w", err)
		}

		filePath := s3Config.Path
		if !strings.Contains(filePath, "{filename}") {
			filePath = path.Join(filePath, "{filename}")
		}
		filePath = replacePathTemplate(filePath, filename)
		_, filename := filepath.Split(filePath)
		link, err := s3Client.UploadFile(ctx, filePath, filetype, file)
		if err != nil {
			return nil, fmt.Errorf("failed to upload via s3: %w", err)
		}
		create = &api.ResourceCreate{
			CreatorID:    userID,
			Filename:     filename,
			Type:         filetype,
			ExternalLink: link,
		}
	}

	resource, err := s.Store.CreateResource(ctx, create)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	if err := s.createResourceCreateActivity(ctx, resource); err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}
	return resource, nil
}

// UpdateResource verifies ownership and applies the patch.
func (s *Service) UpdateResource(ctx context.Context, userID, resourceID int, patch *api.ResourcePatch) (*api.Resource, error) {
	resource, err := s.Store.FindResource(ctx, &api.ResourceFind{ID: &resourceID})
	if err != nil {
		return nil, fmt.Errorf("failed to find resource: %w", err)
	}
	if resource.CreatorID != userID {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}

	currentTs := time.Now().Unix()
	patch.ID = resourceID
	patch.UpdatedTs = &currentTs

	return s.Store.PatchResource(ctx, patch)
}

// DeleteResource verifies ownership, removes any local file, and deletes the record.
func (s *Service) DeleteResource(ctx context.Context, userID, resourceID int) error {
	resource, err := s.Store.FindResource(ctx, &api.ResourceFind{ID: &resourceID, CreatorID: &userID})
	if err != nil {
		return fmt.Errorf("failed to find resource: %w", err)
	}
	if resource.CreatorID != userID {
		return common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}
	if resource.InternalPath != "" {
		if err := os.Remove(resource.InternalPath); err != nil {
			// Log but do not fail: the DB record must still be removed.
			_ = err
		}
	}
	return s.Store.DeleteResource(ctx, &api.ResourceDelete{ID: resourceID})
}

// replacePathTemplate substitutes {filename}, {timestamp}, {year}, {month},
// {day} and {uuid} placeholders in a storage path template.
func replacePathTemplate(template, filename string) string {
	t := time.Now()
	template = strings.ReplaceAll(template, "{filename}", filename)
	template = strings.ReplaceAll(template, "{timestamp}", fmt.Sprintf("%d", t.Unix()))
	template = strings.ReplaceAll(template, "{year}", fmt.Sprintf("%d", t.Year()))
	template = strings.ReplaceAll(template, "{month}", fmt.Sprintf("%02d", t.Month()))
	template = strings.ReplaceAll(template, "{day}", fmt.Sprintf("%02d", t.Day()))
	template = strings.ReplaceAll(template, "{uuid}", common.GenUUID())
	return template
}

func sanitizeUploadFilename(filename string) (string, error) {
	filename = path.Base(strings.ReplaceAll(filename, "\\", "/"))
	isEmptyOrSpecial := filename == "" || filename == "." || filename == ".."
	hasNullByte := strings.ContainsRune(filename, 0)
	if isEmptyOrSpecial || hasNullByte || isWindowsReservedFilename(filename) {
		return "", fmt.Errorf("invalid filename")
	}
	for _, r := range filename {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("invalid filename")
		}
	}
	return filename, nil
}

// isWindowsReservedFilename rejects Windows device names on every platform so
// uploaded files remain portable and cannot target special device paths.
func isWindowsReservedFilename(filename string) bool {
	name := strings.ToUpper(filename)
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		name = name[:dot]
	}
	switch name {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9' {
		return true
	}
	return false
}

// isPathTraversal detects paths that escape the configured storage directory.
func isPathTraversal(relPath string) bool {
	return relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath)
}
