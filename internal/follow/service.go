package follow

import (
	"context"

	"studsphere/backend/internal/notification"
)

type Service struct {
	repo     *Repository
	notifier notification.Notifier
}

func NewService(repo *Repository, notifier notification.Notifier) *Service {
	return &Service{repo: repo, notifier: notifier}
}

func (s *Service) Follow(userID, targetID uint, targetType string) error {
	created, err := s.repo.Follow(userID, targetID, targetType)
	if err != nil || !created {
		return err
	}

	// Only claimed institutions have an inbox identity. Universities have no
	// owner account and providers are not followable (D-Q6) — the follow
	// succeeds silently for any target the notification cannot address.
	if targetType == "institution" || targetType == "" {
		if instUserID, err := s.repo.ApprovedInstitutionUserID(targetID); err == nil && instUserID != 0 {
			data := map[string]any{"name": "Someone"}
			if name, err := s.repo.UserNameByID(userID); err == nil && name != "" {
				data["name"] = name
			}
			_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventSocialNewFollower,
				Recipients: []notification.Ref{{Type: "institution", ID: instUserID}},
				Data:       data,
			})
		}
	}

	return nil
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
