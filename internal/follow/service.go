package follow

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Follow(userID, institutionID uint) error {
	return s.repo.Follow(userID, institutionID)
}

func (s *Service) Unfollow(userID, institutionID uint) error {
	return s.repo.Unfollow(userID, institutionID)
}

func (s *Service) IsFollowing(userID, institutionID uint) (bool, error) {
	return s.repo.IsFollowing(userID, institutionID)
}

func (s *Service) GetFollowedInstitutions(userID uint) ([]uint, error) {
	return s.repo.GetFollowedInstitutionIDs(userID)
}
