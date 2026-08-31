package institution

import (
	"testing"

	"gorm.io/gorm"
)

func TestCreateCourseRequest(t *testing.T) {
	db := setupServiceTestDB(t)
	// CourseApprovalRequest references InstitutionUser via FK
	if err := db.AutoMigrate(&InstitutionUser{}, &CourseApprovalRequest{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// Seed institution user for FK constraint
	inst := &InstitutionUser{
		InstitutionName:    "Test College",
		RegistrationNumber: "REG-001",
		Email:              "test@college.edu",
		Status:             "approved",
	}
	db.Create(inst)

	repo := NewRepository(db)

	req := &CourseApprovalRequest{
		InstitutionID: inst.ID,
		Title:         "BSc Computer Science",
		Description:   "A CS program",
		Duration:      "4 years",
		Level:         "undergraduate",
		Fee:           "200000",
		Eligibility:   "12th pass",
		Capacity:      60,
		Status:        "pending",
	}

	err := repo.CreateCourseRequest(req)
	if err != nil {
		t.Fatalf("CreateCourseRequest failed: %v", err)
	}

	if req.ID == 0 {
		t.Error("expected ID to be set after create")
	}

	// Verify persisted
	var found CourseApprovalRequest
	if err := db.First(&found, req.ID).Error; err != nil {
		t.Fatalf("failed to find created request: %v", err)
	}
	if found.Title != "BSc Computer Science" {
		t.Errorf("Title = %q, want %q", found.Title, "BSc Computer Science")
	}
	if found.InstitutionID != inst.ID {
		t.Errorf("InstitutionID = %d, want %d", found.InstitutionID, inst.ID)
	}
	if found.Status != "pending" {
		t.Errorf("Status = %q, want %q", found.Status, "pending")
	}
}

func TestFindCourseRequestsByInstitution(t *testing.T) {
	db := setupServiceTestDB(t)
	if err := db.AutoMigrate(&InstitutionUser{}, &CourseApprovalRequest{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	inst := &InstitutionUser{
		InstitutionName:    "Test College",
		RegistrationNumber: "REG-001",
		Email:              "test@college.edu",
		Status:             "approved",
	}
	db.Create(inst)

	otherInst := &InstitutionUser{
		InstitutionName:    "Other College",
		RegistrationNumber: "REG-002",
		Email:              "other@college.edu",
		Status:             "approved",
	}
	db.Create(otherInst)

	repo := NewRepository(db)

	// Create 3 requests for inst, 1 for otherInst
	for i := 0; i < 3; i++ {
		repo.CreateCourseRequest(&CourseApprovalRequest{
			InstitutionID: inst.ID,
			Title:         "Course",
			Status:        "pending",
		})
	}
	repo.CreateCourseRequest(&CourseApprovalRequest{
		InstitutionID: otherInst.ID,
		Title:         "Other Course",
		Status:        "pending",
	})

	// First page
	requests, total, err := repo.FindCourseRequestsByInstitution(inst.ID, 1, 10)
	if err != nil {
		t.Fatalf("FindCourseRequestsByInstitution failed: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(requests) != 3 {
		t.Errorf("len = %d, want 3", len(requests))
	}
	for _, r := range requests {
		if r.InstitutionID != inst.ID {
			t.Errorf("InstitutionID = %d, want %d", r.InstitutionID, inst.ID)
		}
	}

	// Pagination: page 1, limit 2
	requests, total, err = repo.FindCourseRequestsByInstitution(inst.ID, 1, 2)
	if err != nil {
		t.Fatalf("FindCourseRequestsByInstitution paginated failed: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(requests) != 2 {
		t.Errorf("len = %d, want 2", len(requests))
	}
}

func TestFindCourseRequestByID(t *testing.T) {
	db := setupServiceTestDB(t)
	if err := db.AutoMigrate(&InstitutionUser{}, &CourseApprovalRequest{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	inst := &InstitutionUser{
		InstitutionName:    "Test College",
		RegistrationNumber: "REG-001",
		Email:              "test@college.edu",
		Status:             "approved",
	}
	db.Create(inst)

	otherInst := &InstitutionUser{
		InstitutionName:    "Other College",
		RegistrationNumber: "REG-002",
		Email:              "other@college.edu",
		Status:             "approved",
	}
	db.Create(otherInst)

	repo := NewRepository(db)

	req := &CourseApprovalRequest{
		InstitutionID: inst.ID,
		Title:         "BSc CS",
		Status:        "pending",
	}
	repo.CreateCourseRequest(req)

	// Found by correct institution
	found, err := repo.FindCourseRequestByID(req.ID, inst.ID)
	if err != nil {
		t.Fatalf("FindCourseRequestByID failed: %v", err)
	}
	if found.ID != req.ID {
		t.Errorf("ID = %d, want %d", found.ID, req.ID)
	}

	// Not found by wrong institution
	_, err = repo.FindCourseRequestByID(req.ID, otherInst.ID)
	if err == nil {
		t.Error("expected error for wrong institution")
	}
}

func TestFindAllCourseRequests(t *testing.T) {
	db := setupServiceTestDB(t)
	if err := db.AutoMigrate(&InstitutionUser{}, &CourseApprovalRequest{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	inst := &InstitutionUser{
		InstitutionName:    "Test College",
		RegistrationNumber: "REG-001",
		Email:              "test@college.edu",
		Status:             "approved",
	}
	db.Create(inst)

	repo := NewRepository(db)

	for i := 0; i < 5; i++ {
		repo.CreateCourseRequest(&CourseApprovalRequest{
			InstitutionID: inst.ID,
			Title:         "Course",
			Status:        "pending",
		})
	}

	// All requests
	requests, total, err := repo.FindAllCourseRequests(1, 10)
	if err != nil {
		t.Fatalf("FindAllCourseRequests failed: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(requests) != 5 {
		t.Errorf("len = %d, want 5", len(requests))
	}

	// Pagination
	requests, total, err = repo.FindAllCourseRequests(1, 3)
	if err != nil {
		t.Fatalf("FindAllCourseRequests paginated failed: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(requests) != 3 {
		t.Errorf("len = %d, want 3", len(requests))
	}
}

func TestFindCourseRequestByIDAdmin(t *testing.T) {
	db := setupServiceTestDB(t)
	if err := db.AutoMigrate(&InstitutionUser{}, &CourseApprovalRequest{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	inst := &InstitutionUser{
		InstitutionName:    "Test College",
		RegistrationNumber: "REG-001",
		Email:              "test@college.edu",
		Status:             "approved",
	}
	db.Create(inst)

	repo := NewRepository(db)

	req := &CourseApprovalRequest{
		InstitutionID: inst.ID,
		Title:         "Admin Course",
		Status:        "pending",
	}
	repo.CreateCourseRequest(req)

	found, err := repo.FindCourseRequestByIDAdmin(req.ID)
	if err != nil {
		t.Fatalf("FindCourseRequestByIDAdmin failed: %v", err)
	}
	if found.ID != req.ID {
		t.Errorf("ID = %d, want %d", found.ID, req.ID)
	}
	if found.Title != "Admin Course" {
		t.Errorf("Title = %q, want %q", found.Title, "Admin Course")
	}

	// Non-existent ID
	_, err = repo.FindCourseRequestByIDAdmin(9999)
	if err == nil {
		t.Error("expected error for non-existent request")
	}
}

func TestUpdateCourseRequestStatus(t *testing.T) {
	db := setupServiceTestDB(t)
	if err := db.AutoMigrate(&InstitutionUser{}, &CourseApprovalRequest{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	inst := &InstitutionUser{
		InstitutionName:    "Test College",
		RegistrationNumber: "REG-001",
		Email:              "test@college.edu",
		Status:             "approved",
	}
	db.Create(inst)

	repo := NewRepository(db)

	req := &CourseApprovalRequest{
		InstitutionID: inst.ID,
		Title:         "Pending Course",
		Status:        "pending",
	}
	repo.CreateCourseRequest(req)

	// Approve
	err := repo.UpdateCourseRequestStatus(req.ID, "approved", 42, "")
	if err != nil {
		t.Fatalf("UpdateCourseRequestStatus approve failed: %v", err)
	}

	var found CourseApprovalRequest
	if err := db.Raw("SELECT * FROM course_approval_requests WHERE id = ?", req.ID).Scan(&found).Error; err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if found.Status != "approved" {
		t.Errorf("Status = %q, want %q", found.Status, "approved")
	}
	if found.ReviewedBy == nil || *found.ReviewedBy != 42 {
		t.Errorf("ReviewedBy = %v, want 42", found.ReviewedBy)
	}
	if found.ReviewedAt == nil {
		t.Error("ReviewedAt should be set")
	}

	// Reject with reason
	req2 := &CourseApprovalRequest{
		InstitutionID: inst.ID,
		Title:         "Another Course",
		Status:        "pending",
	}
	repo.CreateCourseRequest(req2)

	err = repo.UpdateCourseRequestStatus(req2.ID, "rejected", 42, "Incomplete information")
	if err != nil {
		t.Fatalf("UpdateCourseRequestStatus reject failed: %v", err)
	}

	if err := db.Raw("SELECT * FROM course_approval_requests WHERE id = ?", req2.ID).Scan(&found).Error; err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if found.Status != "rejected" {
		t.Errorf("Status = %q, want %q", found.Status, "rejected")
	}
	if found.RejectionReason != "Incomplete information" {
		t.Errorf("RejectionReason = %q, want %q", found.RejectionReason, "Incomplete information")
	}
}

func seedCourseApprovalRequest(db *gorm.DB, instID uint, status string) *CourseApprovalRequest {
	req := &CourseApprovalRequest{
		InstitutionID: instID,
		Title:         "Test Course",
		Description:   "Test Description",
		Duration:      "4 years",
		Level:         "undergraduate",
		Fee:           "200000",
		Eligibility:   "12th pass",
		Capacity:      60,
		Status:        status,
	}
	db.Create(req)
	return req
}

func seedInstitutionUser(db *gorm.DB, regNum string) *InstitutionUser {
	inst := &InstitutionUser{
		InstitutionName:    "Test College " + regNum,
		RegistrationNumber: regNum,
		Email:              regNum + "@college.edu",
		Status:             "approved",
	}
	db.Create(inst)
	return inst
}

func TestFindPublicInstitutionsFiltersByGlobalCourseID(t *testing.T) {
	db := setupServiceTestDB(t)
	if err := db.AutoMigrate(&InstitutionUser{}, &InstitutionSettings{}, &InstitutionProgram{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	offeringInstitution := &InstitutionUser{
		InstitutionName:    "Offering College",
		RegistrationNumber: "REG-OFFERING",
		Email:              "offering@college.edu",
		Status:             "approved",
		ProfileStatus:      "published",
	}
	otherInstitution := &InstitutionUser{
		InstitutionName:    "Other College",
		RegistrationNumber: "REG-OTHER",
		Email:              "other-public@college.edu",
		Status:             "approved",
		ProfileStatus:      "published",
	}
	if err := db.Create(offeringInstitution).Error; err != nil {
		t.Fatalf("create offering institution: %v", err)
	}
	if err := db.Create(otherInstitution).Error; err != nil {
		t.Fatalf("create other institution: %v", err)
	}

	if err := db.Exec(
		"INSERT INTO institution_programs (name, institution_id, global_course_id, status) VALUES (?, ?, ?, ?)",
		"Global Course", offeringInstitution.ID, 42, "active",
	).Error; err != nil {
		t.Fatalf("create institution program: %v", err)
	}

	repo := NewRepository(db)
	institutions, total, err := repo.FindPublicInstitutions(
		1, 18, "", "", "", nil, nil, nil, nil, []string{"42"},
	)
	if err != nil {
		t.Fatalf("FindPublicInstitutions failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(institutions) != 1 || institutions[0].ID != offeringInstitution.ID {
		t.Fatalf("institutions = %#v, want only institution %d", institutions, offeringInstitution.ID)
	}
}
