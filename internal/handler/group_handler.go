// Package handler provides HTTP handlers for the application
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/utils"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gpt-load/internal/channel"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

// getChannelEndpoint returns the API endpoint path for a given channel type
func (s *Server) getChannelEndpoint(channelType string) string {
	switch channelType {
	case "openai":
		return "/v1/chat/completions"
	case "anthropic":
		return "/v1/messages?beta=true"
	case "gemini":
		return "/v1beta/models/"
	default:
		return ""
	}
}

// isValidChannelType checks if the channel type is valid by checking against the registered channels.
func isValidChannelType(channelType string) bool {
	channels := channel.GetChannels()
	for _, t := range channels {
		if t == channelType {
			return true
		}
	}
	return false
}

// UpstreamDefinition defines the structure for an upstream in the request.
type UpstreamDefinition struct {
	URL    string `json:"url"`
	Weight int    `json:"weight"`
}

// validateAndCleanUpstreams validates and cleans the upstreams JSON.
func validateAndCleanUpstreams(upstreams json.RawMessage) (datatypes.JSON, error) {
	if len(upstreams) == 0 {
		return nil, fmt.Errorf("upstreams field is required")
	}

	var defs []UpstreamDefinition
	if err := json.Unmarshal(upstreams, &defs); err != nil {
		return nil, fmt.Errorf("invalid format for upstreams: %w", err)
	}

	if len(defs) == 0 {
		return nil, fmt.Errorf("at least one upstream is required")
	}

	for i := range defs {
		defs[i].URL = strings.TrimSpace(defs[i].URL)
		if defs[i].URL == "" {
			return nil, fmt.Errorf("upstream URL cannot be empty")
		}
		// Basic URL format validation
		if !strings.HasPrefix(defs[i].URL, "http://") && !strings.HasPrefix(defs[i].URL, "https://") {
			return nil, fmt.Errorf("invalid URL format for upstream: %s", defs[i].URL)
		}
		if defs[i].Weight <= 0 {
			return nil, fmt.Errorf("upstream weight must be a positive integer")
		}
	}

	cleanedUpstreams, err := json.Marshal(defs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cleaned upstreams: %w", err)
	}

	return cleanedUpstreams, nil
}

// isValidGroupName checks if the group name is valid.
func isValidGroupName(name string) bool {
	if name == "" {
		return false
	}
	// 允许使用小写字母、数字、下划线和中划线，长度在 3 到 30 个字符之间
	match, _ := regexp.MatchString("^[a-z0-9_-]{3,30}$", name)
	return match
}

// isValidValidationEndpoint checks if the validation endpoint is a valid path.
func isValidValidationEndpoint(endpoint string) bool {
	if endpoint == "" {
		return true
	}
	if !strings.HasPrefix(endpoint, "/") {
		return false
	}
	if strings.Contains(endpoint, "://") {
		return false
	}
	return true
}

// validateAndCleanConfig validates the group config against the GroupConfig struct and system-defined rules.
func (s *Server) validateAndCleanConfig(configMap map[string]any) (map[string]any, error) {
	if configMap == nil {
		return nil, nil
	}

	// 1. Check for unknown fields by comparing against the GroupConfig struct definition.
	var tempGroupConfig models.GroupConfig
	groupConfigType := reflect.TypeOf(tempGroupConfig)
	validFields := make(map[string]bool)
	for i := 0; i < groupConfigType.NumField(); i++ {
		jsonTag := groupConfigType.Field(i).Tag.Get("json")
		fieldName := strings.Split(jsonTag, ",")[0]
		if fieldName != "" && fieldName != "-" {
			validFields[fieldName] = true
		}
	}

	for key := range configMap {
		if !validFields[key] {
			return nil, fmt.Errorf("unknown config field: '%s'", key)
		}
	}

	// 2. Validate the values of the provided fields using the central system settings validator.
	if err := s.SettingsManager.ValidateGroupConfigOverrides(configMap); err != nil {
		return nil, err
	}

	// 3. Unmarshal and marshal back to clean the map and ensure correct types.
	configBytes, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config map: %w", err)
	}

	var validatedConfig models.GroupConfig
	if err := json.Unmarshal(configBytes, &validatedConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into validated config: %w", err)
	}

	validatedBytes, err := json.Marshal(validatedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validated config: %w", err)
	}
	var finalMap map[string]any
	if err := json.Unmarshal(validatedBytes, &finalMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into final map: %w", err)
	}

	return finalMap, nil
}

// GroupCreateRequest defines the payload for creating a group.
type GroupCreateRequest struct {
	Name               string              `json:"name"`
	DisplayName        string              `json:"display_name"`
	Description        string              `json:"description"`
	CodeSnippet        string              `json:"code_snippet"`
	Upstreams          json.RawMessage     `json:"upstreams"`
	ChannelType        string              `json:"channel_type"`
	Sort               int                 `json:"sort"`
	TestModel          string              `json:"test_model"`
	ValidationEndpoint string              `json:"validation_endpoint"`
	ParamOverrides     map[string]any      `json:"param_overrides"`
	Config             map[string]any      `json:"config"`
	HeaderRules        []models.HeaderRule `json:"header_rules"`
	ProxyKeys          string              `json:"proxy_keys"`
	ForceHTTP11        *bool               `json:"force_http11,omitempty"`
}

// CreateGroup handles the creation of a new group.
func (s *Server) CreateGroup(c *gin.Context) {
	var req GroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// Data Cleaning and Validation
	name := strings.TrimSpace(req.Name)
	if !isValidGroupName(name) {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "无效的分组名称。只能包含小写字母、数字、中划线或下划线，长度3-30位"))
		return
	}

	channelType := strings.TrimSpace(req.ChannelType)
	if !isValidChannelType(channelType) {
		supported := strings.Join(channel.GetChannels(), ", ")
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Invalid channel type. Supported types are: %s", supported)))
		return
	}

	testModel := strings.TrimSpace(req.TestModel)
	if testModel == "" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Test model is required"))
		return
	}

	cleanedUpstreams, err := validateAndCleanUpstreams(req.Upstreams)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
		return
	}

	cleanedConfig, err := s.validateAndCleanConfig(req.Config)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Invalid config format: %v", err)))
		return
	}

	validationEndpoint := strings.TrimSpace(req.ValidationEndpoint)
	if !isValidValidationEndpoint(validationEndpoint) {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "无效的测试路径。如果提供，必须是以 / 开头的有效路径，且不能是完整的URL。"))
		return
	}

	// Validate and normalize header rules if provided
	var headerRulesJSON datatypes.JSON
	if len(req.HeaderRules) > 0 {
		normalizedHeaderRules := make([]models.HeaderRule, 0)
		seenKeys := make(map[string]bool)

		for _, rule := range req.HeaderRules {
			key := strings.TrimSpace(rule.Key)
			if key == "" {
				continue
			}

			// Normalize to canonical form
			canonicalKey := http.CanonicalHeaderKey(key)

			// Check for duplicate keys
			if seenKeys[canonicalKey] {
				response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Duplicate header key: %s", canonicalKey)))
				return
			}
			seenKeys[canonicalKey] = true

			normalizedHeaderRules = append(normalizedHeaderRules, models.HeaderRule{
				Key:    canonicalKey,
				Value:  rule.Value,
				Action: rule.Action,
			})
		}

		if len(normalizedHeaderRules) > 0 {
			headerRulesBytes, err := json.Marshal(normalizedHeaderRules)
			if err != nil {
				response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to process header rules: %v", err)))
				return
			}
			headerRulesJSON = headerRulesBytes
		}
	}
	if headerRulesJSON == nil {
		headerRulesJSON = datatypes.JSON("[]")
	}

	group := models.Group{
		Name:               name,
		DisplayName:        strings.TrimSpace(req.DisplayName),
		Description:        strings.TrimSpace(req.Description),
		CodeSnippet:        strings.TrimSpace(req.CodeSnippet),
		Upstreams:          cleanedUpstreams,
		ChannelType:        channelType,
		Sort:               req.Sort,
		TestModel:          testModel,
		ValidationEndpoint: validationEndpoint,
		ParamOverrides:     req.ParamOverrides,
		Config:             cleanedConfig,
		HeaderRules:        headerRulesJSON,
		ProxyKeys:          strings.TrimSpace(req.ProxyKeys),
		ForceHTTP11:        req.ForceHTTP11,
	}

	if err := s.DB.Create(&group).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	if err := s.GroupManager.Invalidate(); err != nil {
		logrus.WithContext(c.Request.Context()).WithError(err).Error("failed to invalidate group cache")
	}
	response.Success(c, s.newGroupResponse(&group))
}

// ListGroups handles listing all groups.
func (s *Server) ListGroups(c *gin.Context) {
	var groups []models.Group
	if err := s.DB.Order("sort asc, id desc").Find(&groups).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	var groupResponses []GroupResponse
	for i := range groups {
		groupResponses = append(groupResponses, *s.newGroupResponse(&groups[i]))
	}

	response.Success(c, groupResponses)
}

// GroupUpdateRequest defines the payload for updating a group.
// Using a dedicated struct avoids issues with zero values being ignored by GORM's Update.
type GroupUpdateRequest struct {
	Name               *string             `json:"name,omitempty"`
	DisplayName        *string             `json:"display_name,omitempty"`
	Description        *string             `json:"description,omitempty"`
	CodeSnippet        *string             `json:"code_snippet,omitempty"`
	Upstreams          json.RawMessage     `json:"upstreams"`
	ChannelType        *string             `json:"channel_type,omitempty"`
	Sort               *int                `json:"sort"`
	TestModel          string              `json:"test_model"`
	ValidationEndpoint *string             `json:"validation_endpoint,omitempty"`
	ParamOverrides     map[string]any      `json:"param_overrides"`
	Config             map[string]any      `json:"config"`
	HeaderRules        []models.HeaderRule `json:"header_rules"`
	ProxyKeys          *string             `json:"proxy_keys,omitempty"`
	ForceHTTP11        *bool               `json:"force_http11,omitempty"`
	CCRModels          []string            `json:"ccr_models,omitempty"`
}

// UpdateGroup handles updating an existing group.
func (s *Server) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	var group models.Group
	if err := s.DB.First(&group, id).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	var req GroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// Start a transaction
	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}
	defer tx.Rollback() // Rollback on panic

	// Apply updates from the request, with cleaning and validation
	if req.Name != nil {
		cleanedName := strings.TrimSpace(*req.Name)
		if !isValidGroupName(cleanedName) {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "无效的分组名称格式。只能包含小写字母、数字、中划线或下划线，长度3-30位"))
			return
		}
		group.Name = cleanedName
	}

	if req.DisplayName != nil {
		group.DisplayName = strings.TrimSpace(*req.DisplayName)
	}

	if req.Description != nil {
		group.Description = strings.TrimSpace(*req.Description)
	}

	if req.CodeSnippet != nil {
		group.CodeSnippet = strings.TrimSpace(*req.CodeSnippet)
	}

	// Handle CCR models update
	if req.CCRModels != nil {
		if err := s.handleCCRModelsUpdate(&group, req.CCRModels); err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
			return
		}
	}

	if req.Upstreams != nil {
		cleanedUpstreams, err := validateAndCleanUpstreams(req.Upstreams)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
			return
		}
		group.Upstreams = cleanedUpstreams
	}

	if req.ChannelType != nil {
		cleanedChannelType := strings.TrimSpace(*req.ChannelType)
		if !isValidChannelType(cleanedChannelType) {
			supported := strings.Join(channel.GetChannels(), ", ")
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Invalid channel type. Supported types are: %s", supported)))
			return
		}
		group.ChannelType = cleanedChannelType
	}
	if req.Sort != nil {
		group.Sort = *req.Sort
	}
	if req.TestModel != "" {
		cleanedTestModel := strings.TrimSpace(req.TestModel)
		if cleanedTestModel == "" {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Test model cannot be empty or just spaces."))
			return
		}
		group.TestModel = cleanedTestModel
	}
	if req.ParamOverrides != nil {
		group.ParamOverrides = req.ParamOverrides
	}
	if req.ValidationEndpoint != nil {
		validationEndpoint := strings.TrimSpace(*req.ValidationEndpoint)
		if !isValidValidationEndpoint(validationEndpoint) {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "无效的测试路径。如果提供，必须是以 / 开头的有效路径，且不能是完整的URL。"))
			return
		}
		group.ValidationEndpoint = validationEndpoint
	}

	if req.Config != nil {
		cleanedConfig, err := s.validateAndCleanConfig(req.Config)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Invalid config format: %v", err)))
			return
		}
		group.Config = cleanedConfig
	}

	if req.ProxyKeys != nil {
		group.ProxyKeys = strings.TrimSpace(*req.ProxyKeys)
	}

	if req.ForceHTTP11 != nil {
		group.ForceHTTP11 = req.ForceHTTP11
	}

	// Handle header rules update
	if req.HeaderRules != nil {
		var headerRulesJSON datatypes.JSON
		normalizedHeaderRules := make([]models.HeaderRule, 0)
		seenKeys := make(map[string]bool)

		for _, rule := range req.HeaderRules {
			key := strings.TrimSpace(rule.Key)
			if key == "" {
				continue
			}

			// Normalize to canonical form
			canonicalKey := http.CanonicalHeaderKey(key)

			// Check for duplicate keys
			if seenKeys[canonicalKey] {
				response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, fmt.Sprintf("Duplicate header key: %s", canonicalKey)))
				return
			}
			seenKeys[canonicalKey] = true

			normalizedHeaderRules = append(normalizedHeaderRules, models.HeaderRule{
				Key:    canonicalKey,
				Value:  rule.Value,
				Action: rule.Action,
			})
		}

		if len(normalizedHeaderRules) > 0 {
			headerRulesBytes, err := json.Marshal(normalizedHeaderRules)
			if err != nil {
				response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to process header rules: %v", err)))
				return
			}
			headerRulesJSON = headerRulesBytes
		} else {
			headerRulesJSON = datatypes.JSON("[]")
		}
		group.HeaderRules = headerRulesJSON
	}

	// Save the updated group object
	if err := tx.Save(&group).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	if err := tx.Commit().Error; err != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	if err := s.GroupManager.Invalidate(); err != nil {
		logrus.WithContext(c.Request.Context()).WithError(err).Error("failed to invalidate group cache")
	}
	response.Success(c, s.newGroupResponse(&group))
}

// UpdateGroupsOrderRequest defines the payload for updating groups order.
type UpdateGroupsOrderRequest struct {
	Groups []struct {
		ID         uint  `json:"id"`
		Sort       int   `json:"sort"`
		Archived   bool  `json:"archived"`
		CategoryID *uint `json:"category_id"`
	} `json:"groups"`
}

// UpdateGroupsOrder handles batch updating of group sort order, archived status, and category assignment.
func (s *Server) UpdateGroupsOrder(c *gin.Context) {
	var req UpdateGroupsOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}
	defer tx.Rollback()

	now := time.Now()
	for _, g := range req.Groups {
		var groupToUpdate models.Group
		if err := tx.First(&groupToUpdate, g.ID).Error; err != nil {
			response.Error(c, app_errors.ParseDBError(err))
			return
		}

		groupToUpdate.Sort = g.Sort
		groupToUpdate.Archived = g.Archived
		groupToUpdate.CategoryID = g.CategoryID

		if g.Archived {
			groupToUpdate.ArchivedAt = &now
		} else {
			groupToUpdate.ArchivedAt = nil
		}

		if err := tx.Save(&groupToUpdate).Error; err != nil {
			response.Error(c, app_errors.ParseDBError(err))
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	if err := s.GroupManager.Invalidate(); err != nil {
		logrus.WithContext(c.Request.Context()).WithError(err).Error("failed to invalidate group cache")
	}

	response.Success(c, gin.H{"message": "Groups order updated successfully"})
}


// GroupResponse defines the structure for a group response, excluding sensitive or large fields.
type GroupResponse struct {
	ID                 uint                `json:"id"`
	Name               string              `json:"name"`
	Endpoint           string              `json:"endpoint"`
	DisplayName        string              `json:"display_name"`
	Description        string              `json:"description"`
	CodeSnippet        string              `json:"code_snippet"`
	CCRModels          []string            `json:"ccr_models"`
	Upstreams          datatypes.JSON      `json:"upstreams"`
	ChannelType        string              `json:"channel_type"`
	Sort               int                 `json:"sort"`
	TestModel          string              `json:"test_model"`
	ValidationEndpoint string              `json:"validation_endpoint"`
	ParamOverrides     datatypes.JSONMap   `json:"param_overrides"`
	Config             datatypes.JSONMap   `json:"config"`
	HeaderRules        []models.HeaderRule `json:"header_rules"`
	ProxyKeys          string              `json:"proxy_keys"`
	ForceHTTP11        *bool               `json:"force_http11,omitempty"`
	LastValidatedAt    *time.Time          `json:"last_validated_at"`
	Archived           bool                `json:"archived"`
	ArchivedAt         *time.Time          `json:"archived_at"`
	CategoryID         *uint               `json:"category_id"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// newGroupResponse creates a new GroupResponse from a models.Group.
func (s *Server) newGroupResponse(group *models.Group) *GroupResponse {
	appURL := s.SettingsManager.GetAppUrl()
	endpoint := ""
	if appURL != "" {
		u, err := url.Parse(appURL)
		if err == nil {
			channelEndpoint := s.getChannelEndpoint(group.ChannelType)
			basePath := strings.TrimRight(u.Path, "/") + "/proxy/" + group.Name

			// 如果channelEndpoint包含查询参数，需要分开处理
			if strings.Contains(channelEndpoint, "?") {
				parts := strings.SplitN(channelEndpoint, "?", 2)
				u.Path = basePath + parts[0]
				if parts[1] != "" {
					u.RawQuery = parts[1]
				}
			} else {
				u.Path = basePath + channelEndpoint
			}
			endpoint = u.String()
		}
	}

	// Parse header rules from JSON
	var headerRules []models.HeaderRule
	if len(group.HeaderRules) > 0 {
		if err := json.Unmarshal(group.HeaderRules, &headerRules); err != nil {
			logrus.WithError(err).Error("Failed to unmarshal header rules")
			headerRules = make([]models.HeaderRule, 0)
		}
	}

	return &GroupResponse{
		ID:                 group.ID,
		Name:               group.Name,
		Endpoint:           endpoint,
		DisplayName:        group.DisplayName,
		Description:        group.Description,
		CodeSnippet:        group.CodeSnippet,
		CCRModels:          parseCCRModelsFromCodeSnippet(group.CodeSnippet),
		Upstreams:          group.Upstreams,
		ChannelType:        group.ChannelType,
		Sort:               group.Sort,
		TestModel:          group.TestModel,
		ValidationEndpoint: group.ValidationEndpoint,
		ParamOverrides:     group.ParamOverrides,
		Config:             group.Config,
		HeaderRules:        headerRules,
		ProxyKeys:          group.ProxyKeys,
		ForceHTTP11:        group.ForceHTTP11,
		LastValidatedAt:    group.LastValidatedAt,
		Archived:           group.Archived,
		ArchivedAt:         group.ArchivedAt,
		CategoryID:         group.CategoryID,
		CreatedAt:          group.CreatedAt,
		UpdatedAt:          group.UpdatedAt,
	}
}

// DeleteGroup handles deleting a group.
func (s *Server) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	// First, get all API keys for this group to clean up from memory store
	var apiKeys []models.APIKey
	if err := s.DB.Where("group_id = ?", id).Find(&apiKeys).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Extract key IDs for memory store cleanup
	var keyIDs []uint
	for _, key := range apiKeys {
		keyIDs = append(keyIDs, key.ID)
	}

	// Use a transaction to ensure atomicity
	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// First check if the group exists
	var group models.Group
	if err := tx.First(&group, id).Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Delete associated API keys first due to foreign key constraint
	if err := tx.Where("group_id = ?", id).Delete(&models.APIKey{}).Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	// Then delete the group
	if err := tx.Delete(&models.Group{}, id).Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Clean up memory store (Redis) within the transaction to ensure atomicity
	// If Redis cleanup fails, the entire transaction will be rolled back
	if len(keyIDs) > 0 {
		if err := s.KeyService.KeyProvider.RemoveKeysFromStore(uint(id), keyIDs); err != nil {
			tx.Rollback()
			logrus.WithFields(logrus.Fields{
				"groupID":  id,
				"keyCount": len(keyIDs),
				"error":    err,
			}).Error("Failed to remove keys from memory store, rolling back transaction")

			response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase,
				"Failed to delete group: unable to clean up cache"))
			return
		}
	}

	// Commit the transaction only if both DB and Redis operations succeed
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	if err := s.GroupManager.Invalidate(); err != nil {
		logrus.WithContext(c.Request.Context()).WithError(err).Error("failed to invalidate group cache")
	}
	response.Success(c, gin.H{"message": "Group and associated keys deleted successfully"})
}

// ConfigOption represents a single configurable option for a group.
type ConfigOption struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	DefaultValue any    `json:"default_value"`
}

// GetGroupConfigOptions returns a list of available configuration options for groups.
func (s *Server) GetGroupConfigOptions(c *gin.Context) {
	var options []ConfigOption

	// 1. Get all system setting definitions from the struct tags
	defaultSettings := utils.DefaultSystemSettings()
	settingDefinitions := utils.GenerateSettingsMetadata(&defaultSettings)
	defMap := make(map[string]models.SystemSettingInfo)
	for _, def := range settingDefinitions {
		defMap[def.Key] = def
	}

	// 2. Get current system setting values
	currentSettings := s.SettingsManager.GetSettings()
	currentSettingsValue := reflect.ValueOf(currentSettings)
	currentSettingsType := currentSettingsValue.Type()
	jsonToFieldMap := make(map[string]string)
	for i := 0; i < currentSettingsType.NumField(); i++ {
		field := currentSettingsType.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonTag != "" {
			jsonToFieldMap[jsonTag] = field.Name
		}
	}

	// 3. Iterate over GroupConfig fields to maintain order and build the response
	groupConfigType := reflect.TypeOf(models.GroupConfig{})

	for i := 0; i < groupConfigType.NumField(); i++ {
		field := groupConfigType.Field(i)
		jsonTag := field.Tag.Get("json")
		key := strings.Split(jsonTag, ",")[0]

		if key == "" || key == "-" {
			continue
		}

		if definition, ok := defMap[key]; ok {
			var defaultValue any
			if fieldName, ok := jsonToFieldMap[key]; ok {
				defaultValue = currentSettingsValue.FieldByName(fieldName).Interface()
			}

			option := ConfigOption{
				Key:          key,
				Name:         definition.Name,
				Description:  definition.Description,
				DefaultValue: defaultValue,
			}
			options = append(options, option)
		}
	}

	response.Success(c, options)
}

// KeyStats defines the statistics for API keys in a group.
type KeyStats struct {
	TotalKeys   int64 `json:"total_keys"`
	ActiveKeys  int64 `json:"active_keys"`
	InvalidKeys int64 `json:"invalid_keys"`
}

// RequestStats defines the statistics for requests over a period.
type RequestStats struct {
	TotalRequests  int64   `json:"total_requests"`
	FailedRequests int64   `json:"failed_requests"`
	FailureRate    float64 `json:"failure_rate"`
}

// GroupStatsResponse defines the complete statistics for a group.
type GroupStatsResponse struct {
	KeyStats    KeyStats     `json:"key_stats"`
	HourlyStats RequestStats `json:"hourly_stats"` // 1 hour
	DailyStats  RequestStats `json:"daily_stats"`  // 24 hours
	WeeklyStats RequestStats `json:"weekly_stats"` // 7 days
}

// calculateRequestStats is a helper to compute request statistics.
func calculateRequestStats(total, failed int64) RequestStats {
	stats := RequestStats{
		TotalRequests:  total,
		FailedRequests: failed,
	}
	if total > 0 {
		stats.FailureRate, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", float64(failed)/float64(total)), 64)
	}
	return stats
}

// GetGroupStats handles retrieving detailed statistics for a specific group.
func (s *Server) GetGroupStats(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}
	groupID := uint(id)

	// 1. 验证分组是否存在
	var group models.Group
	if err := s.DB.First(&group, groupID).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	var resp GroupStatsResponse
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	// 并发执行所有统计查询

	// 2. Key 统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		var totalKeys, activeKeys int64

		if err := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Count(&totalKeys).Error; err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("failed to get total keys: %w", err))
			mu.Unlock()
			return
		}
		if err := s.DB.Model(&models.APIKey{}).Where("group_id = ? AND status = ?", groupID, models.KeyStatusActive).Count(&activeKeys).Error; err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("failed to get active keys: %w", err))
			mu.Unlock()
			return
		}

		mu.Lock()
		resp.KeyStats = KeyStats{
			TotalKeys:   totalKeys,
			ActiveKeys:  activeKeys,
			InvalidKeys: totalKeys - activeKeys,
		}
		mu.Unlock()
	}()

	// 3. 1小时请求统计 (查询 request_logs 表)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var total, failed int64
		now := time.Now()
		oneHourAgo := now.Add(-1 * time.Hour)

		if err := s.DB.Model(&models.RequestLog{}).Where("group_id = ? AND timestamp BETWEEN ? AND ? AND request_type = ?", groupID, oneHourAgo, now, models.RequestTypeFinal).Count(&total).Error; err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("failed to get hourly total requests: %w", err))
			mu.Unlock()
			return
		}
		if err := s.DB.Model(&models.RequestLog{}).Where("group_id = ? AND timestamp BETWEEN ? AND ? AND is_success = ? AND request_type = ?", groupID, oneHourAgo, now, false, models.RequestTypeFinal).Count(&failed).Error; err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("failed to get hourly failed requests: %w", err))
			mu.Unlock()
			return
		}

		mu.Lock()
		resp.HourlyStats = calculateRequestStats(total, failed)
		mu.Unlock()
	}()

	// 4. 24小时和7天统计 (查询 group_hourly_stats 表)
	// 辅助函数，用于从 group_hourly_stats 查询
	queryHourlyStats := func(duration time.Duration) (RequestStats, error) {
		var result struct {
			SuccessCount int64
			FailureCount int64
		}
		now := time.Now()
		// 结束时间为当前小时的整点，查询时不包含该小时
		// 开始时间为结束时间减去统计周期
		endTime := now.Truncate(time.Hour)
		startTime := endTime.Add(-duration)

		err := s.DB.Model(&models.GroupHourlyStat{}).
			Select("SUM(success_count) as success_count, SUM(failure_count) as failure_count").
			Where("group_id = ? AND time >= ? AND time < ?", groupID, startTime, endTime).
			Scan(&result).Error
		if err != nil {
			return RequestStats{}, err
		}
		return calculateRequestStats(result.SuccessCount+result.FailureCount, result.FailureCount), nil
	}

	// 24小时统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats, err := queryHourlyStats(24 * time.Hour)
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("failed to get daily stats: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		resp.DailyStats = stats
		mu.Unlock()
	}()

	// 7天统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats, err := queryHourlyStats(7 * 24 * time.Hour)
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("failed to get weekly stats: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		resp.WeeklyStats = stats
		mu.Unlock()
	}()

	wg.Wait()

	if len(errors) > 0 {
		// 只记录第一个错误，但表明可能存在多个错误
		logrus.WithContext(c.Request.Context()).WithError(errors[0]).Error("Errors occurred while fetching group stats")
		response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase, "Failed to retrieve some statistics"))
		return
	}

	response.Success(c, resp)
}

// GroupCopyRequest defines the payload for copying a group.
type GroupCopyRequest struct {
	NewGroupName    string `json:"new_group_name"`    // 新分组名称
	CopyKeys        string `json:"copy_keys"`         // "none"|"valid_only"|"all"
	CopyDescription bool   `json:"copy_description"`  // 是否复制描述
	CopyCodeSnippet bool   `json:"copy_code_snippet"` // 是否复制代码片段
}

// GroupCopyResponse defines the response for group copy operation.
type GroupCopyResponse struct {
	Group *GroupResponse `json:"group"`
}

// generateUniqueGroupName generates a unique group name by appending _copy and numbers if needed.
func (s *Server) generateUniqueGroupName(baseName string) string {
	var groups []models.Group
	if err := s.DB.Select("name").Find(&groups).Error; err != nil {
		return baseName + "_copy"
	}

	// Create a map of existing names for quick lookup
	existingNames := make(map[string]bool)
	for _, group := range groups {
		existingNames[group.Name] = true
	}

	// Try base name with _copy suffix first
	copyName := baseName + "_copy"
	if !existingNames[copyName] {
		return copyName
	}

	// Try appending numbers to _copy suffix
	for i := 2; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s_copy_%d", baseName, i)
		if !existingNames[candidate] {
			return candidate
		}
	}

	return copyName
}

// CopyGroup handles copying a group with optional content.
func (s *Server) CopyGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}
	sourceGroupID := uint(id)

	var req GroupCopyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// Validate copy keys option
	if req.CopyKeys != "" && req.CopyKeys != "none" && req.CopyKeys != "valid_only" && req.CopyKeys != "all" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid copy_keys value. Must be 'none', 'valid_only', or 'all'"))
		return
	}
	if req.CopyKeys == "" {
		req.CopyKeys = "all"
	}

	// Validate and process new group name
	var newGroupName string
	if req.NewGroupName != "" {
		newGroupName = strings.TrimSpace(req.NewGroupName)
		if !isValidGroupName(newGroupName) {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "无效的分组名称。只能包含小写字母、数字、中划线或下划线，长度3-30位"))
			return
		}
	}

	// Check if source group exists
	var sourceGroup models.Group
	if err := s.DB.First(&sourceGroup, sourceGroupID).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Start transaction
	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}
	defer tx.Rollback()

	// Create new group by copying source group and overriding specific fields
	newGroup := sourceGroup
	newGroup.ID = 0

	// Use provided name or generate unique name
	if newGroupName != "" {
		newGroup.Name = newGroupName
	} else {
		newGroup.Name = s.generateUniqueGroupName(sourceGroup.Name)
	}

	// Handle display name
	if sourceGroup.DisplayName != "" {
		newGroup.DisplayName = sourceGroup.DisplayName + " Copy"
	}

	// Handle description based on copy settings
	if !req.CopyDescription {
		newGroup.Description = ""
	}

	// Handle code snippet based on copy settings
	if !req.CopyCodeSnippet {
		newGroup.CodeSnippet = ""
	}

	newGroup.CreatedAt = time.Time{}
	newGroup.UpdatedAt = time.Time{}
	newGroup.LastValidatedAt = nil

	// Create the new group
	if err := tx.Create(&newGroup).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Prepare key data for async import task
	var sourceKeyValues []string

	if req.CopyKeys != "none" {
		var sourceKeys []models.APIKey
		query := tx.Where("group_id = ?", sourceGroupID)

		// Filter by status if only copying valid keys
		if req.CopyKeys == "valid_only" {
			query = query.Where("status = ?", models.KeyStatusActive)
		}

		if err := query.Find(&sourceKeys).Error; err != nil {
			response.Error(c, app_errors.ParseDBError(err))
			return
		}

		// Extract key values for async import task
		for _, sourceKey := range sourceKeys {
			sourceKeyValues = append(sourceKeyValues, sourceKey.KeyValue)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	// Update caches after successful transaction
	if err := s.GroupManager.Invalidate(); err != nil {
		logrus.WithContext(c.Request.Context()).WithError(err).Error("failed to invalidate group cache")
	}

	// Start async key import task if there are keys to copy (reuse existing logic)
	if len(sourceKeyValues) > 0 {
		// Convert key values array to text format expected by KeyImportService
		keysText := strings.Join(sourceKeyValues, "\n")

		// Directly reuse the AddMultipleKeysAsync logic from key_handler.go
		if _, err := s.KeyImportService.StartImportTask(&newGroup, keysText); err != nil {
			logrus.WithFields(logrus.Fields{
				"groupId":  newGroup.ID,
				"keyCount": len(sourceKeyValues),
				"error":    err,
			}).Error("Failed to start async key import task for group copy")
		} else {
			logrus.WithFields(logrus.Fields{
				"groupId":  newGroup.ID,
				"keyCount": len(sourceKeyValues),
			}).Info("Started async key import task for group copy")
		}
	}

	// Prepare response
	groupResponse := s.newGroupResponse(&newGroup)
	copyResponse := &GroupCopyResponse{
		Group: groupResponse,
	}

	response.Success(c, copyResponse)
}

// List godoc
func (s *Server) List(c *gin.Context) {
	var groups []models.Group
	if err := s.DB.Select("id, name,display_name").Find(&groups).Error; err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase, "无法获取分组列表"))
		return
	}
	response.Success(c, groups)
}

// ArchiveGroup handles archiving a group.
func (s *Server) ArchiveGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	var group models.Group
	if err := s.DB.First(&group, id).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Update group archived status
	now := time.Now()
	group.Archived = true
	group.ArchivedAt = &now

	if err := s.DB.Save(&group).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	if err := s.GroupManager.Invalidate(); err != nil {
		logrus.WithContext(c.Request.Context()).WithError(err).Error("failed to invalidate group cache")
	}

	response.Success(c, s.newGroupResponse(&group))
}

// UnarchiveGroup handles unarchiving a group.
func (s *Server) UnarchiveGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	var group models.Group
	if err := s.DB.First(&group, id).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// Update group archived status
	group.Archived = false
	group.ArchivedAt = nil

	if err := s.DB.Save(&group).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	if err := s.GroupManager.Invalidate(); err != nil {
		logrus.WithContext(c.Request.Context()).WithError(err).Error("failed to invalidate group cache")
	}

	response.Success(c, s.newGroupResponse(&group))
}

// CodeSnippetStructure defines the structure for the code snippet JSON
type CodeSnippetStructure struct {
	Name       string                 `json:"name"`
	APIBaseURL string                 `json:"api_base_url"`
	APIKey     string                 `json:"api_key"`
	Models     []string               `json:"models"`
	Transformer map[string]interface{} `json:"transformer,omitempty"`
}

// handleCCRModelsUpdate handles the update of CCR models by updating the code_snippet field
func (s *Server) handleCCRModelsUpdate(group *models.Group, ccrModels []string) error {
	// If no new CCR models provided, nothing to do
	if len(ccrModels) == 0 {
		return nil
	}

	var snippetObj CodeSnippetStructure

	// If code_snippet is not empty, try to parse it
	if group.CodeSnippet != "" {
		if err := json.Unmarshal([]byte(group.CodeSnippet), &snippetObj); err != nil {
			// If parsing fails and we have new tags to add, return error
			return fmt.Errorf("代码片段不是 json 规范, 无法添加 tag")
		}
	} else {
		// Create default structure based on channel type
		snippetObj = s.createDefaultCodeSnippet(group, ccrModels)
	}

	// Ensure models array exists and merge with new CCR models
	if snippetObj.Models == nil {
		snippetObj.Models = []string{}
	}

	// Create a map for deduplication
	modelMap := make(map[string]bool)
	for _, model := range snippetObj.Models {
		modelMap[model] = true
	}

	// Add new models (deduplicated)
	for _, model := range ccrModels {
		if model != "" && !modelMap[model] {
			snippetObj.Models = append(snippetObj.Models, model)
			modelMap[model] = true
		}
	}

	// Convert back to JSON string with formatting
	updatedJSON, err := json.MarshalIndent(snippetObj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated code snippet: %v", err)
	}

	// Update the group's code snippet
	group.CodeSnippet = string(updatedJSON)

	return nil
}

// createDefaultCodeSnippet creates a default code snippet structure based on channel type
func (s *Server) createDefaultCodeSnippet(group *models.Group, ccrModels []string) CodeSnippetStructure {
	snippet := CodeSnippetStructure{
		Models:      ccrModels,
	}

	// 获取代理密钥（优先使用分组代理密钥，否则使用全局代理密钥）
	apiKey := s.getProxyKey(group)

	switch group.ChannelType {
	case "anthropic":
		snippet.Name = group.Name
		snippet.APIBaseURL = fmt.Sprintf("http://localhost:3001/proxy/%s/v1/messages?beta=true", group.Name)
		snippet.APIKey = apiKey
		snippet.Transformer = map[string]interface{}{
			"use": []string{"Anthropic"},
		}
	case "openai":
		snippet.Name = group.Name
		snippet.APIBaseURL = fmt.Sprintf("http://localhost:3001/proxy/%s/v1/chat/completions", group.Name)
		snippet.APIKey = apiKey
		snippet.Transformer = s.createOpenAITransformer(ccrModels)
	case "gemini":
		snippet.Name = group.Name
		snippet.APIBaseURL = fmt.Sprintf("http://localhost:3001/proxy/%s/v1beta/models/", group.Name)
		snippet.APIKey = apiKey
		snippet.Transformer = map[string]interface{}{
			"use": []string{"gemini"},
		}
	default:
		snippet.Name = group.Name
		snippet.APIBaseURL = fmt.Sprintf("http://localhost:3001/proxy/%s/v1/chat/completions", group.Name)
		snippet.APIKey = apiKey
	}

	return snippet
}

// getProxyKey gets the proxy key for the group (group proxy key first, then global proxy key)
func (s *Server) getProxyKey(group *models.Group) string {
	// 首先尝试获取分组的代理密钥
	if group.ProxyKeys != "" {
		keys := utils.SplitAndTrim(group.ProxyKeys, ",")
		if len(keys) > 0 {
			return keys[0]
		}
	}

	// 如果分组没有代理密钥，则直接从 SettingsManager 获取全局设置
	globalSettings := s.SettingsManager.GetSettings()
	if globalSettings.ProxyKeys != "" {
		keys := utils.SplitAndTrim(globalSettings.ProxyKeys, ",")
		if len(keys) > 0 {
			return keys[0]
		}
	}


	// 如果都没有，返回默认值
	return "your-api-key-here"
}

// createOpenAITransformer creates the transformer configuration for OpenAI channel type
func (s *Server) createOpenAITransformer(ccrModels []string) map[string]interface{} {
	transformer := map[string]interface{}{
		"use": []interface{}{
			[]interface{}{
				"maxtoken",
				map[string]interface{}{
					"max_tokens": 65535,
				},
			},
		},
	}

	// 为每个模型添加 reasoning 配置
	for _, model := range ccrModels {
		if model != "" {
			transformer[model] = map[string]interface{}{
				"use": []string{"reasoning"},
			}
		}
	}

	return transformer
}

// parseCCRModelsFromCodeSnippet extracts CCR models from code_snippet field
func parseCCRModelsFromCodeSnippet(codeSnippet string) []string {
	if codeSnippet == "" {
		return []string{}
	}

	var snippetObj CodeSnippetStructure
	if err := json.Unmarshal([]byte(codeSnippet), &snippetObj); err != nil {
		// If parsing fails, return empty array
		return []string{}
	}

	if snippetObj.Models == nil {
		return []string{}
	}

	return snippetObj.Models
}

// UpdateGroupCCRModelsRequest defines the request body for updating CCR models.
type UpdateGroupCCRModelsRequest struct {
	Models string `json:"models"`
}

// UpdateGroupCCRModels handles the dedicated CCR model update request.
func (s *Server) UpdateGroupCCRModels(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid group ID format"))
		return
	}

	var req UpdateGroupCCRModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	var group models.Group
	if err := s.DB.First(&group, groupID).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	newModels := utils.SplitAndTrim(req.Models, ",")

	if err := s.replaceCCRModelsInCodeSnippet(&group, newModels); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error()))
		return
	}

	if err := s.DB.Save(&group).Error; err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error()))
		return
	}

	response.Success(c, nil)
}

// replaceCCRModelsInCodeSnippet handles the logic of replacing models in code_snippet.
func (s *Server) replaceCCRModelsInCodeSnippet(group *models.Group, newModels []string) error {
	var snippetObj CodeSnippetStructure // Reuse the existing struct

	if group.CodeSnippet != "" {
		if err := json.Unmarshal([]byte(group.CodeSnippet), &snippetObj); err != nil {
			return fmt.Errorf("代码片段不是有效的 JSON, 无法更新模型")
		}
	} else {
		// If code snippet is empty, create a default structure
		snippetObj = s.createDefaultCodeSnippet(group, []string{}) // Initial models are empty
	}

	// **Core Difference: Direct replacement, not merging**
	snippetObj.Models = newModels

	updatedJSON, err := json.MarshalIndent(snippetObj, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化代码片段失败: %v", err)
	}

	group.CodeSnippet = string(updatedJSON)
	return nil
}
