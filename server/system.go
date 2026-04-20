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
		data := s.Profile

		if err := writeJSON(c, data); err != nil {
			return internalError("Failed to compose system profile", err)
		}
		return nil
	})

	g.GET("/status", func(c Context) error {
		ctx := c.Request().Context()
		hostUserType := api.Host
		hostUserFind := api.UserFind{
			Role: &hostUserType,
		}
		hostUser, err := s.Store.FindUser(ctx, &hostUserFind)
		if err != nil && common.ErrorCode(err) != common.NotFound {
			return internalError("Failed to find host user", err)
		}

		if hostUser != nil {
			// data desensitize
			hostUser.OpenID = ""
			hostUser.Email = ""
		}

		systemStatus := api.SystemStatus{
			Host:             hostUser,
			Profile:          *s.Profile,
			DBSize:           0,
			AllowSignUp:      false,
			AdditionalStyle:  "",
			AdditionalScript: "",
			CustomizedProfile: api.CustomizedProfile{
				Name:        "memos",
				LogoURL:     "",
				Description: "",
				Locale:      "en",
				Appearance:  "system",
				ExternalURL: "",
			},
		}

		systemSettingList, err := s.Store.FindSystemSettingList(ctx, &api.SystemSettingFind{})
		if err != nil {
			return internalError("Failed to find system setting list", err)
		}
		for _, systemSetting := range systemSettingList {
			if systemSetting.Name == api.SystemSettingServerID || systemSetting.Name == api.SystemSettingSecretSessionName {
				continue
			}

			var value interface{}
			err := json.Unmarshal([]byte(systemSetting.Value), &value)
			if err != nil {
				return internalError("Failed to unmarshal system setting", err)
			}

			if systemSetting.Name == api.SystemSettingAllowSignUpName {
				systemStatus.AllowSignUp = value.(bool)
			} else if systemSetting.Name == api.SystemSettingAdditionalStyleName {
				systemStatus.AdditionalStyle = value.(string)
			} else if systemSetting.Name == api.SystemSettingAdditionalScriptName {
				systemStatus.AdditionalScript = value.(string)
			} else if systemSetting.Name == api.SystemSettingCustomizedProfileName {
				valueMap := value.(map[string]interface{})
				systemStatus.CustomizedProfile = api.CustomizedProfile{}
				if v := valueMap["name"]; v != nil {
					systemStatus.CustomizedProfile.Name = v.(string)
				}
				if v := valueMap["logoUrl"]; v != nil {
					systemStatus.CustomizedProfile.LogoURL = v.(string)
				}
				if v := valueMap["description"]; v != nil {
					systemStatus.CustomizedProfile.Description = v.(string)
				}
				if v := valueMap["locale"]; v != nil {
					systemStatus.CustomizedProfile.Locale = v.(string)
				}
				if v := valueMap["appearance"]; v != nil {
					systemStatus.CustomizedProfile.Appearance = v.(string)
				}
				if v := valueMap["externalUrl"]; v != nil {
					systemStatus.CustomizedProfile.ExternalURL = v.(string)
				}
			}
		}

		userID, ok := c.Get(getUserIDContextKey()).(int)
		// Get database size for host user.
		if ok {
			user, err := s.Store.FindUser(ctx, &api.UserFind{
				ID: &userID,
			})
			if err != nil {
				return internalError("Failed to find user", err)
			}
			if user != nil && user.Role == api.Host {
				fi, err := os.Stat(s.Profile.DSN)
				if err != nil {
					return internalError("Failed to read database fileinfo", err)
				}
				systemStatus.DBSize = fi.Size()
			}
		}

		if err := writeJSON(c, systemStatus); err != nil {
			return internalError("Failed to encode system status response", err)
		}
		return nil
	})

	g.POST("/system/setting", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}

		user, err := s.Store.FindUser(ctx, &api.UserFind{
			ID: &userID,
		})
		if err != nil {
			return internalError("Failed to find user", err)
		}
		if user == nil || user.Role != api.Host {
			return unauthorizedError("Unauthorized")
		}

		systemSettingUpsert := &api.SystemSettingUpsert{}
		if err := json.NewDecoder(c.Request().Body).Decode(systemSettingUpsert); err != nil {
			return badRequestError("Malformatted post system setting request", err)
		}
		if err := systemSettingUpsert.Validate(); err != nil {
			return badRequestError("Invalid system setting", err)
		}

		systemSetting, err := s.Store.UpsertSystemSetting(ctx, systemSettingUpsert)
		if err != nil {
			return internalError("Failed to upsert system setting", err)
		}

		if err := writeJSON(c, systemSetting); err != nil {
			return internalError("Failed to encode system setting response", err)
		}
		return nil
	})

	g.GET("/system/setting", func(c Context) error {
		ctx := c.Request().Context()
		systemSettingList, err := s.Store.FindSystemSettingList(ctx, &api.SystemSettingFind{})
		if err != nil {
			return internalError("Failed to find system setting list", err)
		}

		if err := writeJSON(c, systemSettingList); err != nil {
			return internalError("Failed to encode system setting list response", err)
		}
		return nil
	})

	g.POST("/system/vacuum", func(c Context) error {
		ctx := c.Request().Context()
		userID, ok := c.Get(getUserIDContextKey()).(int)
		if !ok {
			return unauthorizedError("Missing user in session")
		}
		user, err := s.Store.FindUser(ctx, &api.UserFind{
			ID: &userID,
		})
		if err != nil {
			return internalError("Failed to find user", err)
		}
		if user == nil || user.Role != api.Host {
			return unauthorizedError("Unauthorized")
		}
		if err := s.Store.Vacuum(ctx); err != nil {
			return internalError("Failed to vacuum database", err)
		}
		return c.JSON(http.StatusOK, true)
	})
}

func (s *Server) getSystemServerID(ctx context.Context) (string, error) {
	serverIDKey := api.SystemSettingServerID
	serverIDValue, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{
		Name: &serverIDKey,
	})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return "", err
	}
	if serverIDValue == nil || serverIDValue.Value == "" {
		serverIDValue, err = s.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
			Name:  serverIDKey,
			Value: uuid.NewString(),
		})
		if err != nil {
			return "", err
		}
	}
	return serverIDValue.Value, nil
}

func (s *Server) getSystemSecretSessionName(ctx context.Context) (string, error) {
	secretSessionNameKey := api.SystemSettingSecretSessionName
	secretSessionNameValue, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{
		Name: &secretSessionNameKey,
	})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return "", err
	}
	if secretSessionNameValue == nil || secretSessionNameValue.Value == "" {
		secretSessionNameValue, err = s.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
			Name:  secretSessionNameKey,
			Value: uuid.NewString(),
		})
		if err != nil {
			return "", err
		}
	}
	return secretSessionNameValue.Value, nil
}
