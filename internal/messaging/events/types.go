package events

const (
	SubjectMessageCreated      = "messaging.message.created"
	SubjectMessageRead         = "messaging.message.read"
	SubjectMessageDelivered    = "messaging.message.delivered"
	SubjectMessageEdited       = "messaging.message.edited"
	SubjectMessageDeleted      = "messaging.message.deleted"
	SubjectTypingStart         = "messaging.typing.start"
	SubjectTypingStop          = "messaging.typing.stop"
	SubjectPresenceChanged     = "messaging.presence.changed"
	SubjectConversationUpdated = "messaging.conversation.updated"
)

type Event struct {
	Version int         `json:"version"`
	Type    string      `json:"type"`
	Data    interface{} `json:"data"`
}
