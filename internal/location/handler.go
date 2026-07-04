package location

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type GeocodeResponse struct {
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	BoundingBox []float64 `json:"boundingBox,omitempty"`
}

func (h *Handler) Geocode(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "query parameter 'q' is required",
		})
		return
	}

	if len(query) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "query must be at least 2 characters",
		})
		return
	}

	results, err := h.service.Geocode(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "geocoding failed",
		})
		return
	}

	if len(results) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    []interface{}{},
		})
		return
	}

	locations := make([]GeocodeResponse, 0, len(results))
	for _, r := range results {
		var lat, lon float64
		fmt.Sscanf(r.Lat, "%f", &lat)
		fmt.Sscanf(r.Lon, "%f", &lon)

		resp := GeocodeResponse{
			Name:        r.DisplayName,
			DisplayName: r.DisplayName,
			Latitude:    lat,
			Longitude:   lon,
		}

		if len(r.BoundingBox) == 4 {
			bbox := make([]float64, 4)
			for i, v := range r.BoundingBox {
				fmt.Sscanf(v, "%f", &bbox[i])
			}
			resp.BoundingBox = bbox
		}

		locations = append(locations, resp)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    locations,
	})
}
