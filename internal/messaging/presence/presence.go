package presence

type PresenceService interface {
	SetOnline(userType string, userID uint) error
	SetOffline(userType string, userID uint) error
	IsOnline(userType string, userID uint) (bool, error)
	GetOnlineUsers(userType string) ([]uint, error)
}
