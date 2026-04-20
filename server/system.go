package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
)

func (s *Server) registerSystemRoutes(g Group) {
	g.GET("/ping", func(c Context) error {
		return c.JSON(http.StatusOK, composeResponse(s.Profile))
	})

	g.GET("/status", func(c Context) error {
		ctx := c.Request().Context()
		hostUserType := api.Host
		hostUser, err := s.Store.FindUser(ctx, &api.UserFind{Role: &hostUserType})
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find host user", err)
		}

		if hostUser != nil {
			hostUser.OpenID = ""
			hostUser.Email = ""
		}

		systemStatus := api.SystemStatus{
			Host:               hostUser,
			Profile:            *s.Profile,
			DBSize:             0,
			AllowSignUp:        false,
			DisablePublicMemos: false,
			AdditionalStyle:    "",
			AdditionalScript:   "",
			CustomizedProfile: api.CustomizedProfile{
				Name:        "memos",
				LogoURL:     "",
				Description: "",
				Locale:      "en",
				Appearance:  "system",
				ExternalURL: "",
			},
			StorageServiceID: 0,
		}

		systemSettingList, err := s.Store.FindSystemSettingList(ctx, &api.SystemSettingFind{})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find system setting list", err)
		}
		for _, systemSetting := range systemSettingList {
			if systemSetting.Name == api.SystemSettingServerID || systemSetting.Name == api.SystemSettingSecretSessionName || systemSetting.Name == api.SystemSettingOpenAIConfigName {
				continue
			}

			var baseValue interface{}
			if err := json.Unmarshal([]byte(systemSetting.Value), &baseValue); err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal system setting value", err)
			}

			switch systemSetting.Name {
			case api.SystemSettingAllowSignUpName:
				systemStatus.AllowSignUp = baseValue.(bool)
			case api.SystemSettingDisablePublicMemosName:
				systemStatus.DisablePublicMemos = baseValue.(bool)
			case api.SystemSettingAdditionalStyleName:
				systemStatus.AdditionalStyle = baseValue.(string)
			case api.SystemSettingAdditionalScriptName:
				systemStatus.AdditionalScript = baseValue.(string)
			case api.SystemSettingCustomizedProfileName:
				customizedProfile := api.CustomizedProfile{}
				if err := json.Unmarshal([]byte(systemSetting.Value), &customizedProfile); err != nil {
					return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to unmarshal system setting customized profile value", err)
				}
				systemStatus.CustomizedProfile = customizedProfile
			case api.SystemSettingStorageServiceIDName:
				systemStatus.StorageServiceID = int(baseValue.(float64))
			}
		}

		if userID, ok := c.Get(getUserIDContextKey()).(int); ok {
			user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
			if err != nil {
				return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
			}
			if user != nil && user.Role == api.Host {
				fi, err := os.Stat(s.Profile.DSN)
				if err != nil {
					return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to read database fileinfo", err)
				}
				systemStatus.DBSize = fi.Size()
			}
		}
		return c.JSON(http.StatusOK, composeResponse(systemStatus))
	})

	g.POST("/system/setting", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}
		if user == nil || user.Role != api.Host {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		systemSettingUpsert := &api.SystemSettingUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(systemSettingUpsert); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "Malformatted post system setting request", err)
		}
		if err := systemSettingUpsert.Validate(); err != nil {
			return newHTTPErrorWithInternal(http.StatusBadRequest, "system setting invalidate", err)
		}

		systemSetting, err := s.Store.UpsertSystemSetting(ctx, systemSettingUpsert)
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to upsert system setting", err)
		}
		return c.JSON(http.StatusOK, composeResponse(systemSetting))
	})

	g.GET("/system/setting", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}
		if user == nil || user.Role != api.Host {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		systemSettingList, err := s.Store.FindSystemSettingList(ctx, &api.SystemSettingFind{})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find system setting list", err)
		}
		return c.JSON(http.StatusOK, composeResponse(systemSettingList))
	})

	g.POST("/system/vacuum", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return newHTTPError(http.StatusUnauthorized, "Missing user in session")
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
		if err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to find user", err)
		}
		if user == nil || user.Role != api.Host {
			return newHTTPError(http.StatusUnauthorized, "Unauthorized")
		}

		if err := s.Store.Vacuum(ctx); err != nil {
			return newHTTPErrorWithInternal(http.StatusInternalServerError, "Failed to vacuum database", err)
		}
		return c.JSON(http.StatusOK, true)
	})
}

func (s *Server) getSystemServerID(ctx context.Context) (string, error) {
	serverIDValue, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingServerID})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return "", err
	}
	if serverIDValue == nil || serverIDValue.Value == "" {
		serverIDValue, err = s.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
			Name:  api.SystemSettingServerID,
			Value: uuid.NewString(),
		})
		if err != nil {
			return "", err
		}
	}
	return serverIDValue.Value, nil
}

func (s *Server) getSystemSecretSessionName(ctx context.Context) (string, error) {
	secretSessionNameValue, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingSecretSessionName})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return "", err
	}
	if secretSessionNameValue == nil || secretSessionNameValue.Value == "" {
		secretSessionNameValue, err = s.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
			Name:  api.SystemSettingSecretSessionName,
			Value: uuid.NewString(),
		})
		if err != nil {
			return "", err
		}
	}
	return secretSessionNameValue.Value, nil
}
