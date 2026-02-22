package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// validSection returns a minimal valid SectionConfig for testing.
func validSection() SectionConfig {
	return SectionConfig{
		Server: ServerConfig{
			Host:     "localhost",
			Port:     3306,
			User:     "root",
			Password: "secret",
		},
		Backup: BackupConfig{
			Mode:        "file_per_table",
			Compression: "gz",
			Destination: "/opt/backup",
		},
		Logging: LoggingConfig{
			File:   "/var/log/optidump.log",
			Level:  "info",
			Format: "text",
		},
	}
}

// --- contains ---

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		value  string
		expect bool
	}{
		{"found at start", []string{"a", "b", "c"}, "a", true},
		{"found in middle", []string{"a", "b", "c"}, "b", true},
		{"found at end", []string{"a", "b", "c"}, "c", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"single element match", []string{"a"}, "a", true},
		{"single element no match", []string{"a"}, "b", false},
		{"case sensitive", []string{"Debug"}, "debug", false},
		{"empty string in slice", []string{""}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.value)
			if got != tt.expect {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.value, got, tt.expect)
			}
		})
	}
}

// --- Load ---

func TestLoad_ValidYAML(t *testing.T) {
	yaml := `
production:
  server:
    host: localhost
    port: 3306
    user: root
    password: secret
  backup:
    mode: file_per_table
    compression: gz
    destination: /opt/backup
  logging:
    file: /var/log/test.log
    level: info
    format: text
`
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	section, exists := cfg["production"]
	if !exists {
		t.Fatal("section 'production' not found")
	}
	if section.Server.Host != "localhost" {
		t.Errorf("Host = %q, want %q", section.Server.Host, "localhost")
	}
	if section.Server.Port != 3306 {
		t.Errorf("Port = %d, want %d", section.Server.Port, 3306)
	}
	if section.Backup.Mode != "file_per_table" {
		t.Errorf("Mode = %q, want %q", section.Backup.Mode, "file_per_table")
	}
}

func TestLoad_MultipleSections(t *testing.T) {
	yaml := `
production:
  server:
    host: prod-db
    port: 3306
    user: root
    password: secret
  backup:
    mode: file_per_table
    destination: /opt/backup
  logging:
    file: /var/log/prod.log
    level: info
development:
  server:
    host: localhost
    port: 3306
    user: dev
    password: dev
  backup:
    mode: file_per_database
    destination: /tmp/backup
  logging:
    file: /tmp/dev.log
    level: debug
`
	path := filepath.Join(t.TempDir(), "config.yml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg) != 2 {
		t.Errorf("got %d sections, want 2", len(cfg))
	}
	if _, ok := cfg["production"]; !ok {
		t.Error("missing section 'production'")
	}
	if _, ok := cfg["development"]; !ok {
		t.Error("missing section 'development'")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yml")
	if err == nil {
		t.Fatal("Load() expected error for nonexistent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yml")
	os.WriteFile(path, []byte(":\n  invalid: [unclosed"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for invalid YAML")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.yml")
	os.WriteFile(path, []byte(""), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg) != 0 {
		t.Errorf("got %d sections, want 0", len(cfg))
	}
}

func TestLoad_WithOnlyAndExclude(t *testing.T) {
	yaml := `
test:
  server:
    host: localhost
    port: 3306
    user: root
    password: secret
  backup:
    mode: file_per_table
    destination: /opt/backup
  logging:
    file: /tmp/test.log
    level: info
  only:
    mydb:
      - table1
      - table2
  exclude:
    information_schema:
`
	path := filepath.Join(t.TempDir(), "config.yml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	section := cfg["test"]
	if section.Only == nil {
		t.Fatal("Only map is nil")
	}
	if _, ok := section.Only["mydb"]; !ok {
		t.Error("Only missing 'mydb'")
	}
	if section.Exclude == nil {
		t.Fatal("Exclude map is nil")
	}
	if _, ok := section.Exclude["information_schema"]; !ok {
		t.Error("Exclude missing 'information_schema'")
	}
}

// --- Validate ---

func TestValidate_EmptyConfig(t *testing.T) {
	cfg := make(Config)
	valid, errors := Validate(cfg)
	if valid {
		t.Error("Validate() should return false for empty config")
	}
	if len(errors) != 1 || errors[0] != "Configuration does not contain sections" {
		t.Errorf("unexpected errors: %v", errors)
	}
}

func TestValidate_ValidSection(t *testing.T) {
	cfg := Config{"production": validSection()}
	valid, errors := Validate(cfg)
	if !valid {
		t.Errorf("Validate() returned false, errors: %v", errors)
	}
}

func TestValidate_ServerHostEmpty(t *testing.T) {
	s := validSection()
	s.Server.Host = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.server.host is empty")
}

func TestValidate_ServerUserEmpty(t *testing.T) {
	s := validSection()
	s.Server.User = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.server.user is empty")
}

func TestValidate_ServerPasswordEmpty(t *testing.T) {
	s := validSection()
	s.Server.Password = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.server.password is empty")
}

func TestValidate_ServerPortAndSocketEmpty(t *testing.T) {
	s := validSection()
	s.Server.Port = 0
	s.Server.Socket = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.server.socket and port are empty")
}

func TestValidate_ServerSocketInsteadOfPort(t *testing.T) {
	s := validSection()
	s.Server.Port = 0
	s.Server.Socket = "/var/run/mysqld/mysqld.sock"
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if !valid {
		t.Errorf("expected valid, errors: %v", errors)
	}
}

func TestValidate_BackupModeEmpty(t *testing.T) {
	s := validSection()
	s.Backup.Mode = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.backup.mode is empty")
}

func TestValidate_BackupModeInvalid(t *testing.T) {
	s := validSection()
	s.Backup.Mode = "full_backup"
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.backup.mode is not correct value")
}

func TestValidate_BackupModeFilePerDatabase(t *testing.T) {
	s := validSection()
	s.Backup.Mode = "file_per_database"
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if !valid {
		t.Errorf("expected valid, errors: %v", errors)
	}
}

func TestValidate_BackupCompressionValues(t *testing.T) {
	validValues := []string{"gz", "bz2", "", "false"}
	for _, v := range validValues {
		t.Run("compression_"+v, func(t *testing.T) {
			s := validSection()
			s.Backup.Compression = v
			cfg := Config{"test": s}
			valid, errors := Validate(cfg)
			if !valid {
				t.Errorf("compression %q should be valid, errors: %v", v, errors)
			}
		})
	}
}

func TestValidate_BackupCompressionInvalid(t *testing.T) {
	s := validSection()
	s.Backup.Compression = "zip"
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.backup.compression is not correct value")
}

func TestValidate_BackupDestinationEmpty(t *testing.T) {
	s := validSection()
	s.Backup.Destination = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.backup.destination is empty")
}

func TestValidate_LoggingFileEmpty(t *testing.T) {
	s := validSection()
	s.Logging.File = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.logging.file is empty")
}

func TestValidate_LoggingLevelEmpty(t *testing.T) {
	s := validSection()
	s.Logging.Level = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.logging.level is empty")
}

func TestValidate_LoggingLevelInvalid(t *testing.T) {
	s := validSection()
	s.Logging.Level = "verbose"
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.logging.level is not correct value (verbose)")
}

func TestValidate_LoggingLevelValues(t *testing.T) {
	for _, level := range []string{"debug", "info", "error"} {
		t.Run("level_"+level, func(t *testing.T) {
			s := validSection()
			s.Logging.Level = level
			cfg := Config{"test": s}
			valid, errors := Validate(cfg)
			if !valid {
				t.Errorf("level %q should be valid, errors: %v", level, errors)
			}
		})
	}
}

func TestValidate_LoggingFormatEmpty_DefaultsToText(t *testing.T) {
	s := validSection()
	s.Logging.Format = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if !valid {
		t.Errorf("expected valid with empty format (defaults to text), errors: %v", errors)
	}
}

func TestValidate_LoggingFormatInvalid(t *testing.T) {
	s := validSection()
	s.Logging.Format = "xml"
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.logging.format is not correct value (xml)")
}

func TestValidate_LoggingFormatJSON(t *testing.T) {
	s := validSection()
	s.Logging.Format = "json"
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if !valid {
		t.Errorf("expected valid, errors: %v", errors)
	}
}

func TestValidate_ReportEmptyRecipients(t *testing.T) {
	s := validSection()
	s.Report.Recipient = []string{}
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if !valid {
		t.Errorf("expected valid with empty recipients, errors: %v", errors)
	}
}

func TestValidate_ReportEmptyRecipientEntry(t *testing.T) {
	s := validSection()
	s.Report.Recipient = []string{"admin@example.com", ""}
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.report.recipient.1 is empty")
}

func TestValidate_MultipleErrors(t *testing.T) {
	s := SectionConfig{} // everything empty
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	if len(errors) < 5 {
		t.Errorf("expected at least 5 errors for empty section, got %d: %v", len(errors), errors)
	}
}

// --- Report SMTP validation ---

func TestLoad_WithReportSMTPConfig(t *testing.T) {
	yaml := `
test:
  server:
    host: localhost
    port: 3306
    user: root
    password: secret
  backup:
    mode: file_per_table
    destination: /opt/backup
  logging:
    file: /tmp/test.log
    level: info
  report:
    sender: backup@example.com
    recipient:
      - admin@example.com
    host: smtp.example.com
    port: 587
    encryption: starttls
    username: smtpuser
    password: smtppass
`
	path := filepath.Join(t.TempDir(), "config.yml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	r := cfg["test"].Report
	if r.Host != "smtp.example.com" {
		t.Errorf("Host = %q, want %q", r.Host, "smtp.example.com")
	}
	if r.Port != 587 {
		t.Errorf("Port = %d, want %d", r.Port, 587)
	}
	if r.Encryption != "starttls" {
		t.Errorf("Encryption = %q, want %q", r.Encryption, "starttls")
	}
	if r.Username != "smtpuser" {
		t.Errorf("Username = %q, want %q", r.Username, "smtpuser")
	}
	if r.Password != "smtppass" {
		t.Errorf("Password = %q, want %q", r.Password, "smtppass")
	}
}

func TestValidate_ReportEncryptionValues(t *testing.T) {
	for _, enc := range []string{"none", "starttls", "ssl", ""} {
		t.Run("encryption_"+enc, func(t *testing.T) {
			s := validSection()
			s.Report.Encryption = enc
			cfg := Config{"test": s}
			valid, errors := Validate(cfg)
			if !valid {
				t.Errorf("encryption %q should be valid, errors: %v", enc, errors)
			}
		})
	}
}

func TestValidate_ReportEncryptionInvalid(t *testing.T) {
	s := validSection()
	s.Report.Encryption = "tls13"
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.report.encryption is not correct value (tls13)")
}

func TestValidate_ReportPortValid(t *testing.T) {
	for _, port := range []int{0, 25, 465, 587, 65535} {
		t.Run(fmt.Sprintf("port_%d", port), func(t *testing.T) {
			s := validSection()
			s.Report.Port = port
			cfg := Config{"test": s}
			valid, errors := Validate(cfg)
			if !valid {
				t.Errorf("port %d should be valid, errors: %v", port, errors)
			}
		})
	}
}

func TestValidate_ReportPortInvalid(t *testing.T) {
	s := validSection()
	s.Report.Port = -1
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid for port -1")
	}
	assertContainsError(t, errors, "test.report.port is not a valid port number (-1)")
}

func TestValidate_ReportPasswordWithoutUsername(t *testing.T) {
	s := validSection()
	s.Report.Password = "secret"
	s.Report.Username = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if valid {
		t.Error("expected invalid")
	}
	assertContainsError(t, errors, "test.report.password is set but username is empty")
}

func TestValidate_ReportUsernameWithoutPassword(t *testing.T) {
	s := validSection()
	s.Report.Username = "user"
	s.Report.Password = ""
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if !valid {
		t.Errorf("username without password should be valid, errors: %v", errors)
	}
}

func TestValidate_ReportUsernameAndPassword(t *testing.T) {
	s := validSection()
	s.Report.Username = "user"
	s.Report.Password = "pass"
	cfg := Config{"test": s}
	valid, errors := Validate(cfg)
	if !valid {
		t.Errorf("username and password should be valid, errors: %v", errors)
	}
}

// --- helpers ---

func assertContainsError(t *testing.T, errors []string, expected string) {
	t.Helper()
	for _, e := range errors {
		if e == expected {
			return
		}
	}
	t.Errorf("expected error %q not found in %v", expected, errors)
}
