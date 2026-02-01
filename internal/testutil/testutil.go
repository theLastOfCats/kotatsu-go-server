package testutil

import (
	"log"
	"testing"

	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
)

// SetupTestDB creates an in-memory SQLite DB with schema
func SetupTestDB(t *testing.T) *db.DB {
	database, err := db.New("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to init in-memory db: %v", err)
	}
	return database
}

// MockMailSender captures emails for testing
type MockMailSender struct {
	SentEmails []SentEmail
}

type SentEmail struct {
	To       string
	Subject  string
	TextBody string
	HtmlBody string
}

func (m *MockMailSender) Send(to string, subject string, textBody string, htmlBody string) error {
	m.SentEmails = append(m.SentEmails, SentEmail{to, subject, textBody, htmlBody})
	log.Printf("Mock email sent to %s", to)
	return nil
}
