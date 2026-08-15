package appconf

import "strings"

func SanitizeUmbraConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	baseURL := strings.TrimRight(strings.TrimSpace(config.UmbraBaseURL), "/")
	changed := config.UmbraBaseURL != baseURL
	config.UmbraBaseURL = baseURL
	if baseURL == "" && config.UmbraAuthenticated {
		config.UmbraAuthenticated = false
		changed = true
	}
	return changed
}

func SanitizeOneDriveOAuthConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	trimmedClientID := strings.TrimSpace(config.OneDriveClientID)
	changed := config.OneDriveClientID != trimmedClientID
	config.OneDriveClientID = trimmedClientID

	if config.OneDriveClientID == legacyOneDriveDefaultClientID {
		config.OneDriveClientID = ""
		changed = true
		if config.OneDriveRefreshToken != "" {
			config.OneDriveRefreshToken = ""
		}
	}

	return changed
}

func SanitizeBangumiOAuthConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	trimmedAccessToken := strings.TrimSpace(config.BangumiAccessToken)
	trimmedRefreshToken := strings.TrimSpace(config.BangumiRefreshToken)
	trimmedExpiresAt := strings.TrimSpace(config.BangumiTokenExpiresAt)
	trimmedUserID := strings.TrimSpace(config.BangumiAuthorizedUserID)
	trimmedUsername := strings.TrimSpace(config.BangumiAuthorizedUsername)
	trimmedAvatarURL := strings.TrimSpace(config.BangumiAuthorizedAvatarURL)
	trimmedAuthError := strings.TrimSpace(config.BangumiAuthError)

	changed := config.BangumiAccessToken != trimmedAccessToken ||
		config.BangumiRefreshToken != trimmedRefreshToken ||
		config.BangumiTokenExpiresAt != trimmedExpiresAt ||
		config.BangumiAuthorizedUserID != trimmedUserID ||
		config.BangumiAuthorizedUsername != trimmedUsername ||
		config.BangumiAuthorizedAvatarURL != trimmedAvatarURL ||
		config.BangumiAuthError != trimmedAuthError

	config.BangumiAccessToken = trimmedAccessToken
	config.BangumiRefreshToken = trimmedRefreshToken
	config.BangumiTokenExpiresAt = trimmedExpiresAt
	config.BangumiAuthorizedUserID = trimmedUserID
	config.BangumiAuthorizedUsername = trimmedUsername
	config.BangumiAuthorizedAvatarURL = trimmedAvatarURL
	config.BangumiAuthError = trimmedAuthError
	if config.BangumiStatusPushEnabled == nil {
		config.BangumiStatusPushEnabled = boolPtr(true)
		changed = true
	}

	if config.BangumiAccessToken == "" && config.BangumiTokenExpiresAt != "" {
		config.BangumiTokenExpiresAt = ""
		changed = true
	}

	return changed
}

func SanitizeHikarinagiOAuthConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	trimmedAccessToken := strings.TrimSpace(config.HikarinagiAccessToken)
	trimmedRefreshToken := strings.TrimSpace(config.HikarinagiRefreshToken)
	trimmedExpiresAt := strings.TrimSpace(config.HikarinagiTokenExpiresAt)
	trimmedUserID := strings.TrimSpace(config.HikarinagiAuthorizedUserID)
	trimmedUsername := strings.TrimSpace(config.HikarinagiAuthorizedUsername)
	trimmedAvatarURL := strings.TrimSpace(config.HikarinagiAuthorizedAvatarURL)
	trimmedAuthError := strings.TrimSpace(config.HikarinagiAuthError)

	changed := config.HikarinagiAccessToken != trimmedAccessToken ||
		config.HikarinagiRefreshToken != trimmedRefreshToken ||
		config.HikarinagiTokenExpiresAt != trimmedExpiresAt ||
		config.HikarinagiAuthorizedUserID != trimmedUserID ||
		config.HikarinagiAuthorizedUsername != trimmedUsername ||
		config.HikarinagiAuthorizedAvatarURL != trimmedAvatarURL ||
		config.HikarinagiAuthError != trimmedAuthError

	config.HikarinagiAccessToken = trimmedAccessToken
	config.HikarinagiRefreshToken = trimmedRefreshToken
	config.HikarinagiTokenExpiresAt = trimmedExpiresAt
	config.HikarinagiAuthorizedUserID = trimmedUserID
	config.HikarinagiAuthorizedUsername = trimmedUsername
	config.HikarinagiAuthorizedAvatarURL = trimmedAvatarURL
	config.HikarinagiAuthError = trimmedAuthError
	if config.HikarinagiStatusPushEnabled == nil {
		config.HikarinagiStatusPushEnabled = boolPtr(true)
		changed = true
	}

	if config.HikarinagiAccessToken == "" && config.HikarinagiTokenExpiresAt != "" {
		config.HikarinagiTokenExpiresAt = ""
		changed = true
	}

	return changed
}
