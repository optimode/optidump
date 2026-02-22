package report

import (
	"optidump/internal/config"
	"strings"
	"testing"
	"time"
)

func sampleTimes() (time.Time, time.Time) {
	start := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	end := time.Date(2025, 6, 15, 10, 35, 45, 0, time.UTC)
	return start, end
}

// --- New ---

func TestNew(t *testing.T) {
	start, end := sampleTimes()
	files := map[string]*SavedFile{
		"/opt/backup/test.sql": {SQLSize: 1024, SuccessfulSave: true},
	}

	r := New(start, end, files)

	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.startedAt != start {
		t.Error("startedAt mismatch")
	}
	if r.endedAt != end {
		t.Error("endedAt mismatch")
	}
	if len(r.savedFiles) != 1 {
		t.Errorf("savedFiles len = %d, want 1", len(r.savedFiles))
	}
}

// --- makeMessage ---

func TestMakeMessage_Basic(t *testing.T) {
	start, end := sampleTimes()
	files := map[string]*SavedFile{
		"/opt/backup/users.sql": {
			SQLSize:        2048,
			SuccessfulSave: true,
		},
	}
	r := New(start, end, files)

	msg := r.makeMessage("production", false)

	if !strings.Contains(msg, "Dear Administrator") {
		t.Error("missing greeting")
	}
	if !strings.Contains(msg, "Section: production") {
		t.Error("missing section name")
	}
	if !strings.Contains(msg, "2025-06-15 10:30:00") {
		t.Error("missing start time")
	}
	if !strings.Contains(msg, "2025-06-15 10:35:45") {
		t.Error("missing end time")
	}
	if !strings.Contains(msg, "users.sql") {
		t.Error("missing file name")
	}
	if !strings.Contains(msg, "KB") {
		t.Error("missing size info")
	}
	if strings.Contains(msg, "errors were detected") {
		t.Error("should not contain error notice when hasError=false")
	}
}

func TestMakeMessage_WithError(t *testing.T) {
	start, end := sampleTimes()
	r := New(start, end, map[string]*SavedFile{})

	msg := r.makeMessage("production", true)

	if !strings.Contains(msg, "errors were detected") {
		t.Error("should contain error notice when hasError=true")
	}
}

func TestMakeMessage_FailedSave(t *testing.T) {
	start, end := sampleTimes()
	files := map[string]*SavedFile{
		"/opt/backup/broken.sql": {
			SuccessfulSave: false,
		},
	}
	r := New(start, end, files)

	msg := r.makeMessage("test", false)

	if !strings.Contains(msg, "Failed to save") {
		t.Error("should indicate failed save")
	}
}

func TestMakeMessage_WithCompression(t *testing.T) {
	start, end := sampleTimes()
	files := map[string]*SavedFile{
		"/opt/backup/data.sql": {
			SQLSize:               10240,
			CompressedSize:        2048,
			CompressedFile:        "/opt/backup/data.sql.tar.gz",
			SuccessfulSave:        true,
			SuccessfulCompression: true,
		},
	}
	r := New(start, end, files)

	msg := r.makeMessage("test", false)

	if !strings.Contains(msg, "Size of SQL") {
		t.Error("should contain SQL size")
	}
	if !strings.Contains(msg, "Size of compressed file") {
		t.Error("should contain compressed size")
	}
}

func TestMakeMessage_NoFiles(t *testing.T) {
	start, end := sampleTimes()
	r := New(start, end, map[string]*SavedFile{})

	msg := r.makeMessage("empty", false)

	if !strings.Contains(msg, "No files were backed up") {
		t.Error("should indicate no files")
	}
}

func TestMakeMessage_EndsWithGreeting(t *testing.T) {
	start, end := sampleTimes()
	r := New(start, end, map[string]*SavedFile{})

	msg := r.makeMessage("test", false)

	if !strings.Contains(msg, "Have a nice day") {
		t.Error("should end with greeting")
	}
}

// --- buildEmailMessage ---

func TestBuildEmailMessage_SingleRecipient(t *testing.T) {
	start, end := sampleTimes()
	r := New(start, end, map[string]*SavedFile{})

	msg := r.buildEmailMessage(
		"backup@example.com",
		[]string{"admin@example.com"},
		"Test Subject",
		"Test body",
	)

	if !strings.Contains(msg, "From: backup@example.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(msg, "To: admin@example.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(msg, "Subject: Test Subject") {
		t.Error("missing Subject header")
	}
	if !strings.Contains(msg, "Content-Type: text/plain; charset=UTF-8") {
		t.Error("missing Content-Type header")
	}
	if !strings.Contains(msg, "Test body") {
		t.Error("missing body")
	}
	if strings.Contains(msg, "Cc:") {
		t.Error("single recipient should not have Cc header")
	}
}

func TestBuildEmailMessage_MultipleRecipients(t *testing.T) {
	start, end := sampleTimes()
	r := New(start, end, map[string]*SavedFile{})

	msg := r.buildEmailMessage(
		"backup@example.com",
		[]string{"admin@example.com", "ops@example.com", "dev@example.com"},
		"Subject",
		"Body",
	)

	if !strings.Contains(msg, "To: admin@example.com") {
		t.Error("first recipient should be in To")
	}
	if !strings.Contains(msg, "Cc: ops@example.com, dev@example.com") {
		t.Error("additional recipients should be in Cc")
	}
}

// --- Send ---

func TestSend_EmptyRecipients(t *testing.T) {
	start, end := sampleTimes()
	r := New(start, end, map[string]*SavedFile{})

	err := r.Send(config.ReportConfig{
		Recipient: []string{},
	}, "test", false)

	if err != nil {
		t.Errorf("Send() with empty recipients should return nil, got: %v", err)
	}
}

func TestSend_NilRecipients(t *testing.T) {
	start, end := sampleTimes()
	r := New(start, end, map[string]*SavedFile{})

	err := r.Send(config.ReportConfig{}, "test", false)

	if err != nil {
		t.Errorf("Send() with nil recipients should return nil, got: %v", err)
	}
}
