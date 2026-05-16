package institution

import (
	"encoding/json"
	"errors"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
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

	return &ProfileResponse{
		ID:                    user.ID,
		InstitutionName:       user.InstitutionName,
		Email:                 user.Email,
		RegistrationNumber:    user.RegistrationNumber,
		Role:                  user.Role,
		Location:              user.District,
		Website:               user.WebsiteURL,
		LogoURL:               user.LogoURL,
		BannerURL:             user.BannerURL,
		About:                 user.About,
		Vision:                user.Vision,
		Mission:               user.Mission,
		Videos:                pd.Videos,
		OverviewData:          pd.OverviewData,
		LeadershipData:        pd.LeadershipData,
		CoursesData:           pd.CoursesData,
		ProgramsData:          pd.ProgramsData,
		FacilitiesData:        pd.FacilitiesData,
		AlumniData:            pd.AlumniData,
		DownloadsData:         pd.DownloadsData,
		WhatsNewData:          pd.WhatsNewData,
		EligibilityData:       pd.EligibilityData,
		AdmissionProcessData:  pd.AdmissionProcessData,
		ScholarshipsData:      pd.ScholarshipsData,
		FaqsData:              pd.FaqsData,
		ContactPersonsData:    pd.ContactPersonsData,
		BrochureData:          pd.BrochureData,
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
		user.LogoURL = req.LogoURL
	}
	if req.BannerURL != "" {
		user.BannerURL = req.BannerURL
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
	if req.Videos != nil || req.OverviewData != nil || req.LeadershipData != nil ||
		req.CoursesData != nil || req.ProgramsData != nil || req.FacilitiesData != nil ||
		req.AlumniData != nil || req.DownloadsData != nil ||
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

func (s *Service) CreateProgram(instID uint, req CreateProgramRequest) (*InstitutionProgram, error) {
	program := &InstitutionProgram{
		InstitutionID: instID,
		Name:          req.Name,
		Description:   req.Description,
		Duration:      req.Duration,
		Fee:           req.Fee,
		Eligibility:   req.Eligibility,
		Capacity:      req.Capacity,
		Status:        "active",
	}

	if err := s.repo.CreateProgram(program); err != nil {
		return nil, err
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
	if req.Status != "" {
		program.Status = req.Status
	}

	if err := s.repo.SaveProgram(program); err != nil {
		return nil, err
	}

	return program, nil
}

func (s *Service) DeleteProgram(instID, id uint) error {
	return s.repo.DeleteProgram(id, instID)
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

func (s *Service) GetCounsellingBookings(instID uint) ([]InstitutionCounsellingBooking, error) {
	return s.repo.FindCounsellingBookingsByInstitution(instID)
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

func (s *Service) GetEntrances(instID uint, page, limit int) ([]InstitutionEntrance, int64, error) {
	return s.repo.FindEntrancesByInstitution(instID, page, limit)
}

func (s *Service) GetEntranceByID(instID, id uint) (*InstitutionEntrance, error) {
	return s.repo.FindEntranceByIDAndInstitution(id, instID)
}

func (s *Service) CreateEntrance(instID uint, req CreateEntranceRequest) (*InstitutionEntrance, error) {
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, errors.New("invalid date format")
	}

	entrance := &InstitutionEntrance{
		InstitutionID: instID,
		Title:         req.Title,
		Description:   req.Description,
		Date:          date,
		Duration:      req.Duration,
		TotalSeats:    req.TotalSeats,
		Status:        "upcoming",
	}

	if err := s.repo.CreateEntrance(entrance); err != nil {
		return nil, err
	}

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
	if req.Date != "" {
		if t, err := time.Parse("2006-01-02", req.Date); err == nil {
			entrance.Date = t
		}
	}
	if req.Duration > 0 {
		entrance.Duration = req.Duration
	}
	if req.TotalSeats > 0 {
		entrance.TotalSeats = req.TotalSeats
	}
	if req.Status != "" {
		entrance.Status = req.Status
	}

	if err := s.repo.SaveEntrance(entrance); err != nil {
		return nil, err
	}

	return entrance, nil
}

func (s *Service) DeleteEntrance(instID, id uint) error {
	return s.repo.DeleteEntrance(id, instID)
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

func (s *Service) CreateEvent(instID uint, req CreateEventRequest) (*InstitutionEvent, error) {
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, errors.New("invalid date format")
	}

	event := &InstitutionEvent{
		InstitutionID: instID,
		Title:         req.Title,
		Description:   req.Description,
		Date:          date,
		Location:      req.Location,
		Image:         req.Image,
		Status:        "upcoming",
	}

	if err := s.repo.CreateEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *Service) UpdateEvent(instID, id uint, req UpdateEventRequest) (*InstitutionEvent, error) {
	event, err := s.repo.FindEventByIDAndInstitution(id, instID)
	if err != nil {
		return nil, errors.New("event not found")
	}

	if req.Title != "" {
		event.Title = req.Title
	}
	if req.Description != "" {
		event.Description = req.Description
	}
	if req.Date != "" {
		if t, err := time.Parse("2006-01-02", req.Date); err == nil {
			event.Date = t
		}
	}
	if req.Location != "" {
		event.Location = req.Location
	}
	if req.Image != "" {
		event.Image = req.Image
	}
	if req.Status != "" {
		event.Status = req.Status
	}

	if err := s.repo.SaveEvent(event); err != nil {
		return nil, err
	}

	return event, nil
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

func (s *Service) CreateNews(instID uint, req CreateNewsRequest) (*InstitutionNews, error) {
	news := &InstitutionNews{
		InstitutionID: instID,
		Title:         req.Title,
		Content:       req.Content,
		Excerpt:       req.Excerpt,
		Image:         req.Image,
		Category:      req.Category,
		Published:     true,
	}

	if err := s.repo.CreateNews(news); err != nil {
		return nil, err
	}

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
	if req.Excerpt != "" {
		news.Excerpt = req.Excerpt
	}
	if req.Image != "" {
		news.Image = req.Image
	}
	if req.Category != "" {
		news.Category = req.Category
	}

	if err := s.repo.SaveNews(news); err != nil {
		return nil, err
	}

	return news, nil
}

func (s *Service) DeleteNews(instID, id uint) error {
	return s.repo.DeleteNews(id, instID)
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
	college, err := s.repo.FindCollegeByUniversityID(instID)
	if err != nil {
		return nil, errors.New("no college found for this institution")
	}

	return s.repo.FindScholarshipsByLocation("%" + college.Name + "%")
}

func (s *Service) CreateScholarship(instID uint, req CreateScholarshipRequest) (*Scholarship, error) {
	college, err := s.repo.FindCollegeByUniversityID(instID)
	if err != nil {
		return nil, errors.New("no college found for this institution")
	}

	var deadline time.Time
	if req.Deadline != "" {
		deadline, err = time.Parse("2006-01-02", req.Deadline)
		if err != nil {
			return nil, errors.New("invalid deadline format (expected YYYY-MM-DD)")
		}
	}

	fieldOfStudy, _ := json.Marshal(req.FieldOfStudy)

	scholarship := &Scholarship{
		Title:           req.Title,
		Provider:        college.Name,
		Location:        req.Location,
		Value:           req.Value,
		Deadline:        deadline,
		DegreeLevel:     req.DegreeLevel,
		FundingType:     req.FundingType,
		ScholarshipType: req.ScholarshipType,
		Description:     req.Description,
		ImageURL:        req.ImageURL,
		FieldOfStudy:    fieldOfStudy,
	}

	if err := s.repo.CreateScholarship(scholarship); err != nil {
		return nil, err
	}

	return scholarship, nil
}

func (s *Service) UpdateScholarship(instID, id uint, req UpdateScholarshipRequest) (*Scholarship, error) {
	college, err := s.repo.FindCollegeByUniversityID(instID)
	if err != nil {
		return nil, errors.New("no college found for this institution")
	}

	scholarship, err := s.repo.FindScholarshipByID(id)
	if err != nil {
		return nil, errors.New("scholarship not found")
	}

	if scholarship.Provider != college.Name {
		return nil, errors.New("you can only update your own scholarships")
	}

	if req.Title != "" {
		scholarship.Title = req.Title
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

	if err := s.repo.SaveScholarship(scholarship); err != nil {
		return nil, err
	}

	return scholarship, nil
}

func (s *Service) DeleteScholarship(instID, id uint) error {
	college, err := s.repo.FindCollegeByUniversityID(instID)
	if err != nil {
		return errors.New("no college found for this institution")
	}

	scholarship, err := s.repo.FindScholarshipByID(id)
	if err != nil {
		return errors.New("scholarship not found")
	}

	if scholarship.Provider != college.Name {
		return errors.New("you can only delete your own scholarships")
	}

	return s.repo.DeleteScholarship(id)
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
		InstitutionID: instID,
		Title:         extractAdmissionTitle(&dataStr),
		Status:        "draft",
		Data:          &dataStr,
	}

	if req.Status == "published" {
		now := time.Now()
		page.Status = "published"
		page.PublishedAt = &now
	}

	if err := s.repo.CreateAdmissionPage(page); err != nil {
		return nil, err
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

	if err := s.repo.SaveAdmissionPage(page); err != nil {
		return nil, err
	}

	return toAdmissionPageResponse(page), nil
}

func (s *Service) DeleteAdmissionPage(instID, id uint) error {
	return s.repo.DeleteAdmissionPage(id, instID)
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

func (s *Service) ListPublicInstitutions(page, limit int, search, location string) ([]PublicInstitutionResponse, int64, error) {
	users, total, err := s.repo.FindPublicInstitutions(page, limit, search, location)
	if err != nil {
		return nil, 0, err
	}

	results := make([]PublicInstitutionResponse, len(users))
	for i, u := range users {
		results[i] = PublicInstitutionResponse{
			ID:              u.ID,
			InstitutionName: u.InstitutionName,
			LogoURL:         u.LogoURL,
			BannerURL:       u.BannerURL,
			About:           u.About,
			District:        u.District,
			WebsiteURL:      u.WebsiteURL,
			Status:          u.Status,
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

	return &PublicInstitutionDetailResponse{
		ID:              user.ID,
		InstitutionName: user.InstitutionName,
		Claimed:         user.Claimed,
		LogoURL:         user.LogoURL,
		BannerURL:       user.BannerURL,
		About:           user.About,
		Vision:          user.Vision,
		Mission:         user.Mission,
		District:        user.District,
		WebsiteURL:      user.WebsiteURL,
		Videos:          pd.Videos,
		OverviewData:    pd.OverviewData,
		LeadershipData:  pd.LeadershipData,
		CoursesData:     pd.CoursesData,
		ProgramsData:    pd.ProgramsData,
		FacilitiesData:  pd.FacilitiesData,
		AlumniData:      pd.AlumniData,
		GalleryData:     pd.GalleryData,
		DownloadsData:   pd.DownloadsData,
	}, nil
}
