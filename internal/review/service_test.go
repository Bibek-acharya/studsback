package review

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/university"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestToReviewResponseIncludesReviewerIdentity(t *testing.T) {
	cases := []struct {
		name     string
		user     auth.User
		wantName string
		wantInit string
	}{
		{
			name:     "full name",
			user:     auth.User{FirstName: "Ada", LastName: "Lovelace"},
			wantName: "Ada Lovelace",
			wantInit: "AL",
		},
		{
			name:     "first name only",
			user:     auth.User{FirstName: "Ada"},
			wantName: "Ada",
			wantInit: "A",
		},
		{
			name:     "blank names",
			user:     auth.User{},
			wantName: "",
			wantInit: "U",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			response := toReviewResponse(&Review{UserID: 42, User: tt.user})
			if response.UserName != tt.wantName {
				t.Fatalf("UserName = %q, want %q", response.UserName, tt.wantName)
			}
			if response.UserInitials != tt.wantInit {
				t.Fatalf("UserInitials = %q, want %q", response.UserInitials, tt.wantInit)
			}
		})
	}
}

func TestSubmitUniversityReviewPersistsSeparateProsAndCons(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auth.User{}, &university.University{}, &Review{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&university.University{ID: 7, Name: "Lumbini University"}).Error; err != nil {
		t.Fatal(err)
	}

	user := auth.User{Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	service := NewService(NewRepository(db))
	created, err := service.SubmitUniversityReview(user.ID, CreateUniversityReviewRequest{
		UniversityID: 7,
		Rating:       4,
		Pros:         "Excellent faculty",
		Cons:         "Limited housing",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.UserName != "Ada Lovelace" || created.UserInitials != "AL" {
		t.Fatalf("created identity = %q/%q", created.UserName, created.UserInitials)
	}

	stored, err := service.GetUserUniversityReview(user.ID, 7)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Pros != "Excellent faculty" || stored.Cons != "Limited housing" {
		t.Fatalf("stored pros/cons = %q/%q", stored.Pros, stored.Cons)
	}
	if stored.SummaryTitle != "Excellent faculty" {
		t.Fatalf("SummaryTitle = %q, want pros compatibility value", stored.SummaryTitle)
	}
	if stored.UserName != "Ada Lovelace" || stored.UserInitials != "AL" {
		t.Fatalf("stored identity = %q/%q", stored.UserName, stored.UserInitials)
	}
}

func TestUpdateUniversityReview(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auth.User{}, &university.University{}, &Review{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&university.University{ID: 7, Name: "Lumbini University"}).Error; err != nil {
		t.Fatal(err)
	}

	user := auth.User{Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	service := NewService(NewRepository(db))
	if _, err := service.SubmitUniversityReview(user.ID, CreateUniversityReviewRequest{
		UniversityID: 7,
		Rating:       4,
		Pros:         "Excellent faculty",
		Cons:         "Limited housing",
	}); err != nil {
		t.Fatal(err)
	}

	rating := 5.0
	pros := "Outstanding faculty"
	cons := "Very limited housing"
	updated, err := service.UpdateUniversityReview(user.ID, 7, UpdateUniversityReviewRequest{
		Rating: &rating,
		Pros:   &pros,
		Cons:   &cons,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Ratings["overall"] != 5 || updated.Pros != pros || updated.Cons != cons {
		t.Fatalf("updated review = rating %v, pros %q, cons %q", updated.Ratings["overall"], updated.Pros, updated.Cons)
	}
	if updated.UserName != "Ada Lovelace" {
		t.Fatalf("UserName = %q, want Ada Lovelace", updated.UserName)
	}
}

func TestUpdateUniversityReviewChangesOnlySuppliedFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auth.User{}, &university.University{}, &Review{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&university.University{ID: 7, Name: "Lumbini University"}).Error; err != nil {
		t.Fatal(err)
	}
	user := auth.User{Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	review := &Review{
		UserID:       user.ID,
		UniversityID: 7,
		Ratings:      []byte(`{"overall":4}`),
		Pros:         "Excellent faculty",
		Cons:         "Limited housing",
		SummaryTitle: "Original summary",
		Email:        "original@example.com",
		IsPublished:  true,
	}
	if err := db.Create(review).Error; err != nil {
		t.Fatal(err)
	}

	pros := "Outstanding faculty"
	updated, err := NewService(NewRepository(db)).UpdateUniversityReview(user.ID, 7, UpdateUniversityReviewRequest{Pros: &pros})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Pros != pros {
		t.Fatalf("Pros = %q, want %q", updated.Pros, pros)
	}

	var stored Review
	if err := db.First(&stored, review.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Cons != "Limited housing" || stored.SummaryTitle != "Original summary" || stored.Email != "original@example.com" || !stored.IsPublished {
		t.Fatalf("unrelated fields changed: cons=%q summary=%q email=%q published=%v", stored.Cons, stored.SummaryTitle, stored.Email, stored.IsPublished)
	}
}

func TestUniversityReviewHasCompositeUniqueIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Review{}); err != nil {
		t.Fatal(err)
	}
	first := &Review{UserID: 1, UniversityID: 7, Ratings: []byte(`{"overall":4}`), Pros: "Excellent faculty", Cons: "Limited housing", SummaryTitle: "Original"}
	if err := db.Create(first).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := &Review{UserID: 1, UniversityID: 7, Ratings: []byte(`{"overall":5}`), Pros: "Outstanding faculty", Cons: "Very limited housing", SummaryTitle: "Duplicate"}
	if err := db.Create(duplicate).Error; err == nil {
		t.Fatal("duplicate user/university review was accepted")
	}
}

func TestIsDuplicateReviewError(t *testing.T) {
	if !isDuplicateReviewError(errors.New("ERROR: duplicate key value violates unique constraint")) {
		t.Fatal("duplicate constraint error was not recognized")
	}
}

func TestUpdateUniversityReviewHandlerRejectsEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service := NewService(nil)
	handler := NewHandler(service)
	router.PUT("/api/v1/user/university-reviews/:universityId", func(c *gin.Context) {
		c.Set("user_id", uint(1))
		handler.UpdateUniversityReview(c)
	})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/university-reviews/7", bytes.NewReader(nil))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "At least one review field is required" {
		t.Fatalf("error = %q", body.Error)
	}
}

func TestUpdateUniversityReviewTranslatesSaveNotFound(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auth.User{}, &university.University{}, &Review{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&university.University{ID: 7, Name: "Lumbini University"}).Error; err != nil {
		t.Fatal(err)
	}
	user := auth.User{Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(db))
	created, err := service.SubmitUniversityReview(user.ID, CreateUniversityReviewRequest{
		UniversityID: 7,
		Rating:       4,
		Pros:         "Excellent faculty",
		Cons:         "Limited housing",
	})
	if err != nil {
		t.Fatal(err)
	}

	callbackName := "test_delete_before_review_update"
	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		tx.Session(&gorm.Session{SkipHooks: true}).Where("id = ?", created.ID).Delete(&Review{})
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Callback().Update().Remove(callbackName)

	pros := "Outstanding faculty"
	_, err = service.UpdateUniversityReview(user.ID, 7, UpdateUniversityReviewRequest{Pros: &pros})
	if err == nil || err.Error() != "review not found" {
		t.Fatalf("error = %v, want review not found", err)
	}
}
