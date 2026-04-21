package projectshiksha

import (
	"time"
)

// ShikshaApplication represents a Project Shiksha scholarship application
type ShikshaApplication struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	
	// Personal Details
	FullName              string    `gorm:"size:100;not null" json:"full_name"`
	Gender                string    `gorm:"size:10;not null" json:"gender"`
	DOBBS                 string    `gorm:"size:20;not null" json:"dob_bs"`
	DOBAD                 string    `gorm:"size:50;not null" json:"dob_ad"`
	Age                   int       `json:"age"`
	Phone                 string    `gorm:"size:15;not null" json:"phone"`
	Email                 string    `gorm:"size:100" json:"email"`
	SEESchoolType         string    `gorm:"size:20;not null" json:"see_school_type"`
	OtherSchoolType       string    `gorm:"size:100" json:"other_school_type"`
	SchoolName            string    `gorm:"size:200;not null" json:"school_name"`
	
	// Photo URL (stored in MinIO)
	PhotoURL              string    `gorm:"size:500" json:"photo_url"`
	
	// Permanent Address
	PermProvince          string    `gorm:"size:50;not null" json:"perm_province"`
	PermDistrict          string    `gorm:"size:50;not null" json:"perm_district"`
	PermMunicipality      string    `gorm:"size:100;not null" json:"perm_municipality"`
	PermWard              int       `json:"perm_ward"`
	PermTole              string    `gorm:"size:100" json:"perm_tole"`
	
	// Temporary Address
	TempProvince          string    `gorm:"size:50;not null" json:"temp_province"`
	TempDistrict          string    `gorm:"size:50;not null" json:"temp_district"`
	TempMunicipality      string    `gorm:"size:100;not null" json:"temp_municipality"`
	TempWard              int       `json:"temp_ward"`
	TempTole              string    `gorm:"size:100" json:"temp_tole"`
	
	// Family Background
	GuardianName          string    `gorm:"size:100;not null" json:"guardian_name"`
	GuardianPhone         string    `gorm:"size:15;not null" json:"guardian_phone"`
	GuardianEmail         string    `gorm:"size:100" json:"guardian_email"`
	FatherOccupation      string    `gorm:"size:50;not null" json:"father_occupation"`
	MotherOccupation      string    `gorm:"size:50;not null" json:"mother_occupation"`
	FamilyIncome          int       `json:"family_income"`
	FamilyMembers         int       `json:"family_members"`
	
	// Documents (URLs stored in MinIO)
	SEEMarksheetURL       string    `gorm:"size:500" json:"see_marksheet_url"`
	CitizenshipURL        string    `gorm:"size:500" json:"citizenship_url"`
	
	// Payment Information
	PaymentStatus         string    `gorm:"size:20;default:'pending'" json:"payment_status"` // pending, completed, failed
	PaymentMethod         string    `gorm:"size:20" json:"payment_method"` // esewa, khalti, bank
	PaymentAmount         float64   `json:"payment_amount"`
	PaymentScreenshotURL  string    `gorm:"size:500" json:"payment_screenshot_url"`
	PaymentVerifiedAt     *time.Time `json:"payment_verified_at"`
	
	// Roll Number (generated after payment)
	RollNumber            string    `gorm:"size:20;uniqueIndex" json:"roll_number"`
	
	// Application Status
	Status                string    `gorm:"size:20;default:'submitted'" json:"status"` // submitted, under_review, accepted, rejected
	
	// User association (optional - for logged in users)
	UserID                *uint     `json:"user_id"`
}

// TableName overrides the table name
type ShikshaPayment struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	
	ApplicationID         uint      `gorm:"not null;index" json:"application_id"`
	PaymentMethod         string    `gorm:"size:20;not null" json:"payment_method"` // esewa, khalti, bank
	Amount                float64   `json:"amount"`
	Status                string    `gorm:"size:20;default:'pending'" json:"status"` // pending, completed, failed, verified
	TransactionID         string    `gorm:"size:100" json:"transaction_id"`
	ScreenshotURL         string    `gorm:"size:500" json:"screenshot_url"`
	VerifiedAt            *time.Time `json:"verified_at"`
	VerifiedBy            *uint     `json:"verified_by"`
}
