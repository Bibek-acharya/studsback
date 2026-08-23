package college

import (
	"encoding/json"
	"math"
	"time"
)

type comparisonInstitution struct {
	ID          uint
	LogoURL     string
	ProfileData []byte
}

type comparisonScholarship struct {
	ID       uint      `json:"id"`
	Title    string    `json:"title"`
	Value    string    `json:"value,omitempty"`
	Deadline time.Time `json:"deadline,omitempty"`
}

type comparisonReviewRow struct {
	ID        uint
	Ratings   []byte
	Pros      string
	Cons      string
	Course    string
	CreatedAt time.Time
}

func (r *Repository) BuildComparisonCollege(college College) (ComparisonCollege, error) {
	result := ComparisonCollege{
		College:      buildCollegeResponse(college),
		Facilities:   []string{},
		Scholarships: []interface{}{},
		Gallery:      []interface{}{},
		Reviews:      []ComparisonReview{},
	}

	var institution comparisonInstitution
	instErr := r.db.Table("institution_users").
		Select("id, logo_url, profile_data").
		Where("college_id = ? AND deleted_at IS NULL", college.ID).
		Order("verified DESC, claimed DESC, id ASC").
		First(&institution).Error
	if instErr == nil {
		result.InstitutionID = institution.ID
		result.LogoURL = institution.LogoURL
		applyInstitutionProfile(&result, institution.ProfileData)

		var scholarships []comparisonScholarship
		if err := r.db.Table("scholarships").
			Select("id, title, value, deadline").
			Where("institution_id = ? AND status = ? AND deleted_at IS NULL", institution.ID, "published").
			Order("created_at DESC").Find(&scholarships).Error; err != nil {
			return result, err
		}
		for _, scholarship := range scholarships {
			result.Scholarships = append(result.Scholarships, scholarship)
		}
	}

	if err := r.applyComparisonReviews(&result, college.ID, result.InstitutionID); err != nil {
		return result, err
	}
	return result, nil
}

func applyInstitutionProfile(result *ComparisonCollege, raw []byte) {
	if len(raw) == 0 {
		return
	}
	var profile struct {
		Facilities []map[string]interface{} `json:"facilities_data"`
		Gallery    interface{}              `json:"gallery_data"`
	}
	if json.Unmarshal(raw, &profile) != nil {
		return
	}
	for _, facility := range profile.Facilities {
		for _, key := range []string{"heading", "title", "name"} {
			if value, ok := facility[key].(string); ok && value != "" {
				result.Facilities = append(result.Facilities, value)
				break
			}
		}
	}
	if profile.Gallery != nil {
		result.Gallery = profile.Gallery
	}
}

func (r *Repository) applyComparisonReviews(result *ComparisonCollege, collegeID, institutionID uint) error {
	query := r.db.Table("reviews").Where("is_published = ? AND deleted_at IS NULL", true)
	if institutionID > 0 {
		query = query.Where("college_id = ? OR institution_id = ?", collegeID, institutionID)
	} else {
		query = query.Where("college_id = ?", collegeID)
	}

	var rows []comparisonReviewRow
	if err := query.Order("created_at DESC").Find(&rows).Error; err != nil {
		return err
	}

	var ratingTotal float64
	for _, row := range rows {
		ratings := map[string]float64{}
		_ = json.Unmarshal(row.Ratings, &ratings)
		rating := averageRatings(ratings)
		ratingTotal += rating
		result.Reviews = append(result.Reviews, ComparisonReview{
			ID: row.ID, Rating: rating, Ratings: ratings, Pros: row.Pros,
			Cons: row.Cons, Course: row.Course, CreatedAt: row.CreatedAt,
		})
	}
	result.ReviewCount = len(rows)
	if len(rows) > 0 {
		result.Rating = math.Round(ratingTotal/float64(len(rows))*10) / 10
	}
	return nil
}

func averageRatings(ratings map[string]float64) float64 {
	if overall, ok := ratings["overall"]; ok {
		return overall
	}
	if len(ratings) == 0 {
		return 0
	}
	var total float64
	for _, rating := range ratings {
		total += rating
	}
	return math.Round(total/float64(len(ratings))*10) / 10
}
