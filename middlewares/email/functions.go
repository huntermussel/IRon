package email

import (
	"fmt"
	"strings"
)

func searchEmails(query string) string {
	// In a real implementation, this would use an Email API (e.g., Gmail).
	// For now, we return a mock response based on the query.
	if strings.Contains(strings.ToLower(query), "invoice") {
		return fmt.Sprintf("[MOCK] search_emails: found 1 email for '%s'\n1. Subject: Invoice #123 (from: billing@service.com)", query)
	}
	return fmt.Sprintf("[MOCK] search_emails: found 2 emails for '%s'\n1. Subject: Meeting (from: boss@corp.com)\n2. Subject: Re: Project (from: team@corp.com)", query)
}

func sendEmail(to, subject, body string) string {
	// In a real implementation, this would use an Email API (e.g., SMTP or Gmail).
	// For now, we return a mock response.
	return fmt.Sprintf("[MOCK] send_email: sent to %s with subject '%s'", to, subject)
}
