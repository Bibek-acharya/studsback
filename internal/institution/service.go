package institution

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"studsphere/backend/internal/shared/utils"
	"studsphere/backend/internal/system"
)

type Service struct {
	repo      *Repository
	systemSvc *system.Service
}

func NewService(repo *Repository, systemSvc *system.Service) *Service {
	return &Service{repo: repo, systemSvc: systemSvc}
}

func (s *Service) GetDashboard(instID uint) (*DashboardResponse, error) {
	totalPrograms, err := s.repo.CountProgramsByInstitution(instID)
	if err != nil {
		return nil, err
	}

	totalStudents, err := s.repo.CountDistinctStudentsByInstitution(instID)
	if err != nil {
		return nil, err
	}

	activeEntrances, err := s.repo.CountActiveEntrances(instID)
	if err != nil {
		return nil, err
	}

	pendingBookings, err := s.repo.CountPendingBookings(instID)
	if err != nil {
		return nil, err
	}

	unreadMessages, err := s.repo.CountUnreadMessages(instID)
	if err != nil {
		return nil, err
	}

	return &DashboardResponse{
		TotalPrograms:   totalPrograms,
		TotalStudents:   totalStudents,
		ActiveEntrances: activeEntrances,
		PendingBookings: pendingBookings,
		UnreadMessages:  unreadMessages,
	}, nil
}

func (s *Service) GetAnalytics(instID uint) (*AnalyticsResponse, error) {
	programs, err := s.repo.FindAllProgramsByInstitution(instID)
	if err != nil {
		return nil, err
	}

	entranceCount, err := s.repo.CountEntrancesByInstitution(instID)
	if err != nil {
		return nil, err
	}

	programStats := make([]ProgramStat, len(programs))
	for i, p := range programs {
		programStats[i] = ProgramStat{
			ID:        p.ID,
			Name:      p.Name,
			Status:    p.Status,
			Entrances: entranceCount,
		}
	}

	totalApplicants, err := s.repo.CountTotalApplicants(instID)
	if err != nil {
		return nil, err
	}

	return &AnalyticsResponse{
		ProgramStats:    programStats,
		TotalApplicants: totalApplicants,
	}, nil
}

func (s *Service) GetProfile(instID uint) (*ProfileResponse, error) {
	user, err := s.repo.FindInstitutionUserByID(instID)
	if err != nil {
		return nil, err
	}

	var pd ProfileData
	if user.ProfileData != nil && *user.ProfileData != "" {
		json.Unmarshal([]byte(*user.ProfileData), &pd)
	}

	subType := "free"
	var sub struct {
		ExpireDate *time.Time `json:"expire_date"`
	}
	s.repo.db.Table("institution_subscriptions").
		Where("institution_id = ?", instID).
		Select("expire_date").
		Scan(&sub)
	if sub.ExpireDate != nil && sub.ExpireDate.After(time.Now()) {
		subType = "premium"
	}

	return &ProfileResponse{
		SubscriptionType:     subType,
		ID:                   user.ID,
		InstitutionName:      user.InstitutionName,
		Email:                user.Email,
		RegistrationNumber:   user.RegistrationNumber,
		Role:                 user.Role,
		Location:             user.District,
		Website:              user.WebsiteURL,
		ContactEmail:         user.ContactEmail,
		ContactPhone:         user.ContactPhone,
		MapURL:               user.MapURL,
		FacebookURL:          user.FacebookURL,
		InstagramURL:         user.InstagramURL,
		TiktokURL:            user.TiktokURL,
		YoutubeURL:           user.YoutubeURL,
		LinkedinURL:          user.LinkedinURL,
		LogoURL:              user.LogoURL,
		BannerURL:            user.BannerURL,
		About:                user.About,
		Vision:               user.Vision,
		Mission:              user.Mission,
		Affiliation:          user.Affiliation,
		Videos:               pd.Videos,
		OverviewData:         pd.OverviewData,
		LeadershipData:       pd.LeadershipData,
		CoursesData:          pd.CoursesData,
		ProgramsData:         pd.ProgramsData,
		FacilitiesData:       pd.FacilitiesData,
		AlumniData:           pd.AlumniData,
		DownloadsData:        pd.DownloadsData,
		GalleryData:          pd.GalleryData,
		WhatsNewData:         pd.WhatsNewData,
		EligibilityData:      pd.EligibilityData,
		AdmissionProcessData: pd.AdmissionProcessData,
		ScholarshipsData:     pd.ScholarshipsData,
		FaqsData:             pd.FaqsData,
		ContactPersonsData:   pd.ContactPersonsData,
		BrochureData:         pd.BrochureData,
	}, nil
}

func (s *Service) UpdateProfile(instID uint, req UpdateProfileRequest) (*ProfileResponse, error) {
	user, err := s.repo.FindInstitutionUserByID(instID)
	if err != nil {
		return nil, err
	}

	if req.InstitutionName != "" {
		user.InstitutionName = req.InstitutionName
	}
	if req.RegistrationNumber != "" {
		user.RegistrationNumber = req.RegistrationNumber
	}
	if req.Location != "" {
		user.District = req.Location
	}
	if req.Website != "" {
		user.WebsiteURL = req.Website
	}
	if req.LogoURL != "" {
		if utils.IsDataURI(req.LogoURL) {
			url, err := utils.SaveDataURI(req.LogoURL, "institution/logo")
			if err != nil {
				return nil, fmt.Errorf("failed to save logo data URI: %w", err)
			}
			user.LogoURL = url
		} else {
			user.LogoURL = req.LogoURL
		}
	}
	if req.BannerURL != "" {
		if utils.IsDataURI(req.BannerURL) {
			url, err := utils.SaveDataURI(req.BannerURL, "institution/banner")
			if err != nil {
				return nil, fmt.Errorf("failed to save banner data URI: %w", err)
			}
			user.BannerURL = url
		} else {
			user.BannerURL = req.BannerURL
		}
	}
	if req.About != "" {
		user.About = req.About
	}
	if req.Vision != "" {
		user.Vision = req.Vision
	}
	if req.Mission != "" {
		user.Mission = req.Mission
	}
	if req.ContactEmail != "" {
		user.ContactEmail = req.ContactEmail
	}
	if req.ContactPhone != "" {
		user.ContactPhone = req.ContactPhone
	}
	if req.Affiliation != "" {
		user.Affiliation = req.Affiliation
	}
	if req.MapURL != "" {
		user.MapURL = req.MapURL
	}
	if req.FacebookURL != "" {
		user.FacebookURL = req.FacebookURL
	}
	if req.InstagramURL != "" {
		user.InstagramURL = req.InstagramURL
	}
	if req.TiktokURL != "" {
		user.TiktokURL = req.TiktokURL
	}
	if req.YoutubeURL != "" {
		user.YoutubeURL = req.YoutubeURL
	}
	if req.LinkedinURL != "" {
		user.LinkedinURL = req.LinkedinURL
	}
	if req.Videos != nil || req.OverviewData != nil || req.LeadershipData != nil ||
		req.CoursesData != nil || req.ProgramsData != nil || req.FacilitiesData != nil ||
		req.AlumniData != nil || req.DownloadsData != nil || req.GalleryData != nil ||
		req.WhatsNewData != nil || req.EligibilityData != nil || req.AdmissionProcessData != nil ||
		req.ScholarshipsData != nil || req.FaqsData != nil || req.ContactPersonsData != nil ||
		req.BrochureData != nil {

		var existing map[string]interface{}
		if user.ProfileData != nil && *user.ProfileData != "" {
			json.Unmarshal([]byte(*user.ProfileData), &existing)
		}
		if existing == nil {
			existing = make(map[string]interface{})
		}

		setField(&existing, "videos", req.Videos)
		setField(&existing, "overview_data", req.OverviewData)
		setField(&existing, "leadership_data", req.LeadershipData)
		setField(&existing, "courses_data", req.CoursesData)
		setField(&existing, "programs_data", req.ProgramsData)
		setField(&existing, "facilities_data", req.FacilitiesData)
		setField(&existing, "alumni_data", req.AlumniData)
		setField(&existing, "downloads_data", req.DownloadsData)
		setField(&existing, "gallery_data", req.GalleryData)
		setField(&existing, "whats_new_data", req.WhatsNewData)
		setField(&existing, "eligibility_data", req.EligibilityData)
		setField(&existing, "admission_process_data", req.AdmissionProcessData)
		setField(&existing, "scholarships_data", req.ScholarshipsData)
		setField(&existing, "faqs_data", req.FaqsData)
		setField(&existing, "contact_persons_data", req.ContactPersonsData)
		setField(&existing, "brochure_data", req.BrochureData)

		if data, err := json.Marshal(existing); err == nil {
			str := string(data)
			user.ProfileData = &str
		}
	}

	if err := s.repo.SaveInstitutionUser(user); err != nil {
		return nil, err
	}

	return s.GetProfile(instID)
}

func setField(data *map[string]interface{}, key string, val interface{}) {
	if val != nil {
		switch v := val.(type) {
		case string:
			if v != "" {
				(*data)[key] = v
			}
		default:
			(*data)[key] = v
		}
	}
}

func (s *Service) UpdatePassword(instID uint, req UpdatePasswordRequest) error {
	user, err := s.repo.FindInstitutionUserByID(instID)
	if err != nil {
		return errors.New("institution not found")
	}

	if err := user.CheckPassword(req.CurrentPassword); err != nil {
		return errors.New("current password is incorrect")
	}

	if err := user.HashPassword(req.NewPassword); err != nil {
		return errors.New("failed to hash password")
	}

	return s.repo.SaveInstitutionUser(user)
}

func (s *Service) GetPrograms(instID uint, page, limit int) ([]InstitutionProgram, int64, error) {
	return s.repo.FindProgramsByInstitution(instID, page, limit)
}

func (s *Service) GetProgramByID(instID, id uint) (*InstitutionProgram, error) {
	return s.repo.FindProgramByIDAndInstitution(id, instID)
}

func (s *Service) GetProgramByIDOnly(id uint) (*InstitutionProgram, error) {
	return s.repo.FindProgramByID(id)
}

func (s *Service) CreateProgram(instID uint, req CreateProgramRequest) (*InstitutionProgram, error) {
	status := "active"
	if req.Status != "" {
		status = req.Status
	}

	program := &InstitutionProgram{
		InstitutionID:       instID,
		InstitutionName:     req.InstitutionName,
		InstitutionLocation: req.InstitutionLocation,
		InstitutionLink:     req.InstitutionLink,
		Name:                req.Name,
		Description:         req.Description,
		Duration:            req.Duration,
		Fee:                 req.Fee,
		Eligibility:         req.Eligibility,
		Capacity:            req.Capacity,
		BannerURL:           req.BannerURL,
		Status:              status,
	}

	if req.Data != nil {
		data, _ := json.Marshal(req.Data)
		str := string(data)
		program.Data = &str
	}

	// If linked to a global course, set the reference
	if req.GlobalCourseID != nil && *req.GlobalCourseID > 0 {
		globalCourse, err := s.repo.FindGlobalCourseByID(*req.GlobalCourseID)
		if err == nil && globalCourse != nil {
			program.GlobalCourseID = req.GlobalCourseID

			overrides := map[string]interface{}{}

			if title, ok := globalCourse["title"].(string); ok && title != "" {
				if program.Name == "" {
					program.Name = title
				} else if program.Name != title {
					overrides["name"] = program.Name
				}
			}
			if desc, ok := globalCourse["description"].(string); ok {
				if program.Description == "" {
					program.Description = desc
				} else if program.Description != desc {
					overrides["description"] = program.Description
				}
			}
			if dur, ok := globalCourse["duration"].(string); ok {
				if program.Duration == "" {
					program.Duration = dur
				} else if program.Duration != dur {
					overrides["duration"] = program.Duration
				}
			}

			if len(overrides) > 0 {
				data, _ := json.Marshal(overrides)
				str := string(data)
				program.Overrides = &str
			}
		}
	}

	if err := s.repo.CreateProgram(program); err != nil {
		return nil, err
	}

	// If no global course linked, create a draft course for super admin review
	if req.GlobalCourseID == nil || *req.GlobalCourseID == 0 {
		draftID, err := s.repo.CreateCourseFromProgram(program)
		if err != nil {
			// Log but don't fail - program was created successfully
			fmt.Printf("[WARN] Failed to create draft course from program %d: %v\n", program.ID, err)
		} else {
			fmt.Printf("[INFO] Created draft course %d from program %d\n", draftID, program.ID)
		}
	}

	return program, nil
}

func (s *Service) UpdateProgram(instID, id uint, req UpdateProgramRequest) (*InstitutionProgram, error) {
	program, err := s.repo.FindProgramByIDAndInstitution(id, instID)
	if err != nil {
		return nil, errors.New("program not found")
	}

	if req.Name != "" {
		program.Name = req.Name
	}
	if req.Description != "" {
		program.Description = req.Description
	}
	if req.Duration != "" {
		program.Duration = req.Duration
	}
	if req.Fee != "" {
		program.Fee = req.Fee
	}
	if req.Eligibility != "" {
		program.Eligibility = req.Eligibility
	}
	if req.Capacity > 0 {
		program.Capacity = req.Capacity
	}
	if req.BannerURL != "" {
		program.BannerURL = req.BannerURL
	}
	if req.Data != nil {
		data, _ := json.Marshal(req.Data)
		str := string(data)
		program.Data = &str
	}
	if req.Status != "" {
		program.Status = req.Status
	}
	if req.InstitutionName != "" {
		program.InstitutionName = req.InstitutionName
	}
	if req.InstitutionLocation != "" {
		program.InstitutionLocation = req.InstitutionLocation
	}
	if req.InstitutionLink != "" {
		program.InstitutionLink = req.InstitutionLink
	}
	if req.GlobalCourseID != nil {
		program.GlobalCourseID = req.GlobalCourseID
	}

	// Recalculate overrides if program is linked to a global course
	if program.GlobalCourseID != nil && *program.GlobalCourseID > 0 {
		s.recalculateOverrides(program)
	} else {
		program.Overrides = nil
		program.NullifiedFields = nil
	}

	if err := s.repo.SaveProgram(program); err != nil {
		return nil, err
	}

	return program, nil
}

func (s *Service) recalculateOverrides(program *InstitutionProgram) {
	if program.GlobalCourseID == nil || *program.GlobalCourseID == 0 {
		return
	}
	globalCourse, err := s.repo.FindGlobalCourseByID(*program.GlobalCourseID)
	if err != nil || globalCourse == nil {
		return
	}

	overrides := map[string]interface{}{}
	gcName, _ := globalCourse["title"].(string)
	gcDesc, _ := globalCourse["description"].(string)
	gcDur, _ := globalCourse["duration"].(string)
	gcFee, _ := globalCourse["est_fee"].(string)

	if program.Name != "" && program.Name != gcName {
		overrides["name"] = program.Name
	}
	if program.Description != "" && program.Description != gcDesc {
		overrides["description"] = program.Description
	}
	if program.Duration != "" && program.Duration != gcDur {
		overrides["duration"] = program.Duration
	}
	if program.Fee != "" && program.Fee != gcFee {
		overrides["fee"] = program.Fee
	}

	if len(overrides) > 0 {
		data, _ := json.Marshal(overrides)
		str := string(data)
		program.Overrides = &str
	} else {
		program.Overrides = nil
	}
}

func (s *Service) DeleteProgram(instID, id uint) error {
	return s.repo.DeleteProgram(id, instID)
}

func (s *Service) DeleteProgramByID(id uint) error {
	return s.repo.DeleteProgramByID(id)
}

func (s *Service) GetMedia(instID uint) ([]InstitutionMedia, error) {
	return s.repo.FindMediaByInstitution(instID)
}

func (s *Service) CreateMedia(instID uint, req CreateMediaRequest) (*InstitutionMedia, error) {
	media := &InstitutionMedia{
		InstitutionID: instID,
		URL:           req.URL,
		Type:          req.Type,
		Title:         req.Title,
	}

	if err := s.repo.CreateMedia(media); err != nil {
		return nil, err
	}

	return media, nil
}

func (s *Service) DeleteMedia(instID, id uint) error {
	return s.repo.DeleteMedia(id, instID)
}

func (s *Service) GetCounsellingSessions(instID uint) ([]InstitutionCounsellingSession, error) {
	return s.repo.FindCounsellingSessionsByInstitution(instID)
}

func (s *Service) GetPublicCounsellingSessions(instID uint) ([]PublicCounsellingSessionResponse, error) {
	sessions, err := s.repo.FindUpcomingSessionsByInstitution(instID)
	if err != nil {
		return nil, err
	}

	resp := make([]PublicCounsellingSessionResponse, len(sessions))
	for i, session := range sessions {
		resp[i] = PublicCounsellingSessionResponse{
			ID:             session.ID,
			Title:          session.Title,
			Description:    session.Description,
			ScheduledAt:    session.ScheduledAt.Format(time.RFC3339),
			Duration:       session.Duration,
			MaxSeats:       session.MaxSeats,
			BookedSeats:    session.BookedSeats,
			AvailableSeats: session.MaxSeats - session.BookedSeats,
			Status:         session.Status,
		}
	}

	return resp, nil
}

func (s *Service) GetCounsellingBookings(instID uint) ([]InstitutionCounsellingBooking, error) {
	return s.repo.FindCounsellingBookingsByInstitution(instID)
}

func (s *Service) CreateCounsellingSession(instID uint, req CreateCounsellingSessionRequest) (*InstitutionCounsellingSession, error) {
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		scheduledAt, err = time.Parse("2006-01-02T15:04", req.ScheduledAt)
		if err != nil {
			return nil, errors.New("invalid scheduled_at format, use RFC3339 or YYYY-MM-DDTHH:mm")
		}
	}

	session := &InstitutionCounsellingSession{
		InstitutionID: instID,
		Title:         req.Title,
		Description:   req.Description,
		ScheduledAt:   scheduledAt,
		Duration:      req.Duration,
		MaxSeats:      req.MaxSeats,
		Status:        "scheduled",
	}

	if err := s.repo.CreateCounsellingSession(session); err != nil {
		return nil, err
	}

	return session, nil
}

func (s *Service) DeleteCounsellingSession(instID, id uint) error {
	session, err := s.repo.FindCounsellingSessionByID(id, instID)
	if err != nil {
		return errors.New("session not found")
	}
	return s.repo.DeleteCounsellingSession(session)
}

func (s *Service) UpdateBookingStatus(instID, id uint, status string) (*InstitutionCounsellingBooking, error) {
	booking, err := s.repo.FindBookingByIDWithSession(id, instID)
	if err != nil {
		return nil, errors.New("booking not found")
	}

	validStatuses := map[string]bool{
		"pending": true, "confirmed": true, "cancelled": true, "completed": true,
	}
	if !validStatuses[status] {
		return nil, errors.New("invalid status")
	}

	booking.Status = status

	if err := s.repo.SaveBooking(booking); err != nil {
		return nil, err
	}

	return booking, nil
}

func (s *Service) CreatePublicBooking(userID uint, req PublicCounsellingBookingRequest) (*InstitutionCounsellingBooking, error) {
	if req.SessionMode != "online" && req.SessionMode != "in_person" {
		return nil, errors.New("session_mode must be 'online' or 'in_person'")
	}

	session, err := s.repo.FindCounsellingSessionByIDOnly(req.SessionID)
	if err != nil {
		return nil, errors.New("session not found")
	}

	if session.Status != "scheduled" {
		return nil, errors.New("session is not available for booking")
	}

	if session.BookedSeats >= session.MaxSeats {
		return nil, errors.New("no available seats in this session")
	}

	if s.repo.CheckUserSessionBooking(userID, req.SessionID) {
		return nil, errors.New("you have already booked this session")
	}

	booking := &InstitutionCounsellingBooking{
		SessionID:        req.SessionID,
		UserID:           userID,
		Status:           "pending",
		Notes:            req.StudentNotes,
		StudentName:      req.StudentName,
		StudentPhone:     req.StudentPhone,
		StudentEmail:     req.StudentEmail,
		ProgramLevel:     req.ProgramLevel,
		InterestedCourse: req.InterestedCourse,
		SessionMode:      req.SessionMode,
	}

	if err := s.repo.CreateBooking(booking); err != nil {
		return nil, errors.New("failed to create booking")
	}

	if err := s.repo.IncrementBookedSeats(req.SessionID); err != nil {
		return nil, errors.New("failed to update seat count")
	}

	return booking, nil
}

func (s *Service) GetEntrances(instID uint, status string, page, limit int) ([]InstitutionEntrance, int64, error) {
	return s.repo.FindEntrancesByInstitution(instID, status, page, limit)
}

func (s *Service) GetEntranceByID(instID, id uint) (*InstitutionEntrance, error) {
	return s.repo.FindEntranceByIDAndInstitution(id, instID)
}

func (s *Service) GetEntranceByIDOnly(id uint) (*InstitutionEntrance, error) {
	return s.repo.FindEntranceByID(id)
}

func (s *Service) CreateEntrance(instID uint, req CreateEntranceRequest) (*InstitutionEntrance, error) {
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, errors.New("invalid date format")
	}

	entrance := &InstitutionEntrance{
		InstitutionID:       instID,
		InstitutionName:     req.InstitutionName,
		InstitutionLocation: req.InstitutionLocation,
		InstitutionLink:     req.InstitutionLink,
		InstitutionLogo:     req.InstitutionLogo,
		Title:               req.Title,
		Description:         req.Description,
		Program:             req.Program,
		Date:                date,
		StartTime:           req.StartTime,
		EndTime:             req.EndTime,
		Duration:            req.Duration,
		TotalMarks:          req.TotalMarks,
		PassingMarks:        req.PassingMarks,
		TotalSeats:          req.TotalSeats,
		Instructions:        req.Instructions,
		HeroBanner:          req.HeroBanner,
		Status:              "draft",
		ApplicationFee:      req.ApplicationFee,
		OverviewDetails:     req.OverviewDetails,
		ExamDateSchedules:   req.ExamDateSchedules,
		EligibilityList:     req.EligibilityList,
		ApplicationSteps:    req.ApplicationSteps,
		ExamPattern:         req.ExamPattern,
		SubjectMarks:        req.SubjectMarks,
		ModelSets:           req.ModelSets,
		UpcomingDates:       req.UpcomingDates,
		ContactPersons:      req.ContactPersons,
		Faqs:                req.Faqs,
		Email:               req.Email,
		ContactNumber:       req.ContactNumber,
		SocialLinks:         req.SocialLinks,
		ApplicationLink:     req.ApplicationLink,
		NoticeFile:          req.NoticeFile,
		EmbeddedMap:         req.EmbeddedMap,
		RequiredDocuments:   req.RequiredDocuments,
		ExaminationSchedule: req.ExaminationSchedule,
		ProgramsOffered:     req.ProgramsOffered,
	}

	if req.Status != "" {
		entrance.Status = req.Status
	}

	if req.Questions != nil {
		data, _ := json.Marshal(req.Questions)
		str := string(data)
		entrance.Questions = &str
	}

	if err := s.repo.CreateEntrance(entrance); err != nil {
		return nil, err
	}

	s.systemSvc.CreatePublicNotification(
		"New Entrance: "+entrance.Title,
		entrance.Description,
		"entrance",
		fmt.Sprintf("/entrance/%d", entrance.ID),
		"fa-pencil-alt",
		"text-red-600",
		"bg-red-100",
	)

	return entrance, nil
}

func (s *Service) UpdateEntrance(instID, id uint, req UpdateEntranceRequest) (*InstitutionEntrance, error) {
	entrance, err := s.repo.FindEntranceByIDAndInstitution(id, instID)
	if err != nil {
		return nil, errors.New("entrance not found")
	}

	if req.Title != "" {
		entrance.Title = req.Title
	}
	if req.Description != "" {
		entrance.Description = req.Description
	}
	if req.Program != "" {
		entrance.Program = req.Program
	}
	if req.Date != "" {
		if t, err := time.Parse("2006-01-02", req.Date); err == nil {
			entrance.Date = t
		}
	}
	if req.StartTime != "" {
		entrance.StartTime = req.StartTime
	}
	if req.EndTime != "" {
		entrance.EndTime = req.EndTime
	}
	if req.Duration > 0 {
		entrance.Duration = req.Duration
	}
	if req.TotalMarks > 0 {
		entrance.TotalMarks = req.TotalMarks
	}
	if req.PassingMarks > 0 {
		entrance.PassingMarks = req.PassingMarks
	}
	if req.TotalSeats > 0 {
		entrance.TotalSeats = req.TotalSeats
	}
	if req.Instructions != "" {
		entrance.Instructions = req.Instructions
	}
	if req.HeroBanner != "" {
		entrance.HeroBanner = req.HeroBanner
	}
	if req.Questions != nil {
		data, _ := json.Marshal(req.Questions)
		str := string(data)
		entrance.Questions = &str
	}
	if req.Status != "" {
		entrance.Status = req.Status
	}
	if req.ApplicationFee != "" {
		entrance.ApplicationFee = req.ApplicationFee
	}
	if len(req.OverviewDetails) > 0 {
		entrance.OverviewDetails = req.OverviewDetails
	}
	if len(req.ExamDateSchedules) > 0 {
		entrance.ExamDateSchedules = req.ExamDateSchedules
	}
	if len(req.EligibilityList) > 0 {
		entrance.EligibilityList = req.EligibilityList
	}
	if len(req.ApplicationSteps) > 0 {
		entrance.ApplicationSteps = req.ApplicationSteps
	}
	if len(req.ExamPattern) > 0 {
		entrance.ExamPattern = req.ExamPattern
	}
	if len(req.SubjectMarks) > 0 {
		entrance.SubjectMarks = req.SubjectMarks
	}
	if len(req.ModelSets) > 0 {
		entrance.ModelSets = req.ModelSets
	}
	if len(req.UpcomingDates) > 0 {
		entrance.UpcomingDates = req.UpcomingDates
	}
	if len(req.ContactPersons) > 0 {
		entrance.ContactPersons = req.ContactPersons
	}
	if len(req.Faqs) > 0 {
		entrance.Faqs = req.Faqs
	}
	if req.Email != "" {
		entrance.Email = req.Email
	}
	if req.ContactNumber != "" {
		entrance.ContactNumber = req.ContactNumber
	}
	if len(req.SocialLinks) > 0 {
		entrance.SocialLinks = req.SocialLinks
	}
	if req.ApplicationLink != "" {
		entrance.ApplicationLink = req.ApplicationLink
	}
	if req.NoticeFile != "" {
		entrance.NoticeFile = req.NoticeFile
	}
	if req.EmbeddedMap != "" {
		entrance.EmbeddedMap = req.EmbeddedMap
	}
	if len(req.RequiredDocuments) > 0 {
		entrance.RequiredDocuments = req.RequiredDocuments
	}
	if len(req.ExaminationSchedule) > 0 {
		entrance.ExaminationSchedule = req.ExaminationSchedule
	}
	if len(req.ProgramsOffered) > 0 {
		entrance.ProgramsOffered = req.ProgramsOffered
	}
	if req.InstitutionName != "" {
		entrance.InstitutionName = req.InstitutionName
	}
	if req.InstitutionLocation != "" {
		entrance.InstitutionLocation = req.InstitutionLocation
	}
	if req.InstitutionLink != "" {
		entrance.InstitutionLink = req.InstitutionLink
	}
	if req.InstitutionAffiliation != "" {
		entrance.InstitutionAffiliation = req.InstitutionAffiliation
	}
	if req.InstitutionLogo != "" {
		entrance.InstitutionLogo = req.InstitutionLogo
	}

	if err := s.repo.SaveEntrance(entrance); err != nil {
		return nil, err
	}

	return entrance, nil
}

func (s *Service) DeleteEntrance(instID, id uint) error {
	return s.repo.DeleteEntrance(id, instID)
}

func (s *Service) DeleteEntranceByID(id uint) error {
	return s.repo.DeleteEntranceByID(id)
}

func (s *Service) GetEntranceApplicants(instID, entranceID uint) ([]InstitutionEntranceApplicant, error) {
	_, err := s.repo.FindEntranceByIDAndInstitution(entranceID, instID)
	if err != nil {
		return nil, errors.New("entrance not found")
	}

	return s.repo.FindEntranceApplicants(entranceID)
}

func (s *Service) GetEvents(instID uint, page, limit int) ([]InstitutionEvent, int64, error) {
	return s.repo.FindEventsByInstitution(instID, page, limit)
}

func (s *Service) GetEventByID(instID, id uint) (*InstitutionEvent, error) {
	return s.repo.FindEventByIDAndInstitution(id, instID)
}

func parseEventTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", s)
	}
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
	}
	if err != nil {
		return nil
	}
	return &t
}

func (s *Service) CreateEvent(instID uint, req CreateEventRequest) (*InstitutionEvent, error) {
	var tagsJSON *string
	if req.Tags != nil {
		b, _ := json.Marshal(req.Tags)
		t := string(b)
		tagsJSON = &t
	}

	event := &InstitutionEvent{
		InstitutionID:      instID,
		Name:               req.Name,
		ShortDesc:          req.ShortDesc,
		Description:        req.Description,
		ImageURL:           req.ImageURL,
		EventType:          req.EventType,
		Category:           req.Category,
		MaxParticipants:    req.MaxParticipants,
		OnlineLink:         req.OnlineLink,
		OrganizedBy:        req.OrganizedBy,
		ContactPerson:      req.ContactPerson,
		ContactEmail:       req.ContactEmail,
		Location:           req.Location,
		Tags:               tagsJSON,
		EnableRegistration: req.EnableRegistration,
		Status:             "draft",
	}

	event.StartDate = parseEventTime(req.StartDate)
	event.EndDate = parseEventTime(req.EndDate)

	if req.Status == "upcoming" || req.Status == "published" {
		event.Status = req.Status
	}

	if err := s.repo.CreateEvent(event); err != nil {
		return nil, err
	}

	s.systemSvc.CreatePublicNotification(
		"New Event: "+event.Name,
		event.ShortDesc,
		"event",
		fmt.Sprintf("/events/%d", event.ID),
		"fa-calendar",
		"text-purple-600",
		"bg-purple-100",
	)

	return event, nil
}

func (s *Service) UpdateEvent(instID, id uint, req UpdateEventRequest) (*InstitutionEvent, error) {
	event, err := s.repo.FindEventByIDAndInstitution(id, instID)
	if err != nil {
		return nil, errors.New("event not found")
	}

	if req.Name != "" {
		event.Name = req.Name
	}
	if req.ShortDesc != "" {
		event.ShortDesc = req.ShortDesc
	}
	if req.Description != "" {
		event.Description = req.Description
	}
	if req.ImageURL != "" {
		event.ImageURL = req.ImageURL
	}
	if req.EventType != "" {
		event.EventType = req.EventType
	}
	if req.Category != "" {
		event.Category = req.Category
	}
	if req.OnlineLink != "" {
		event.OnlineLink = req.OnlineLink
	}
	if req.OrganizedBy != "" {
		event.OrganizedBy = req.OrganizedBy
	}
	if req.ContactPerson != "" {
		event.ContactPerson = req.ContactPerson
	}
	if req.ContactEmail != "" {
		event.ContactEmail = req.ContactEmail
	}
	if req.Location != "" {
		event.Location = req.Location
	}
	if req.StartDate != "" {
		event.StartDate = parseEventTime(req.StartDate)
	}
	if req.EndDate != "" {
		event.EndDate = parseEventTime(req.EndDate)
	}
	if req.MaxParticipants > 0 {
		event.MaxParticipants = req.MaxParticipants
	}
	if req.Tags != nil {
		b, _ := json.Marshal(req.Tags)
		t := string(b)
		event.Tags = &t
	}
	event.EnableRegistration = req.EnableRegistration
	if req.Status != "" {
		event.Status = req.Status
	}

	if err := s.repo.SaveEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *Service) ListPublicEvents(page, limit int) ([]EventResponse, int64, error) {
	events, total, err := s.repo.FindAllPublishedEvents(page, limit)
	if err != nil {
		return nil, 0, err
	}
	var resp []EventResponse
	for _, e := range events {
		resp = append(resp, toEventResponse(e))
	}
	return resp, total, nil
}

func (s *Service) GetPublicEventByID(id uint) (*InstitutionEvent, error) {
	event, err := s.repo.FindPublishedEventByID(id)
	if err != nil {
		return nil, err
	}
	if event.Status != "upcoming" && event.Status != "published" {
		return nil, errors.New("event not found")
	}
	return event, nil
}

func (s *Service) GetPublicEventBySlug(slug string) (*InstitutionEvent, error) {
	return s.repo.FindEventBySlug(slug)
}

func (s *Service) DeleteEvent(instID, id uint) error {
	return s.repo.DeleteEvent(id, instID)
}

func (s *Service) GetNews(instID uint, page, limit int) ([]InstitutionNews, int64, error) {
	return s.repo.FindNewsByInstitution(instID, page, limit)
}

func (s *Service) GetNewsByID(instID, id uint) (*InstitutionNews, error) {
	return s.repo.FindNewsByIDAndInstitution(id, instID)
}

func (s *Service) ListPublicNews(page, limit int) ([]NewsResponse, int64, error) {
	news, total, err := s.repo.FindAllPublishedNews(page, limit)
	if err != nil {
		return nil, 0, err
	}
	var resp []NewsResponse
	for _, n := range news {
		resp = append(resp, toNewsResponse(n))
	}
	return resp, total, nil
}

func (s *Service) GetPublicNewsByID(id uint) (*InstitutionNews, error) {
	return s.repo.FindPublishedNewsByID(id)
}

func (s *Service) GetPublicNewsBySlug(slug string) (*InstitutionNews, error) {
	return s.repo.FindNewsBySlug(slug)
}

func (s *Service) CreateNews(instID uint, req CreateNewsRequest) (*InstitutionNews, error) {
	var tagsJSON *string
	if req.Tags != nil {
		b, _ := json.Marshal(req.Tags)
		t := string(b)
		tagsJSON = &t
	}

	now := time.Now()

	news := &InstitutionNews{
		InstitutionID: instID,
		Title:         req.Title,
		ShortDesc:     req.ShortDesc,
		Content:       req.Content,
		ImageURL:      req.ImageURL,
		NewsType:      req.NewsType,
		PublishedBy:   req.PublishedBy,
		Tags:          tagsJSON,
		AllowComments: req.AllowComments,
		Status:        "draft",
	}

	if req.PublishDate != "" {
		news.PublishDate = &req.PublishDate
	}

	if req.Status == "published" {
		news.Status = "published"
		news.PublishedAt = &now
	}

	if req.PublishedBy == "" {
		news.PublishedBy = "Admin"
	}

	if err := s.repo.CreateNews(news); err != nil {
		return nil, err
	}

	s.systemSvc.CreatePublicNotification(
		"New News: "+news.Title,
		news.ShortDesc,
		"news",
		fmt.Sprintf("/news/%d", news.ID),
		"fa-newspaper",
		"text-blue-600",
		"bg-blue-100",
	)

	return news, nil
}

func (s *Service) UpdateNews(instID, id uint, req UpdateNewsRequest) (*InstitutionNews, error) {
	news, err := s.repo.FindNewsByIDAndInstitution(id, instID)
	if err != nil {
		return nil, errors.New("news not found")
	}

	if req.Title != "" {
		news.Title = req.Title
	}
	if req.Content != "" {
		news.Content = req.Content
	}
	if req.ShortDesc != "" {
		news.ShortDesc = req.ShortDesc
	}
	if req.ImageURL != "" {
		news.ImageURL = req.ImageURL
	}
	if req.NewsType != "" {
		news.NewsType = req.NewsType
	}
	if req.PublishedBy != "" {
		news.PublishedBy = req.PublishedBy
	}
	if req.PublishDate != "" {
		news.PublishDate = &req.PublishDate
	}
	if req.Tags != nil {
		b, _ := json.Marshal(req.Tags)
		t := string(b)
		news.Tags = &t
	}
	news.AllowComments = req.AllowComments
	if req.Status != "" {
		news.Status = req.Status
		if req.Status == "published" && news.PublishedAt == nil {
			now := time.Now()
			news.PublishedAt = &now
		}
	}

	if err := s.repo.SaveNews(news); err != nil {
		return nil, err
	}

	return news, nil
}

func (s *Service) DeleteNews(instID, id uint) error {
	return s.repo.DeleteNews(id, instID)
}

func (s *Service) GetBlogs(instID uint, page, limit int) ([]InstitutionBlog, int64, error) {
	return s.repo.FindBlogsByInstitution(instID, page, limit)
}

func (s *Service) GetBlogByID(instID, id uint) (*InstitutionBlog, error) {
	return s.repo.FindBlogByIDAndInstitution(id, instID)
}

func (s *Service) CreateBlog(instID uint, req CreateBlogRequest) (*InstitutionBlog, error) {
	status := req.Status
	if status == "" {
		status = "draft"
	}
	var publishedAt *time.Time
	if status == "published" {
		now := time.Now()
		publishedAt = &now
	}

	blog := &InstitutionBlog{
		InstitutionID: instID,
		Title:         req.Title,
		Content:       req.Content,
		Excerpt:       req.Excerpt,
		Image:         req.Image,
		Category:      req.Category,
		BlogCategory:  req.BlogCategory,
		ReadTime:      req.ReadTime,
		Tags:          req.Tags,
		Status:        status,
		PublishedAt:   publishedAt,
	}

	if err := s.repo.CreateBlog(blog); err != nil {
		return nil, err
	}

	if status == "published" {
		s.systemSvc.CreatePublicNotification(
			"New Blog: "+blog.Title,
			blog.Excerpt,
			"blog",
			fmt.Sprintf("/blog/%d", blog.ID),
			"fa-blog",
			"text-green-600",
			"bg-green-100",
		)
	}

	return blog, nil
}

func (s *Service) UpdateBlog(instID, id uint, req UpdateBlogRequest) (*InstitutionBlog, error) {
	blog, err := s.repo.FindBlogByIDAndInstitution(id, instID)
	if err != nil {
		return nil, errors.New("blog not found")
	}

	if req.Title != "" {
		blog.Title = req.Title
	}
	if req.Content != "" {
		blog.Content = req.Content
	}
	if req.Excerpt != "" {
		blog.Excerpt = req.Excerpt
	}
	if req.Image != "" {
		blog.Image = req.Image
	}
	if req.Category != "" {
		blog.Category = req.Category
	}
	if req.BlogCategory != "" {
		blog.BlogCategory = req.BlogCategory
	}
	if req.ReadTime != "" {
		blog.ReadTime = req.ReadTime
	}
	if req.Tags != "" {
		blog.Tags = req.Tags
	}
	if req.Status != "" {
		blog.Status = req.Status
		if req.Status == "published" && blog.PublishedAt == nil {
			now := time.Now()
			blog.PublishedAt = &now
		}
	}

	if err := s.repo.SaveBlog(blog); err != nil {
		return nil, err
	}

	return blog, nil
}

func (s *Service) DeleteBlog(instID, id uint) error {
	return s.repo.DeleteBlog(id, instID)
}

func (s *Service) ListPublicBlogs(page, limit int) ([]InstitutionBlog, int64, error) {
	return s.repo.FindPublishedBlogs(page, limit)
}

func (s *Service) GetPublicBlogBySlug(slug string) (*InstitutionBlog, error) {
	return s.repo.FindBlogBySlug(slug)
}

func (s *Service) GetQMS(instID uint, page, limit int) ([]InstitutionQMS, int64, error) {
	return s.repo.FindQMSByInstitution(instID, page, limit)
}

func (s *Service) GetQMSByID(instID, id uint) (*InstitutionQMS, error) {
	return s.repo.FindQMSByIDAndInstitution(id, instID)
}

func (s *Service) CreateQMS(instID uint, req CreateQMSRequest) (*InstitutionQMS, error) {
	qms := &InstitutionQMS{
		InstitutionID: instID,
		Title:         req.Title,
		Description:   req.Description,
		Category:      req.Category,
		Score:         req.Score,
		Status:        "pending",
	}

	if err := s.repo.CreateQMS(qms); err != nil {
		return nil, err
	}

	return qms, nil
}

func (s *Service) UpdateQMS(instID, id uint, req UpdateQMSRequest) (*InstitutionQMS, error) {
	qms, err := s.repo.FindQMSByIDAndInstitution(id, instID)
	if err != nil {
		return nil, errors.New("qms record not found")
	}

	if req.Title != "" {
		qms.Title = req.Title
	}
	if req.Description != "" {
		qms.Description = req.Description
	}
	if req.Category != "" {
		qms.Category = req.Category
	}
	if req.Status != "" {
		qms.Status = req.Status
	}
	if req.Score > 0 {
		qms.Score = req.Score
	}

	if err := s.repo.SaveQMS(qms); err != nil {
		return nil, err
	}

	return qms, nil
}

func (s *Service) DeleteQMS(instID, id uint) error {
	return s.repo.DeleteQMS(id, instID)
}

func (s *Service) GetMessages(instID uint, page, limit int) ([]InstitutionMessage, int64, error) {
	return s.repo.FindMessagesByInstitution(instID, page, limit)
}

func (s *Service) GetMessageByID(instID, id uint) (*InstitutionMessage, error) {
	message, err := s.repo.FindMessageByIDAndInstitution(id, instID)
	if err != nil {
		return nil, err
	}

	if !message.Read {
		message.Read = true
		if err := s.repo.SaveMessage(message); err != nil {
			return nil, err
		}
	}

	return message, nil
}

func (s *Service) CreateMessage(instID uint, req CreateMessageRequest) (*InstitutionMessage, error) {
	message := &InstitutionMessage{
		InstitutionID: instID,
		UserID:        req.UserID,
		Subject:       req.Subject,
		Content:       req.Content,
		Direction:     "outbound",
	}

	if err := s.repo.CreateMessage(message); err != nil {
		return nil, err
	}

	if err := s.repo.CreateUserMessage(instID, req.UserID, req.Subject, req.Content, "received"); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *Service) CreateInquiry(instID uint, userID uint, req CreateInquiryRequest) (*InstitutionMessage, error) {
	message := &InstitutionMessage{
		InstitutionID: instID,
		UserID:        userID,
		Subject:       req.Subject,
		Content:       req.Content,
		Direction:     "inbound",
	}
	if err := s.repo.CreateMessage(message); err != nil {
		return nil, err
	}

	if err := s.repo.CreateUserMessage(userID, instID, req.Subject, req.Content, "sent"); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *Service) GetMessageStudents(instID uint) ([]StudentContact, error) {
	messages, err := s.repo.FindAllMessagesByInstitution(instID)
	if err != nil {
		return nil, err
	}

	contactMap := map[uint]*StudentContact{}
	for _, msg := range messages {
		if _, exists := contactMap[msg.UserID]; !exists {
			user, err := s.repo.FindUserByID(msg.UserID)
			if err != nil {
				user = &User{}
			}
			contactMap[msg.UserID] = &StudentContact{
				UserID:      msg.UserID,
				Name:        user.FirstName + " " + user.LastName,
				LastMessage: msg.Content,
			}
		}
		if !msg.Read && msg.Direction == "inbound" {
			contactMap[msg.UserID].Unread++
		}
	}

	contacts := make([]StudentContact, 0, len(contactMap))
	for _, c := range contactMap {
		contacts = append(contacts, *c)
	}

	return contacts, nil
}

func (s *Service) GetSettings(instID uint) (*SettingsResponse, error) {
	settings, err := s.repo.FindOrCreateSettings(instID)
	if err != nil {
		return nil, err
	}

	return &SettingsResponse{
		ID:            settings.ID,
		CreatedAt:     settings.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     settings.UpdatedAt.Format(time.RFC3339),
		InstitutionID: settings.InstitutionID,
		EmailNotifs:   settings.EmailNotifs,
		Timezone:      settings.Timezone,
		Language:      settings.Language,
		PublicProfile: settings.PublicProfile,
	}, nil
}

func (s *Service) UpdateSettings(instID uint, req UpdateSettingsRequest) (*SettingsResponse, error) {
	settings, err := s.repo.FindOrCreateSettings(instID)
	if err != nil {
		return nil, err
	}

	settings.EmailNotifs = req.EmailNotifs
	if req.Timezone != "" {
		settings.Timezone = req.Timezone
	}
	if req.Language != "" {
		settings.Language = req.Language
	}
	settings.PublicProfile = req.PublicProfile

	if err := s.repo.SaveSettings(settings); err != nil {
		return nil, err
	}

	return &SettingsResponse{
		ID:            settings.ID,
		CreatedAt:     settings.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     settings.UpdatedAt.Format(time.RFC3339),
		InstitutionID: settings.InstitutionID,
		EmailNotifs:   settings.EmailNotifs,
		Timezone:      settings.Timezone,
		Language:      settings.Language,
		PublicProfile: settings.PublicProfile,
	}, nil
}

func (s *Service) GetScholarships(instID uint) ([]Scholarship, error) {
	return s.repo.FindScholarshipsByInstitution(instID)
}

func (s *Service) CreateScholarship(instID uint, req CreateScholarshipRequest) (*Scholarship, error) {
	var deadline time.Time
	if req.Deadline != "" {
		var err error
		deadline, err = time.Parse("2006-01-02", req.Deadline)
		if err != nil {
			return nil, errors.New("invalid deadline format (expected YYYY-MM-DD)")
		}
	}

	fieldOfStudy, _ := json.Marshal(req.FieldOfStudy)

	status := req.Status
	if status == "" {
		status = "draft"
	}

	scholarship := &Scholarship{
		InstitutionID:   instID,
		Title:           req.Title,
		ShortDesc:       req.ShortDesc,
		Provider:        req.Provider,
		Location:        req.Location,
		Value:           req.Value,
		Deadline:        deadline,
		DegreeLevel:     req.DegreeLevel,
		FundingType:     req.FundingType,
		ScholarshipType: req.ScholarshipType,
		Description:     req.Description,
		ImageURL:        req.ImageURL,
		FieldOfStudy:    fieldOfStudy,
		Status:          status,
	}

	if err := s.repo.CreateScholarship(scholarship); err != nil {
		return nil, err
	}

	return scholarship, nil
}

func (s *Service) UpdateScholarship(instID, id uint, req UpdateScholarshipRequest) (*Scholarship, error) {
	scholarship, err := s.repo.FindScholarshipByIDAndInstitution(id, instID)
	if err != nil {
		return nil, errors.New("scholarship not found")
	}

	if req.Title != "" {
		scholarship.Title = req.Title
	}
	if req.ShortDesc != "" {
		scholarship.ShortDesc = req.ShortDesc
	}
	if req.Provider != "" {
		scholarship.Provider = req.Provider
	}
	if req.Location != "" {
		scholarship.Location = req.Location
	}
	if req.Value != "" {
		scholarship.Value = req.Value
	}
	if req.Deadline != "" {
		if deadline, err := time.Parse("2006-01-02", req.Deadline); err == nil {
			scholarship.Deadline = deadline
		}
	}
	if req.DegreeLevel != "" {
		scholarship.DegreeLevel = req.DegreeLevel
	}
	if req.FundingType != "" {
		scholarship.FundingType = req.FundingType
	}
	if req.ScholarshipType != "" {
		scholarship.ScholarshipType = req.ScholarshipType
	}
	if req.Description != "" {
		scholarship.Description = req.Description
	}
	if req.ImageURL != "" {
		scholarship.ImageURL = req.ImageURL
	}
	if len(req.FieldOfStudy) > 0 {
		if data, err := json.Marshal(req.FieldOfStudy); err == nil {
			scholarship.FieldOfStudy = data
		}
	}
	if req.Status != "" {
		scholarship.Status = req.Status
	}

	if err := s.repo.SaveScholarship(scholarship); err != nil {
		return nil, err
	}

	return scholarship, nil
}

func (s *Service) DeleteScholarship(instID, id uint) error {
	_, err := s.repo.FindScholarshipByIDAndInstitution(id, instID)
	if err != nil {
		return errors.New("scholarship not found")
	}
	return s.repo.DeleteScholarship(id)
}

func (s *Service) ListPublicScholarships(page, limit int) ([]ScholarshipResponse, int64, error) {
	scholarships, total, err := s.repo.FindAllPublishedScholarships(page, limit)
	if err != nil {
		return nil, 0, err
	}
	var resp []ScholarshipResponse
	for _, sch := range scholarships {
		resp = append(resp, toScholarshipResponse(sch))
	}
	return resp, total, nil
}

func (s *Service) GetPublicScholarshipByID(id uint) (*Scholarship, error) {
	return s.repo.FindPublishedScholarshipByID(id)
}

func (s *Service) GetAdmissions(instID uint, status string) ([]Admission, error) {
	college, err := s.repo.FindCollegeByUniversityID(instID)
	if err != nil {
		return nil, errors.New("no college found for this institution")
	}

	return s.repo.FindAdmissionsByCollegeID(college.ID, status)
}

func (s *Service) UpdateAdmissionStatus(instID, id uint, req UpdateAdmissionStatusRequest) (*Admission, error) {
	college, err := s.repo.FindCollegeByUniversityID(instID)
	if err != nil {
		return nil, errors.New("no college found for this institution")
	}

	admission, err := s.repo.FindAdmissionByID(id)
	if err != nil {
		return nil, errors.New("admission not found")
	}

	if admission.CollegeID != college.ID {
		return nil, errors.New("you can only manage admissions for your own college")
	}

	now := time.Now()
	instUID := instID
	admission.Status = req.Status
	admission.Notes = req.Notes
	admission.ReviewedBy = &instUID
	admission.ReviewedAt = &now

	if err := s.repo.SaveAdmission(admission); err != nil {
		return nil, err
	}

	return admission, nil
}

// --- Admission Page Service ---

func extractAdmissionTitle(data *string) string {
	if data == nil || *data == "" {
		return ""
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(*data), &parsed); err != nil {
		return ""
	}
	if od, ok := parsed["overview_data"].(map[string]interface{}); ok {
		if heading, ok := od["overviewHeading"].(string); ok && heading != "" {
			return heading
		}
	}
	if programs, ok := parsed["programs_data"].([]interface{}); ok && len(programs) > 0 {
		if first, ok := programs[0].(map[string]interface{}); ok {
			if title, ok := first["title"].(string); ok && title != "" {
				return title
			}
		}
	}
	return ""
}

func extractFirstProgram(data *string) string {
	if data == nil || *data == "" {
		return ""
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(*data), &parsed); err != nil {
		return ""
	}
	if programs, ok := parsed["programs_data"].([]interface{}); ok && len(programs) > 0 {
		if first, ok := programs[0].(map[string]interface{}); ok {
			if title, ok := first["title"].(string); ok {
				return title
			}
		}
	}
	return ""
}

func extractLevel(data *string) string {
	if data == nil || *data == "" {
		return ""
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(*data), &parsed); err != nil {
		return ""
	}
	if od, ok := parsed["overview_data"].(map[string]interface{}); ok {
		if level, ok := od["level"].(string); ok {
			return level
		}
	}
	return ""
}

func (s *Service) CreateAdmissionPage(instID uint, req CreateAdmissionPageRequest) (*AdmissionPageResponse, error) {
	dataStr := string(req.Data)
	page := &AdmissionPage{
		InstitutionID:       instID,
		InstitutionName:     req.InstitutionName,
		InstitutionLocation: req.InstitutionLocation,
		InstitutionLink:     req.InstitutionLink,
		Title:               extractAdmissionTitle(&dataStr),
		Status:              "draft",
		Data:                &dataStr,
	}

	if req.Status == "published" {
		now := time.Now()
		page.Status = "published"
		page.PublishedAt = &now
	}

	if err := s.repo.CreateAdmissionPage(page); err != nil {
		return nil, err
	}

	if page.Status == "published" {
		s.systemSvc.CreatePublicNotification(
			"Admission Open: "+page.Title,
			"New admission opening available",
			"admission",
			fmt.Sprintf("/admissions/%d", page.ID),
			"fa-door-open",
			"text-orange-600",
			"bg-orange-100",
		)
	}

	return toAdmissionPageResponse(page), nil
}

func (s *Service) GetAdmissionPages(instID uint, status string, page, limit int) ([]AdmissionPageListItem, int64, error) {
	pages, total, err := s.repo.FindAdmissionPagesByInstitution(instID, status, page, limit)
	if err != nil {
		return nil, 0, err
	}

	items := make([]AdmissionPageListItem, len(pages))
	for i, p := range pages {
		items[i] = toAdmissionPageListItem(p)
	}
	return items, total, nil
}

func (s *Service) GetAdmissionPage(instID, id uint) (*AdmissionPageResponse, error) {
	page, err := s.repo.FindAdmissionPageByID(id, instID)
	if err != nil {
		return nil, err
	}
	return toAdmissionPageResponse(page), nil
}

func (s *Service) UpdateAdmissionPage(instID, id uint, req UpdateAdmissionPageRequest) (*AdmissionPageResponse, error) {
	page, err := s.repo.FindAdmissionPageByID(id, instID)
	if err != nil {
		return nil, err
	}

	wasPublished := page.Status == "published"

	if req.Data != nil {
		dataStr := string(req.Data)
		page.Data = &dataStr
		page.Title = extractAdmissionTitle(&dataStr)
	}

	if req.Status != nil {
		page.Status = *req.Status
		if *req.Status == "published" {
			now := time.Now()
			page.PublishedAt = &now
		} else {
			page.PublishedAt = nil
		}
	}
	if req.InstitutionName != "" {
		page.InstitutionName = req.InstitutionName
	}
	if req.InstitutionLocation != "" {
		page.InstitutionLocation = req.InstitutionLocation
	}
	if req.InstitutionLink != "" {
		page.InstitutionLink = req.InstitutionLink
	}

	if err := s.repo.SaveAdmissionPage(page); err != nil {
		return nil, err
	}

	if page.Status == "published" && !wasPublished {
		s.systemSvc.CreatePublicNotification(
			"Admission Open: "+page.Title,
			"New admission opening available",
			"admission",
			fmt.Sprintf("/admissions/%d", page.ID),
			"fa-door-open",
			"text-orange-600",
			"bg-orange-100",
		)
	}

	return toAdmissionPageResponse(page), nil
}

func (s *Service) DeleteAdmissionPage(instID, id uint) error {
	return s.repo.DeleteAdmissionPage(id, instID)
}

func (s *Service) DeleteAdmissionPageByID(id uint) error {
	return s.repo.DeleteAdmissionPageByID(id)
}

// --- Superadmin Service Methods ---

func (s *Service) GetAllPrograms(page, limit int) ([]InstitutionProgram, int64, error) {
	return s.repo.FindAllPrograms(page, limit)
}

func (s *Service) GetAllEntrances(status string, page, limit int) ([]InstitutionEntrance, int64, error) {
	return s.repo.FindAllEntrances(status, page, limit)
}

func (s *Service) GetAllAdmissionPages(status string, page, limit int) ([]AdmissionPage, int64, error) {
	return s.repo.FindAllAdmissionPages(status, page, limit)
}

func (s *Service) GetEntranceApplicantsByID(entranceID uint) ([]InstitutionEntranceApplicant, error) {
	_, err := s.repo.FindEntranceByID(entranceID)
	if err != nil {
		return nil, errors.New("entrance not found")
	}
	return s.repo.FindEntranceApplicants(entranceID)
}

func (s *Service) GetPublishedAdmissionPages(page, limit int) ([]AdmissionPageListItem, int64, error) {
	pages, total, err := s.repo.FindPublishedAdmissionPages(page, limit)
	if err != nil {
		return nil, 0, err
	}

	items := make([]AdmissionPageListItem, len(pages))
	for i, p := range pages {
		items[i] = toAdmissionPageListItem(p)
	}
	return items, total, nil
}

func mapAdmissionLevel(level string) string {
	mapping := map[string]string{
		"high-school": "+2",
		"a-level":     "A-Level",
		"diploma":     "Diploma/CTEVT",
		"ctevt":       "Diploma/CTEVT",
		"bachelor":    "Bachelor",
		"master":      "Master",
	}
	if mapped, ok := mapping[level]; ok {
		return mapped
	}
	return level
}

func (s *Service) GetPublishedAdmissionInstitutions(page, limit int, level string) (*PublishedAdmissionInstitutionListResponse, error) {
	results, total, err := s.repo.FindPublishedAdmissionInstitutions(page, limit, mapAdmissionLevel(level))
	if err != nil {
		return nil, err
	}

	colleges := make([]PublishedAdmissionInstitutionItem, len(results))
	for i, row := range results {
		id, _ := row["id"].(int64)
		name, _ := row["name"].(string)
		imageURL, _ := row["image_url"].(string)
		location, _ := row["location"].(string)
		collegeType, _ := row["type"].(string)
		rating, _ := row["rating"].(float64)
		website, _ := row["website"].(string)
		affiliation, _ := row["affiliation"].(string)
		verified, _ := row["verified"].(bool)
		heroBanner, _ := row["hero_banner"].(string)

		featuredPrograms := []FeaturedProgramItem{}
		if fp, ok := row["featured_programs"]; ok && fp != nil {
			switch v := fp.(type) {
			case []byte:
				json.Unmarshal(v, &featuredPrograms)
			case string:
				if s := strings.TrimSpace(v); s != "" {
					json.Unmarshal([]byte(v), &featuredPrograms)
				}
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						title, _ := m["title"].(string)
						status, _ := m["admissionStatus"].(string)
						featuredPrograms = append(featuredPrograms, FeaturedProgramItem{
							Title:           strings.TrimSpace(title),
							AdmissionStatus: strings.TrimSpace(status),
						})
					}
				}
			}
		}
		if len(featuredPrograms) == 0 {
			featuredPrograms = []FeaturedProgramItem{}
		}

		colleges[i] = PublishedAdmissionInstitutionItem{
			ID:               uint(id),
			Name:             name,
			ImageURL:         imageURL,
			Location:         location,
			Type:             collegeType,
			Rating:           rating,
			Website:          website,
			Affiliation:      affiliation,
			Verified:         verified,
			FeaturedPrograms: featuredPrograms,
			Programs:         0,
			HeroBanner:       heroBanner,
		}
	}

	totalPages := int64(1)
	if limit > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	return &PublishedAdmissionInstitutionListResponse{
		Colleges: colleges,
		Pagination: PaginationMeta{
			Total:      total,
			Page:       page,
			Limit:      limit,
			PageSize:   limit,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *Service) GetPublishedAdmissionInstitutionByID(id uint) (*PublishedAdmissionInstitutionDetailResponse, error) {
	rows, err := s.repo.FindPublishedAdmissionInstitutionByID(id)
	if err != nil {
		return nil, err
	}

	admissionPage, err := s.repo.FindPublishedAdmissionByInstitutionID(id)
	if err != nil {
		return nil, err
	}

	college := &PublishedAdmissionInstitutionItem{}
	if len(rows) > 0 {
		row := rows[0]
		college.ID = uint(getInt64(row, "id"))
		college.Name = getString(row, "name")
		college.ImageURL = getString(row, "image_url")
		college.Location = getString(row, "location")
		college.Type = getString(row, "type")
		college.Rating = getFloat64(row, "rating")
		college.Website = getString(row, "website")
		college.Affiliation = getString(row, "affiliation")
		college.Verified = getBool(row, "verified")
		college.ContactEmail = getString(row, "contact_email")
		college.ContactPhone = getString(row, "contact_phone")

		featuredPrograms := []FeaturedProgramItem{}
		if admissionPage != nil && admissionPage.Data != nil {
			var pageData struct {
				ProgramsData []struct {
					Title           string `json:"title"`
					AdmissionStatus string `json:"admissionStatus"`
				} `json:"programs_data"`
			}
			if err := json.Unmarshal([]byte(*admissionPage.Data), &pageData); err == nil {
				for _, p := range pageData.ProgramsData {
					if t := strings.TrimSpace(p.Title); t != "" {
						featuredPrograms = append(featuredPrograms, FeaturedProgramItem{
							Title:           t,
							AdmissionStatus: strings.TrimSpace(p.AdmissionStatus),
						})
					}
				}
			}
		}
		college.FeaturedPrograms = featuredPrograms
	}
	pCount, _ := s.repo.CountProgramsByInstitution(id)
	college.Programs = int(pCount)

	var data json.RawMessage
	if admissionPage.Data != nil {
		data = json.RawMessage(*admissionPage.Data)
	}

	var publishedAt *string
	if admissionPage.PublishedAt != nil {
		s := admissionPage.PublishedAt.Format(time.RFC3339)
		publishedAt = &s
	}

	return &PublishedAdmissionInstitutionDetailResponse{
		Institution: college,
		Data:        data,
		CreatedAt:   admissionPage.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   admissionPage.UpdatedAt.Format(time.RFC3339),
		PublishedAt: publishedAt,
	}, nil
}

func getInt64(m map[string]interface{}, key string) int64 {
	v, _ := m[key].(int64)
	return v
}

func getString(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func getFloat64(m map[string]interface{}, key string) float64 {
	v, _ := m[key].(float64)
	return v
}

func getBool(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func toAdmissionPageResponse(p *AdmissionPage) *AdmissionPageResponse {
	resp := &AdmissionPageResponse{
		ID:            p.ID,
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     p.UpdatedAt.Format(time.RFC3339),
		InstitutionID: p.InstitutionID,
		Title:         p.Title,
		Status:        p.Status,
	}
	if p.Data != nil {
		resp.Data = json.RawMessage(*p.Data)
	}
	if p.PublishedAt != nil {
		s := p.PublishedAt.Format(time.RFC3339)
		resp.PublishedAt = &s
	}
	return resp
}

func toAdmissionPageListItem(p AdmissionPage) AdmissionPageListItem {
	item := AdmissionPageListItem{
		ID:         p.ID,
		Title:      p.Title,
		Program:    extractFirstProgram(p.Data),
		Level:      extractLevel(p.Data),
		Status:     p.Status,
		LastEdited: p.UpdatedAt.Format("2006-01-02"),
	}
	if p.PublishedAt != nil {
		s := p.PublishedAt.Format("2006-01-02")
		item.PublishedAt = &s
	}
	return item
}

func (s *Service) GetScholarshipApplications(instID uint, status string) ([]ScholarshipApplication, error) {
	college, err := s.repo.FindCollegeByUniversityID(instID)
	if err != nil {
		return nil, errors.New("no college found for this institution")
	}

	scholarships, err := s.repo.FindScholarshipsByProvider(college.Name)
	if err != nil {
		return nil, err
	}

	scholarshipIDs := make([]uint, len(scholarships))
	for i, s := range scholarships {
		scholarshipIDs[i] = s.ID
	}

	if len(scholarshipIDs) == 0 {
		return []ScholarshipApplication{}, nil
	}

	return s.repo.FindScholarshipApplicationsByIDs(scholarshipIDs, status)
}

func (s *Service) UpdateScholarshipApplicationStatus(instID, id uint, req UpdateScholarshipApplicationStatusRequest) (*ScholarshipApplication, error) {
	college, err := s.repo.FindCollegeByUniversityID(instID)
	if err != nil {
		return nil, errors.New("no college found for this institution")
	}

	application, err := s.repo.FindScholarshipApplicationByID(id)
	if err != nil {
		return nil, errors.New("application not found")
	}

	if application.Scholarship.Provider != college.Name {
		return nil, errors.New("you can only manage applications for your own scholarships")
	}

	application.Status = req.Status

	if err := s.repo.SaveScholarshipApplication(application); err != nil {
		return nil, err
	}

	return application, nil
}

func (s *Service) ListPublicInstitutions(page, limit int, search, location, instType string) ([]PublicInstitutionResponse, int64, error) {
	users, total, err := s.repo.FindPublicInstitutions(page, limit, search, location, instType)
	if err != nil {
		return nil, 0, err
	}

	results := make([]PublicInstitutionResponse, len(users))
	for i, u := range users {
		logoURL := u.LogoURL
		bannerURL := u.BannerURL
		if strings.HasPrefix(logoURL, "data:") {
			logoURL = ""
		}
		if strings.HasPrefix(bannerURL, "data:") {
			bannerURL = ""
		}
		results[i] = PublicInstitutionResponse{
			ID:              u.ID,
			InstitutionName: u.InstitutionName,
			Verified:        u.Verified,
			Claimed:         u.Claimed,
			Affiliation:     u.Affiliation,
			LogoURL:         logoURL,
			BannerURL:       bannerURL,
			About:           u.About,
			District:        u.District,
			WebsiteURL:      u.WebsiteURL,
			Status:          u.Status,
			Featured:        u.Featured,
			CollegeID:       u.CollegeID,
			Type:            u.OrganizationType,
		}
	}
	return results, total, nil
}

func (s *Service) GetPublicInstitution(id uint) (*PublicInstitutionDetailResponse, error) {
	user, err := s.repo.FindPublicInstitutionByID(id)
	if err != nil {
		return nil, errors.New("institution not found or not public")
	}

	var pd ProfileData
	if user.ProfileData != nil && *user.ProfileData != "" {
		json.Unmarshal([]byte(*user.ProfileData), &pd)
	}

	logoURL := user.LogoURL
	bannerURL := user.BannerURL
	if strings.HasPrefix(logoURL, "data:") {
		logoURL = ""
	}
	if strings.HasPrefix(bannerURL, "data:") {
		bannerURL = ""
	}

	programs, _ := s.repo.FindAllProgramsByInstitution(id)
	programResponses := make([]ProgramResponse, 0, len(programs))
	for _, p := range programs {
		var data interface{}
		if p.Data != nil {
			json.Unmarshal([]byte(*p.Data), &data)
		}
		programResponses = append(programResponses, ProgramResponse{
			ID:                  p.ID,
			CreatedAt:           p.CreatedAt.Format(time.RFC3339),
			UpdatedAt:           p.UpdatedAt.Format(time.RFC3339),
			InstitutionID:       p.InstitutionID,
			InstitutionName:     p.InstitutionName,
			InstitutionLocation: p.InstitutionLocation,
			InstitutionLink:     p.InstitutionLink,
			Name:                p.Name,
			Description:         p.Description,
			Duration:            p.Duration,
			Fee:                 p.Fee,
			Eligibility:         p.Eligibility,
			Capacity:            p.Capacity,
			BannerURL:           p.BannerURL,
			Data:                data,
			Status:              p.Status,
		})
	}

	events, _, _ := s.repo.FindEventsByInstitution(id, 1, 100)
	eventResponses := make([]EventResponse, 0, len(events))
	for _, e := range events {
		if e.Status == "upcoming" || e.Status == "published" {
			eventResponses = append(eventResponses, toEventResponse(e))
		}
	}

	newsList, _, _ := s.repo.FindNewsByInstitution(id, 1, 100)
	newsResponses := make([]NewsResponse, 0, len(newsList))
	for _, n := range newsList {
		if n.Status == "published" {
			newsResponses = append(newsResponses, toNewsResponse(n))
		}
	}

	var scholarshipResponses []ScholarshipResponse
	college, err := s.repo.FindCollegeByUniversityID(id)
	if err == nil && college != nil {
		scholarships, _ := s.repo.FindScholarshipsByLocation("%" + college.Name + "%")
		scholarshipResponses = make([]ScholarshipResponse, 0, len(scholarships))
		for _, sch := range scholarships {
			if sch.Status != "published" {
				continue
			}
			scholarshipResponses = append(scholarshipResponses, toScholarshipResponse(sch))
		}
	}

	var admissionPageData json.RawMessage
	admissionPage, err := s.repo.FindPublishedAdmissionByInstitutionID(id)
	if err == nil && admissionPage != nil && admissionPage.Data != nil {
		admissionPageData = json.RawMessage(*admissionPage.Data)
	}

	return &PublicInstitutionDetailResponse{
		ID:                      user.ID,
		InstitutionName:         user.InstitutionName,
		Verified:                user.Verified,
		Claimed:                 user.Claimed,
		Featured:                user.Featured,
		LogoURL:                 logoURL,
		BannerURL:               bannerURL,
		About:                   user.About,
		Vision:                  user.Vision,
		Mission:                 user.Mission,
		District:                user.District,
		WebsiteURL:              user.WebsiteURL,
		Videos:                  pd.Videos,
		OverviewData:            pd.OverviewData,
		LeadershipData:          pd.LeadershipData,
		CoursesData:             pd.CoursesData,
		ProgramsData:            pd.ProgramsData,
		FacilitiesData:          pd.FacilitiesData,
		AlumniData:              pd.AlumniData,
		GalleryData:             pd.GalleryData,
		DownloadsData:           pd.DownloadsData,
		InstitutionPrograms:     programResponses,
		InstitutionEvents:       eventResponses,
		InstitutionNews:         newsResponses,
		InstitutionScholarships: scholarshipResponses,
		AdmissionPageData:       admissionPageData,
		ContactEmail:            user.ContactEmail,
		ContactPhone:            user.ContactPhone,
		MapURL:                  user.MapURL,
		FacebookURL:             user.FacebookURL,
		InstagramURL:            user.InstagramURL,
		TiktokURL:               user.TiktokURL,
		YoutubeURL:              user.YoutubeURL,
		LinkedinURL:             user.LinkedinURL,
		BrochureData:            pd.BrochureData,
		Type:                    user.OrganizationType,
	}, nil
}

func (s *Service) GetPublicInstitutionFilterCounts() (*PublicInstitutionFilterCountsResponse, error) {
	return s.repo.GetPublicFilterCounts()
}
