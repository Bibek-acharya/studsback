package faq

import (
	"net/http"
	"strconv"

	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.service.GetCategories()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch FAQs")
		return
	}
	if categories == nil {
		categories = []FAQCategory{}
	}
	response.Success(c, http.StatusOK, "FAQs fetched successfully", categories)
}

func (h *Handler) GetCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid category ID")
		return
	}
	cat, err := h.service.GetCategory(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Category not found")
		return
	}
	response.Success(c, http.StatusOK, "Category fetched successfully", cat)
}

func (h *Handler) CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	cat, err := h.service.CreateCategory(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create category")
		return
	}
	response.Success(c, http.StatusCreated, "Category created", cat)
}

func (h *Handler) UpdateCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid category ID")
		return
	}
	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	cat, err := h.service.UpdateCategory(uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Category updated", cat)
}

func (h *Handler) DeleteCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid category ID")
		return
	}
	if err := h.service.DeleteCategory(uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete category")
		return
	}
	response.Success(c, http.StatusOK, "Category deleted", nil)
}

func (h *Handler) CreateItem(c *gin.Context) {
	var req CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.service.CreateItem(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create FAQ item")
		return
	}
	response.Success(c, http.StatusCreated, "FAQ item created", item)
}

func (h *Handler) UpdateItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}
	var req UpdateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.service.UpdateItem(uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "FAQ item updated", item)
}

func (h *Handler) DeleteItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}
	if err := h.service.DeleteItem(uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete FAQ item")
		return
	}
	response.Success(c, http.StatusOK, "FAQ item deleted", nil)
}
