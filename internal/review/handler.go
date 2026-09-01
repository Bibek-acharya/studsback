package review

import (
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func getUserID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	if userID == nil {
		return 0
	}
	return userID.(uint)
}

func (h *Handler) SubmitReview(c *gin.Context) {
	userID := getUserID(c)

	var req CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	review, err := h.service.SubmitReview(userID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Review submitted successfully", gin.H{"review": review})
}

func (h *Handler) GetUserReviews(c *gin.Context) {
	userID := getUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := h.service.GetUserReviews(userID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Reviews fetched successfully", result)
}

func (h *Handler) GetCollegeReviews(c *gin.Context) {
	collegeID, err := strconv.ParseUint(c.Param("collegeId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid college ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	instID, _ := strconv.ParseUint(c.Query("inst_id"), 10, 64)

	result, err := h.service.GetCollegeReviews(uint(collegeID), uint(instID), getUserID(c), page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Reviews fetched successfully", result)
}

func (h *Handler) GetInstitutionReviews(c *gin.Context) {
	instID := getUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := h.service.GetInstitutionReviews(instID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Reviews fetched successfully", result)
}

func (h *Handler) UpdateReview(c *gin.Context) {
	userID := getUserID(c)

	reviewID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid review ID")
		return
	}

	var req UpdateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	review, err := h.service.UpdateReview(uint(reviewID), userID, req)
	if err != nil {
		if err.Error() == "review not found" {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Review updated successfully", gin.H{"review": review})
}

func (h *Handler) DeleteReview(c *gin.Context) {
	userID := getUserID(c)

	reviewID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid review ID")
		return
	}

	if err := h.service.DeleteReview(uint(reviewID), userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "Review not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Review deleted successfully", nil)
}

func (h *Handler) MarkHelpful(c *gin.Context) {
	userID := getUserID(c)

	reviewID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid review ID")
		return
	}

	var req VoteReviewRequest
	_ = c.ShouldBindJSON(&req)
	vote := req.Vote
	if vote != "down" {
		vote = "up"
	}

	up, down, myVote, err := h.service.VoteReview(uint(reviewID), userID, vote)
	if err != nil {
		if err.Error() == "review not found" {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Vote recorded", gin.H{
		"helpful_upvotes":   up,
		"helpful_downvotes": down,
		"my_vote":           myVote,
	})
}

func (h *Handler) ReportReview(c *gin.Context) {
	userID := getUserID(c)

	reviewID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid review ID")
		return
	}

	var req ReportReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ReportReview(uint(reviewID), userID, req.Reason); err != nil {
		if err.Error() == "review not found" {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Review reported successfully", nil)
}

func (h *Handler) SubmitUniversityReview(c *gin.Context) {
	userID := getUserID(c)

	var req CreateUniversityReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, humanizeValidationError(err))
		return
	}

	review, err := h.service.SubmitUniversityReview(userID, req)
	if err != nil {
		if err.Error() == "you have already reviewed this university" {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Review submitted successfully", gin.H{"review": review})
}

func (h *Handler) GetUniversityReviews(c *gin.Context) {
	universityID, err := strconv.ParseUint(c.Param("universityId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid university ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := h.service.GetUniversityReviews(uint(universityID), page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Reviews fetched successfully", result)
}

func (h *Handler) GetMyUniversityReview(c *gin.Context) {
	userID := getUserID(c)

	universityID, err := strconv.ParseUint(c.Param("universityId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid university ID")
		return
	}

	review, err := h.service.GetUserUniversityReview(userID, uint(universityID))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Review not found")
		return
	}

	response.Success(c, http.StatusOK, "Review fetched successfully", gin.H{"review": review})
}

func (h *Handler) UpdateUniversityReview(c *gin.Context) {
	userID := getUserID(c)

	universityID, err := strconv.ParseUint(c.Param("universityId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid university ID")
		return
	}

	var req UpdateUniversityReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if errors.Is(err, io.EOF) {
			response.Error(c, http.StatusBadRequest, "At least one review field is required")
			return
		}
		response.Error(c, http.StatusBadRequest, humanizeValidationError(err))
		return
	}

	review, err := h.service.UpdateUniversityReview(userID, uint(universityID), req)
	if err != nil {
		if err.Error() == "review not found" {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		if err.Error() == "At least one review field is required" {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Review updated successfully", gin.H{"review": review})
}

// Admin handlers for managing reviews
func (h *Handler) AdminGetUniversityReviews(c *gin.Context) {
	universityID, err := strconv.ParseUint(c.Param("universityId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid university ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := h.service.AdminGetUniversityReviews(uint(universityID), page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Reviews fetched successfully", result)
}

func (h *Handler) AdminDeleteReview(c *gin.Context) {
	reviewID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid review ID")
		return
	}

	if err := h.service.AdminDeleteReview(uint(reviewID)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "Review not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Review deleted successfully", nil)
}

// Date report handlers
func (h *Handler) CreateDateReport(c *gin.Context) {
	var req CreateDateReportRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Handle file upload
	var fileURL string
	file, header, err := c.Request.FormFile("file")
	if err == nil {
		defer file.Close()
		// Save file using utility function
		savedPath, err := utils.SaveUploadedDocument(header, "date-reports")
		if err != nil {
			response.Error(c, http.StatusBadRequest, "Failed to upload file: "+err.Error())
			return
		}
		fileURL = savedPath
	}

	report, err := h.service.CreateDateReport(req, fileURL)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Date report submitted successfully", gin.H{"report": report})
}

func (h *Handler) GetAllDateReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	reports, total, err := h.service.GetAllDateReports(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	response.Success(c, http.StatusOK, "Date reports fetched successfully", gin.H{
		"reports": reports,
		"meta": Meta{
			Total:      int(total),
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	})
}

func (h *Handler) UpdateDateReportStatus(c *gin.Context) {
	reportID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid report ID")
		return
	}

	var req UpdateDateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateDateReportStatus(uint(reportID), req.Status); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "Report not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Report status updated successfully", nil)
}

func (h *Handler) DeleteDateReport(c *gin.Context) {
	reportID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid report ID")
		return
	}

	if err := h.service.DeleteDateReport(uint(reportID)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "Report not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Report deleted successfully", nil)
}
