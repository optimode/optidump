package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig contains MySQL server connection settings
type ServerConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Socket   string `yaml:"socket"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// BackupConfig contains backup-specific settings
type BackupConfig struct {
	Mode        string `yaml:"mode"`        // file_per_table, file_per_database
	Compression string `yaml:"compression"` // gz, bz2, or empty/false for no compression
	Destination string `yaml:"destination"`
	Command     string `yaml:"command"`
	Options     string `yaml:"options"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	File   string `yaml:"file"`
	Level  string `yaml:"level"`  // debug, info, error
	Format string `yaml:"format"` // text, json
}

// ReportConfig contains email reporting settings
type ReportConfig struct {
	Sender     string   `yaml:"sender"`
	Recipient  []string `yaml:"recipient"`
	Host       string   `yaml:"host"`
	Port       int      `yaml:"port"`
	Encryption string   `yaml:"encryption"` // none, starttls, ssl
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password"`
}

// SectionConfig represents a complete backup section configuration
type SectionConfig struct {
	Server  ServerConfig           `yaml:"server"`
	Backup  BackupConfig           `yaml:"backup"`
	Logging LoggingConfig          `yaml:"logging"`
	Report  ReportConfig           `yaml:"report"`
	Only    map[string]interface{} `yaml:"only,omitempty"`
	Exclude map[string]interface{} `yaml:"exclude,omitempty"`
}

// Config holds all section configurations
type Config map[string]SectionConfig

// Load reads and parses a YAML configuration file
func Load(filepath string) (Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("configuration file does not exist: %s", filepath)
	}

	config := make(Config)
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("syntax error in configuration file: %w", err)
	}

	return config, nil
}

// Validate validates the entire configuration
func Validate(cfg Config) (bool, []string) {
	var errors []string

	if len(cfg) == 0 {
		errors = append(errors, "Configuration does not contain sections")
		return false, errors
	}

	for sectionName, section := range cfg {
		sectionErrors := validateSection(sectionName, section)
		errors = append(errors, sectionErrors...)
	}

	return len(errors) == 0, errors
}

// validateSection validates a single section configuration
func validateSection(name string, section SectionConfig) []string {
	var errors []string

	// Validate server configuration
	errors = append(errors, validateServer(name, section.Server)...)

	// Validate backup configuration
	errors = append(errors, validateBackup(name, section.Backup)...)

	// Validate logging configuration
	errors = append(errors, validateLogging(name, section.Logging)...)

	// Validate report configuration
	errors = append(errors, validateReport(name, section.Report)...)

	return errors
}

// validateServer validates server configuration
func validateServer(sectionName string, server ServerConfig) []string {
	var errors []string

	if server.Host == "" {
		errors = append(errors, fmt.Sprintf("%s.server.host is empty", sectionName))
	}

	if server.User == "" {
		errors = append(errors, fmt.Sprintf("%s.server.user is empty", sectionName))
	}

	if server.Password == "" {
		errors = append(errors, fmt.Sprintf("%s.server.password is empty", sectionName))
	}

	if server.Port == 0 && server.Socket == "" {
		errors = append(errors, fmt.Sprintf("%s.server.socket and port are empty", sectionName))
	}

	return errors
}

// validateBackup validates backup configuration
func validateBackup(sectionName string, backup BackupConfig) []string {
	var errors []string
	validModes := []string{"file_per_table", "file_per_database"}
	validCompression := []string{"gz", "bz2", "", "false"}

	if backup.Mode == "" {
		errors = append(errors, fmt.Sprintf("%s.backup.mode is empty", sectionName))
	} else if !contains(validModes, backup.Mode) {
		errors = append(errors, fmt.Sprintf("%s.backup.mode is not correct value", sectionName))
	}

	if !contains(validCompression, backup.Compression) {
		errors = append(errors, fmt.Sprintf("%s.backup.compression is not correct value", sectionName))
	}

	if backup.Destination == "" {
		errors = append(errors, fmt.Sprintf("%s.backup.destination is empty", sectionName))
	}

	return errors
}

// validateLogging validates logging configuration
func validateLogging(sectionName string, logging LoggingConfig) []string {
	var errors []string
	validLevels := []string{"debug", "info", "error"}
	validFormats := []string{"text", "json"}

	if logging.File == "" {
		errors = append(errors, fmt.Sprintf("%s.logging.file is empty", sectionName))
	}

	if logging.Level == "" {
		errors = append(errors, fmt.Sprintf("%s.logging.level is empty", sectionName))
	} else if !contains(validLevels, logging.Level) {
		errors = append(errors, fmt.Sprintf("%s.logging.level is not correct value (%s)", sectionName, logging.Level))
	}

	// Set default format to "text" if not specified
	if logging.Format == "" {
		logging.Format = "text"
	} else if !contains(validFormats, logging.Format) {
		errors = append(errors, fmt.Sprintf("%s.logging.format is not correct value (%s)", sectionName, logging.Format))
	}

	return errors
}

// validateReport validates report configuration
func validateReport(sectionName string, report ReportConfig) []string {
	var errors []string
	validEncryptions := []string{"none", "starttls", "ssl", ""}

	if len(report.Recipient) > 0 {
		for i, recipient := range report.Recipient {
			if recipient == "" {
				errors = append(errors, fmt.Sprintf("%s.report.recipient.%d is empty", sectionName, i))
			}
		}
	}

	if !contains(validEncryptions, report.Encryption) {
		errors = append(errors, fmt.Sprintf("%s.report.encryption is not correct value (%s)", sectionName, report.Encryption))
	}

	if report.Port < 0 || report.Port > 65535 {
		errors = append(errors, fmt.Sprintf("%s.report.port is not a valid port number (%d)", sectionName, report.Port))
	}

	if report.Password != "" && report.Username == "" {
		errors = append(errors, fmt.Sprintf("%s.report.password is set but username is empty", sectionName))
	}

	return errors
}

// contains checks if a string slice contains a specific value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
