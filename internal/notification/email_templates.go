// internal/notification/email_templates.go
package notification

import (
	"fmt"
	"html"
)

// renderEmail renders subject and HTML body for an email delivery.
// Events with EmailTmpl set (welcome) get a dedicated render; everything else
// renders generically from the registry Title/Body templates plus the link.
func renderEmail(def EventDef, req NotifyRequest) (string, string, error) {
	if def.EmailTmpl == "welcome" {
		// Welcome email uses emailqueue's richer template at send time; this
		// generic render is the notification-module subject/body fallback.
		firstName, _ := req.Data["first_name"].(string)
		if firstName == "" {
			firstName = "User"
		}
		subject := "Welcome to StudSphere"
		htmlBody := fmt.Sprintf("<p>Hi %s, welcome to StudSphere!</p>", html.EscapeString(firstName))
		return subject, htmlBody, nil
	}
	subject, err := ResolveTemplate(def.TitleTpl, req.Data)
	if err != nil || subject == "" {
		subject = humanizeKey(def.Key) // fallback, same as in-app
	}
	body, err := ResolveTemplate(def.BodyTpl, req.Data)
	if err != nil || body == "" {
		body = subject
	}
	link := def.LinkTpl
	if req.Link != "" {
		link = req.Link
	}
	htmlBody := fmt.Sprintf("<p>%s</p>", html.EscapeString(body))
	if link != "" {
		htmlBody += fmt.Sprintf(`<p><a href="%s">View details</a></p>`, html.EscapeString(link))
	}
	return subject, htmlBody, nil
}
