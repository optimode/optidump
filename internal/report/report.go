package report

import (
	"fmt"
	"net/smtp"
	"optidump/internal/config"
	"strings"
	"time"
)

// SavedFile contains information about a backed up file
type SavedFile struct {
	SQLSize               int64
	CompressedSize        int64
	CompressedFile        string
	SuccessfulSave        bool
	SuccessfulCompression bool
	Message               string
}

// Report handles email reporting for backup operations
type Report struct {
	startedAt  time.Time
	endedAt    time.Time
	savedFiles map[string]*SavedFile
}

// New creates a new report instance
func New(startedAt, endedAt time.Time, savedFiles map[string]*SavedFile) *Report {
	return &Report{
		startedAt:  startedAt,
		endedAt:    endedAt,
		savedFiles: savedFiles,
	}
}

// Send sends an email report about the backup operation
func (r *Report) Send(cfg config.ReportConfig, sectionName string, hasError bool) error {
	if len(cfg.Recipient) == 0 {
		return nil // No recipients configured
	}

	sender := cfg.Sender
	if sender == "" {
		sender = "optidump@localhost"
	}

	// Prepare email message
	subject := fmt.Sprintf("Report of a mysql backup: %s", sectionName)
	if hasError {
		subject += " (with errors)"
	}

	body := r.makeMessage(sectionName, hasError)

	// Build email headers and body
	message := r.buildEmailMessage(sender, cfg.Recipient, subject, body)

	// Send email via localhost SMTP
	err := smtp.SendMail(
		"localhost:25",
		nil, // No authentication
		sender,
		cfg.Recipient,
		[]byte(message),
	)

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// buildEmailMessage constructs the full email message with headers
func (r *Report) buildEmailMessage(sender string, recipients []string, subject, body string) string {
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("From: %s\r\n", sender))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", recipients[0]))

	if len(recipients) > 1 {
		cc := strings.Join(recipients[1:], ", ")
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
	}

	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.String()
}

// makeMessage creates the email message body
func (r *Report) makeMessage(sectionName string, hasError bool) string {
	var msg strings.Builder

	msg.WriteString("Dear Administrator,\n\n")
	msg.WriteString("We made a backup of the following data:\n\n")

	msg.WriteString(fmt.Sprintf("Section: %s\n", sectionName))
	msg.WriteString(fmt.Sprintf("Started at: %s\n", r.startedAt.Format("2006-01-02 15:04:05")))
	msg.WriteString(fmt.Sprintf("Ended at: %s\n\n", r.endedAt.Format("2006-01-02 15:04:05")))

	if hasError {
		msg.WriteString("Important! 1 or more errors were detected, please check!\n\n")
	}

	msg.WriteString("Performed backups:\n\n")

	if len(r.savedFiles) > 0 {
		i := 1
		for sqlFile, result := range r.savedFiles {
			msg.WriteString(fmt.Sprintf("%d. %s\n", i, sqlFile))

			if !result.SuccessfulSave {
				msg.WriteString("\tFailed to save!\n\n")
			} else {
				sqlSizeKB := float64(result.SQLSize) / 1024.0
				compressedSizeKB := float64(result.CompressedSize) / 1024.0

				msg.WriteString(fmt.Sprintf("\tSize of SQL: %.2f KB", sqlSizeKB))
				if result.CompressedSize > 0 {
					msg.WriteString(fmt.Sprintf(", Size of compressed file: %.2f KB", compressedSizeKB))
				}
				msg.WriteString("\n\n")
			}

			i++
		}
	} else {
		msg.WriteString("No files were backed up.\n\n")
	}

	msg.WriteString("Have a nice day!\n")

	return msg.String()
}
