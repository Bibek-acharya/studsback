package scholarship

import (
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type ProfileContextResponse struct {
	EducationEntries []EducationEntryResponse `json:"educationEntries"`
	Preferences      *PreferencesData         `json:"preferences,omitempty"`
	BookmarkedFields []string                 `json:"bookmarkedFields"`
}

type EducationEntryResponse struct {
	Level           string `json:"level"`
	Stream          string `json:"stream"`
	Grade           string `json:"grade"`
	GradingSystem   string `json:"gradingSystem"`
	InstitutionName string `json:"institutionName"`
}

func (h *Handler) GetRecommendationContext(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	profile, err := h.service.repo.GetUserProfileForRecommendation(uid)
	if err != nil {
		response.Error(c, 500, "Failed to fetch profile")
		return
	}

	entries := make([]EducationEntryResponse, 0, len(profile.EducationEntries))
	for _, e := range profile.EducationEntries {
		entries = append(entries, EducationEntryResponse{
			Level:           e.Level,
			Stream:          e.Stream,
			Grade:           e.Grade,
			GradingSystem:   e.GradingSystem,
			InstitutionName: e.InstitutionName,
		})
	}

	response.Success(c, 200, "Recommendation context retrieved", ProfileContextResponse{
		EducationEntries: entries,
		Preferences:      profile.Preferences,
		BookmarkedFields: profile.BookmarkedFields,
	})
}
