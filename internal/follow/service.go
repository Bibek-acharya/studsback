package follow

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Follow(userID, targetID uint, targetType string) error {
	return s.repo.Follow(userID, targetID, targetType)
}

func (s *Service) Unfollow(userID, targetID uint, targetType string) error {
	return s.repo.Unfollow(userID, targetID, targetType)
}

func (s *Service) IsFollowing(userID, targetID uint, targetType string) (bool, error) {
	return s.repo.IsFollowing(userID, targetID, targetType)
}

func (s *Service) GetFollowedInstitutions(userID uint) ([]uint, error) {
	return s.repo.GetFollowedTargetIDs(userID, "institution")
}

func (s *Service) GetFollowedUniversities(userID uint) ([]uint, error) {
	return s.repo.GetFollowedTargetIDs(userID, "university")
}
