// Package handler provides HTTP handlers for the application
package handler

import (
	"strconv"
	"strings"

	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// CategoryCreateRequest 定义创建分类的请求体
type CategoryCreateRequest struct {
	Name string `json:"name"`
}

// CategoryUpdateRequest 定义更新分类的请求体
type CategoryUpdateRequest struct {
	Name string `json:"name"`
}

// CategoryOrderRequest 定义批量更新分类排序的请求体
type CategoryOrderRequest struct {
	Categories []struct {
		ID   uint `json:"id"`
		Sort int  `json:"sort"`
	} `json:"categories"`
}

// CreateCategory 创建新分类
func (s *Server) CreateCategory(c *gin.Context) {
	var req CategoryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// 验证分类名称
	name := strings.TrimSpace(req.Name)
	if name == "" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "分类名称不能为空"))
		return
	}

	if len(name) > 50 {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "分类名称长度不能超过50个字符"))
		return
	}

	// 检查名称是否已存在
	var existingCategory models.Category
	if err := s.DB.Where("name = ?", name).First(&existingCategory).Error; err == nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "分类名称已存在"))
		return
	}

	// 获取当前最大排序值
	var maxSort int
	s.DB.Model(&models.Category{}).Select("COALESCE(MAX(sort), -1) + 1").Scan(&maxSort)

	category := models.Category{
		Name: name,
		Sort: maxSort,
	}

	if err := s.DB.Create(&category).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, category)
}

// ListCategories 获取所有分类列表
func (s *Server) ListCategories(c *gin.Context) {
	var categories []models.Category
	if err := s.DB.Order("sort ASC").Find(&categories).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, categories)
}

// UpdateCategory 更新分类名称
func (s *Server) UpdateCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid category ID format"))
		return
	}

	var req CategoryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// 验证分类名称
	name := strings.TrimSpace(req.Name)
	if name == "" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "分类名称不能为空"))
		return
	}

	if len(name) > 50 {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "分类名称长度不能超过50个字符"))
		return
	}

	// 获取要更新的分类
	var category models.Category
	if err := s.DB.First(&category, id).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// 检查名称是否已被其他分类使用
	var existingCategory models.Category
	if err := s.DB.Where("name = ? AND id != ?", name, id).First(&existingCategory).Error; err == nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "分类名称已存在"))
		return
	}

	// 更新分类名称
	category.Name = name
	if err := s.DB.Save(&category).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, category)
}

// DeleteCategory 删除分类（将分类下所有group移到归档）
func (s *Server) DeleteCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Invalid category ID format"))
		return
	}

	categoryID := uint(id)

	// 开始事务
	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}
	defer tx.Rollback()

	// 检查分类是否存在
	var category models.Category
	if err := tx.First(&category, categoryID).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// 将该分类下的所有group移到归档
	if err := tx.Model(&models.Group{}).
		Where("category_id = ?", categoryID).
		Updates(map[string]interface{}{
			"category_id": nil,
			"archived":    true,
		}).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// 删除分类
	if err := tx.Delete(&category).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	// 清除缓存
	if err := s.GroupManager.Invalidate(); err != nil {
		logrus.WithContext(c.Request.Context()).WithError(err).Error("failed to invalidate group cache")
	}

	response.Success(c, gin.H{"message": "分类删除成功，关联的分组已移至归档"})
}

// UpdateCategoriesOrder 批量更新分类排序
func (s *Server) UpdateCategoriesOrder(c *gin.Context) {
	var req CategoryOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// 开始事务
	tx := s.DB.Begin()
	if tx.Error != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}
	defer tx.Rollback()

	// 批量更新排序
	for _, cat := range req.Categories {
		if err := tx.Model(&models.Category{}).
			Where("id = ?", cat.ID).
			Update("sort", cat.Sort).Error; err != nil {
			response.Error(c, app_errors.ParseDBError(err))
			return
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		response.Error(c, app_errors.ErrDatabase)
		return
	}

	response.Success(c, gin.H{"message": "分类排序更新成功"})
}