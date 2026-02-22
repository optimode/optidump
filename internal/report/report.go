package report

import (
	"crypto/tls"
	"fmt"
	"net"
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

	// Apply defaults
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.Port
	if port == 0 {
		port = 25
	}

	encryption := cfg.Encryption
	if encryption == "" {
		encryption = "none"
	}

	// Prepare email message
	subject := fmt.Sprintf("Report of a mysql backup: %s", sectionName)
	if hasError {
		subject += " (with errors)"
	}

	body := r.makeMessage(sectionName, hasError)
	message := r.buildEmailMessage(sender, cfg.Recipient, subject, body)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", host, port)

	var client *smtp.Client
	var err error

	switch encryption {
	case "ssl":
		client, err = dialSSL(addr, host)
	case "starttls":
		client, err = dialStartTLS(addr, host)
	default:
		client, err = smtp.Dial(addr)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Authenticate if credentials provided
	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(sender); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}

	// Set recipients
	for _, recipient := range cfg.Recipient {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("SMTP RCPT TO failed for %s: %w", recipient, err)
		}
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}

	if _, err := w.Write([]byte(message)); err != nil {
		return fmt.Errorf("failed to write email body: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close email body: %w", err)
	}

	return client.Quit()
}

// dialSSL connects to an SMTP server over a direct TLS connection (port 465 typical)
func dialSSL(addr, host string) (*smtp.Client, error) {
	tlsConfig := &tls.Config{ServerName: host}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return client, nil
}

// dialStartTLS connects to an SMTP server in plain text, then upgrades to TLS
func dialStartTLS(addr, host string) (*smtp.Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return nil, err
	}

	tlsConfig := &tls.Config{ServerName: host}
	if err := client.StartTLS(tlsConfig); err != nil {
		client.Close()
		return nil, fmt.Errorf("STARTTLS failed: %w", err)
	}

	return client, nil
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
