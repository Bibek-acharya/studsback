package review

import (
	"testing"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/university"

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
