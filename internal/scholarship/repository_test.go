package scholarship

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCreateProviderApplication_PersistsFullFormData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE provider_applications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scholarship_id INTEGER,
			user_id INTEGER,
			full_name TEXT,
			first_name TEXT,
			last_name TEXT,
			email TEXT,
			phone_number TEXT,
			gender TEXT,
			ethnicity TEXT,
			ethnicity_other TEXT,
			date_of_birth_bs TEXT,
			date_of_birth_ad DATETIME,
			age INTEGER,
			photo_url TEXT,
			see_gpa TEXT,
			school_type TEXT,
			school_name TEXT,
			school_province TEXT,
			school_district TEXT,
			school_municipality TEXT,
			school_tole TEXT,
			province TEXT,
			district TEXT,
			permanent_province TEXT,
			permanent_district TEXT,
			permanent_municipality TEXT,
			permanent_ward TEXT,
			permanent_tole TEXT,
			temporary_province TEXT,
			temporary_district TEXT,
			temporary_municipality TEXT,
			temporary_ward TEXT,
			temporary_tole TEXT,
			guardian_name TEXT,
			guardian_phone TEXT,
			guardian_email TEXT,
			father_occupation TEXT,
			father_occupation_other TEXT,
			mother_occupation TEXT,
			mother_occupation_other TEXT,
			family_monthly_income REAL,
			family_members_count INTEGER,
			status TEXT,
			evaluation_notes TEXT,
			documents BLOB,
			personal_statement TEXT,
			stream TEXT,
			exam_center TEXT,
			gpa REAL,
			created_at DATETIME,
			updated_at DATETIME
		);
	`); err != nil {
		t.Fatalf("create provider_applications table: %v", err)
	}

	repo := NewRepository(db)
	now := time.Date(2026, 5, 1, 15, 30, 0, 0, time.UTC)
	application := &ScholarshipApplication{
		FullName:              "Ram Bahadur Thapa",
		Gender:                "Male",
		Ethnicity:             "Limbu",
		EthnicityOther:        "Other ethnicity",
		DateOfBirthBS:         "2067-01-12",
		DateOfBirthAD:         now,
		Age:                   16,
		PhoneNumber:           "9874563210",
		Email:                 "student@example.com",
		PhotoURL:              "https://example.com/photo.jpg",
		SEEGPA:                "3.45",
		SchoolType:            "Public",
		SchoolName:            "Nobel College",
		SchoolProvince:        "Bagmati Province",
		SchoolDistrict:        "Dolakha",
		SchoolMunicipality:    "Jiri Municipality",
		SchoolTole:            "School Tole",
		PermanentProvince:     "Bagmati Province",
		PermanentDistrict:     "Dhading",
		PermanentMunicipality: "Galchhi Gaunpalika",
		PermanentWard:         "2",
		PermanentTole:         "Permanent Tole",
		TemporaryProvince:     "Bagmati Province",
		TemporaryDistrict:     "Dhading",
		TemporaryMunicipality: "Galchhi Gaunpalika",
		TemporaryWard:         "2",
		TemporaryTole:         "Temporary Tole",
		GuardianName:          "Ram Bahadur's Guardian",
		GuardianPhone:         "9874563210",
		GuardianEmail:         "guardian@example.com",
		FatherOccupation:      "Business/Commerce",
		FatherOccupationOther: "Furniture",
		MotherOccupation:      "Agriculture/Farming",
		MotherOccupationOther: "Small shop",
		FamilyMonthlyIncome:   12500,
		FamilyMembersCount:    17,
		Stream:                "Science",
		ExamCenter:            "Nobel College",
	}

	if err := repo.CreateProviderApplication(2, application); err != nil {
		t.Fatalf("CreateProviderApplication() error = %v", err)
	}

	var row struct {
		SchoolName        string
		SchoolDistrict    string
		PermanentProvince string
		GuardianName      string
		FatherOccupation  string
		FamilyIncome      float64 `gorm:"column:family_monthly_income"`
		ExamCenter        string
	}

	if err := db.Table("provider_applications").
		Select("school_name", "school_district", "permanent_province", "guardian_name", "father_occupation", "family_monthly_income", "exam_center").
		Where("scholarship_id = ?", 2).
		Scan(&row).Error; err != nil {
		t.Fatalf("scan provider application: %v", err)
	}

	if row.SchoolName != "Nobel College" {
		t.Fatalf("SchoolName = %q, want %q", row.SchoolName, "Nobel College")
	}
	if row.SchoolDistrict != "Dolakha" {
		t.Fatalf("SchoolDistrict = %q, want %q", row.SchoolDistrict, "Dolakha")
	}
	if row.PermanentProvince != "Bagmati Province" {
		t.Fatalf("PermanentProvince = %q, want %q", row.PermanentProvince, "Bagmati Province")
	}
	if row.GuardianName != "Ram Bahadur's Guardian" {
		t.Fatalf("GuardianName = %q, want %q", row.GuardianName, "Ram Bahadur's Guardian")
	}
	if row.FatherOccupation != "Business/Commerce" {
		t.Fatalf("FatherOccupation = %q, want %q", row.FatherOccupation, "Business/Commerce")
	}
	if row.FamilyIncome != 12500 {
		t.Fatalf("FamilyIncome = %v, want %v", row.FamilyIncome, 12500)
	}
	if row.ExamCenter != "Nobel College" {
		t.Fatalf("ExamCenter = %q, want %q", row.ExamCenter, "Nobel College")
	}
}
