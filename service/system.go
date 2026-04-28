package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/common/log"
	"go.uber.org/zap"
)

// GetSystemStatus builds the public system status object.  When an
// authenticated Host user is provided the database file size is also included.
func (s *Service) GetSystemStatus(ctx context.Context, userID *int) (*api.SystemStatus, error) {
	hostUserType := api.Host
	hostUser, err := s.Store.FindUser(ctx, &api.UserFind{Role: &hostUserType})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return nil, fmt.Errorf("failed to find host user: %w", err)
	}
	if hostUser != nil {
		hostUser.OpenID = ""
		hostUser.Email = ""
	}

	systemStatus := &api.SystemStatus{
		Host:               hostUser,
		Profile:            *s.Profile,
		DBSize:             0,
		AllowSignUp:        false,
		IgnoreUpgrade:      false,
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
		StorageServiceID: api.DatabaseStorage,
		LocalStoragePath: "",
	}

	systemSettingList, err := s.Store.FindSystemSettingList(ctx, &api.SystemSettingFind{})
	if err != nil {
		return nil, fmt.Errorf("failed to find system settings: %w", err)
	}
	for _, setting := range systemSettingList {
		if setting.Name == api.SystemSettingServerIDName ||
			setting.Name == api.SystemSettingSecretSessionName ||
			setting.Name == api.SystemSettingOpenAIConfigName {
			continue
		}

		var baseValue any
		if err := json.Unmarshal([]byte(setting.Value), &baseValue); err != nil {
			log.Warn("Failed to unmarshal system setting value", zap.String("setting name", setting.Name.String()))
			continue
		}

		switch setting.Name {
		case api.SystemSettingAllowSignUpName:
			systemStatus.AllowSignUp = baseValue.(bool)
		case api.SystemSettingIgnoreUpgradeName:
			systemStatus.IgnoreUpgrade = baseValue.(bool)
		case api.SystemSettingDisablePublicMemosName:
			systemStatus.DisablePublicMemos = baseValue.(bool)
		case api.SystemSettingAdditionalStyleName:
			systemStatus.AdditionalStyle = baseValue.(string)
		case api.SystemSettingAdditionalScriptName:
			systemStatus.AdditionalScript = baseValue.(string)
		case api.SystemSettingCustomizedProfileName:
			customizedProfile := api.CustomizedProfile{}
			if err := json.Unmarshal([]byte(setting.Value), &customizedProfile); err != nil {
				return nil, fmt.Errorf("failed to unmarshal customized profile: %w", err)
			}
			systemStatus.CustomizedProfile = customizedProfile
		case api.SystemSettingStorageServiceIDName:
			systemStatus.StorageServiceID = int(baseValue.(float64))
		case api.SystemSettingLocalStoragePathName:
			systemStatus.LocalStoragePath = baseValue.(string)
		}
	}

	if userID != nil {
		user, err := s.Store.FindUser(ctx, &api.UserFind{ID: userID})
		if err != nil {
			return nil, fmt.Errorf("failed to find user: %w", err)
		}
		if user != nil && user.Role == api.Host && s.Profile.Driver == "sqlite3" {
			fi, err := os.Stat(s.Profile.DSN)
			if err != nil {
				return nil, fmt.Errorf("failed to read database fileinfo: %w", err)
			}
			systemStatus.DBSize = fi.Size()
		}
	}

	return systemStatus, nil
}

// UpsertSystemSetting validates the payload, checks that the caller is a Host,
// and persists the setting.
func (s *Service) UpsertSystemSetting(ctx context.Context, userID int, upsert *api.SystemSettingUpsert) (*api.SystemSetting, error) {
	user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil || user.Role != api.Host {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}
	if err := upsert.Validate(); err != nil {
		return nil, common.Errorf(common.Invalid, err)
	}
	return s.Store.UpsertSystemSetting(ctx, upsert)
}

// GetSystemSettingList checks that the caller is a Host and returns all settings.
func (s *Service) GetSystemSettingList(ctx context.Context, userID int) ([]*api.SystemSetting, error) {
	user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil || user.Role != api.Host {
		return nil, common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}
	return s.Store.FindSystemSettingList(ctx, &api.SystemSettingFind{})
}

// VacuumDatabase checks that the caller is a Host and runs the vacuum.
func (s *Service) VacuumDatabase(ctx context.Context, userID int) error {
	user, err := s.Store.FindUser(ctx, &api.UserFind{ID: &userID})
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil || user.Role != api.Host {
		return common.Errorf(common.NotAuthorized, fmt.Errorf("unauthorized"))
	}
	return s.Store.Vacuum(ctx)
}

// GetSystemServerID returns (or bootstraps) the server-id system setting.
func (s *Service) GetSystemServerID(ctx context.Context) (string, error) {
	serverIDValue, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingServerIDName})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return "", err
	}
	if serverIDValue == nil || serverIDValue.Value == "" {
		serverIDValue, err = s.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
			Name:  api.SystemSettingServerIDName,
			Value: uuid.NewString(),
		})
		if err != nil {
			return "", err
		}
	}
	return serverIDValue.Value, nil
}

// GetSystemSecretSession returns (or bootstraps) the secret-session system setting.
func (s *Service) GetSystemSecretSession(ctx context.Context) (string, error) {
	secretSessionValue, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingSecretSessionName})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return "", err
	}
	if secretSessionValue == nil || secretSessionValue.Value == "" {
		secretSessionValue, err = s.Store.UpsertSystemSetting(ctx, &api.SystemSettingUpsert{
			Name:  api.SystemSettingSecretSessionName,
			Value: uuid.NewString(),
		})
		if err != nil {
			return "", err
		}
	}
	return secretSessionValue.Value, nil
}

// GetSystemCustomizedProfile returns the customized profile from system settings.
func (s *Service) GetSystemCustomizedProfile(ctx context.Context) (*api.CustomizedProfile, error) {
	systemSetting, err := s.Store.FindSystemSetting(ctx, &api.SystemSettingFind{Name: api.SystemSettingCustomizedProfileName})
	if err != nil && common.ErrorCode(err) != common.NotFound {
		return nil, err
	}
	profile := &api.CustomizedProfile{
		Name:        "memos",
		LogoURL:     "",
		Description: "",
		Locale:      "en",
		Appearance:  "system",
		ExternalURL: "",
	}
	if systemSetting != nil {
		if err := json.Unmarshal([]byte(systemSetting.Value), profile); err != nil {
			return nil, err
		}
	}
	return profile, nil
}
