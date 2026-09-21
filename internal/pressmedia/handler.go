package pressmedia

import (
	"net/http"
	"strconv"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListItems handles GET /api/v1/media-press (published items only).
func (h *Handler) ListItems(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := ItemFilters{
		Category:      c.Query("category"),
		PublishedOnly: true,
	}

	items, total, err := h.service.GetItems(filters, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch media & press items")
		return
	}

	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Media & press items fetched successfully", gin.H{
		"page":  page,
		"limit": limit,
		"total": total,
		"items": items,
	})
}

// AdminListItems handles GET /api/v1/superadmin/media-press (all items).
func (h *Handler) AdminListItems(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := ItemFilters{
		Category: c.Query("category"),
	}

	items, total, err := h.service.GetItems(filters, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch media & press items")
		return
	}

	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Media & press items fetched successfully", gin.H{
		"page":  page,
		"limit": limit,
		"total": total,
		"items": items,
	})
}

func (h *Handler) GetItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}
	item, err := h.service.GetItem(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Item not found")
		return
	}
	response.Success(c, http.StatusOK, "Item fetched successfully", item)
}

func (h *Handler) CreateItem(c *gin.Context) {
	var req CreatePressMediaItemInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.service.CreateItem(req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, http.StatusCreated, "Media & press item created", item)
}

func (h *Handler) UpdateItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}
	var req UpdatePressMediaItemInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.service.UpdateItem(uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Media & press item updated", item)
}

func (h *Handler) DeleteItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}
	if err := h.service.DeleteItem(uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete media & press item")
		return
	}
	response.Success(c, http.StatusOK, "Media & press item deleted", nil)
}

// UploadImage handles POST /api/v1/superadmin/media-press/:id/image.
// It stores the uploaded image via SaveUploadedImage and sets ImageURL.
func (h *Handler) UploadImage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}

	item, err := h.service.GetItem(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Item not found")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	imageURL, err := utils.SaveUploadedImage(fileHeader, "media-press")
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	item.ImageURL = imageURL
	if err := h.service.UpdateItemModel(item); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update item image")
		return
	}

	response.Success(c, http.StatusOK, "Item image uploaded", item)
}
