package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"optidump/internal/backup"
	"optidump/internal/config"
	"optidump/internal/logger"
	"optidump/internal/report"

	"golang.org/x/term"
)

// Build-time variables set via ldflags
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

const description = "Make backup of MySQL databases to backup destination with optional compression, exclude and/or only list"

var (
	configFile  = flag.String("config", "/etc/optidump/config.yml", "Absolute path to configuration file")
	section     = flag.String("section", "", "Section from configuration file")
	checkConfig = flag.Bool("check-config", false, "Check configuration file syntax and exit")
	dryRun      = flag.Bool("dry-run", false, "Test mode to examine which databases and tables will be backed up")
	level       = flag.String("level", "", "Logging level, overrides the configuration (debug, info, error)")
	console     = flag.Bool("console", false, "Log to console/stdout instead of file (auto-enabled in interactive terminal)")
	showVersion = flag.Bool("version", false, "Show version information")
)

func main() {
	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", description)
		fmt.Fprintf(os.Stderr, "Version: %s\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s --config /path/to/config.yml --section section_name\n", filepath.Base(os.Args[0]))
	}

	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("optidump %s (commit: %s, built: %s)\n", version, gitCommit, buildTime)
		os.Exit(0)
	}

	// Make config path absolute if it's relative
	if !filepath.IsAbs(*configFile) {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}
		*configFile = filepath.Join(cwd, *configFile)
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration
	valid, errors := config.Validate(cfg)
	if !valid {
		fmt.Fprintln(os.Stderr, "Configuration verification failed:")
		for _, errMsg := range errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", errMsg)
		}
		os.Exit(1)
	}

	// Check config mode - just validate and exit
	if *checkConfig {
		fmt.Println("Configuration is valid")
		os.Exit(0)
	}

	// Validate section parameter
	if *section == "" {
		fmt.Fprintln(os.Stderr, "Error: section parameter is required")
		flag.Usage()
		os.Exit(1)
	}

	// Check if section exists
	sectionConfig, exists := cfg[*section]
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: section '%s' not found in configuration\n", *section)
		os.Exit(1)
	}

	// Override log level if specified
	if *level != "" {
		sectionConfig.Logging.Level = *level
	}

	// Set default log format if not specified
	if sectionConfig.Logging.Format == "" {
		sectionConfig.Logging.Format = "text"
	}

	// Detect if running in interactive terminal
	isTerminal := term.IsTerminal(int(os.Stdin.Fd()))
	useConsole := *console || (isTerminal && *dryRun)

	// Setup logger
	log, err := logger.New(sectionConfig.Logging.File, sectionConfig.Logging.Level, sectionConfig.Logging.Format, useConsole)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	// Start backup process
	startedAt := time.Now()

	dryMode := ""
	if *dryRun {
		dryMode = " in dry mode"
	}
	log.Info(fmt.Sprintf("Starting %s backup%s", *section, dryMode))

	if *dryRun {
		log.Info("Dry mode does not perform actual backup operations")
	}

	// Create backup instance and run
	bkp := backup.New(log)
	if err := bkp.Backup(*section, sectionConfig, *dryRun); err != nil {
		log.Error(fmt.Sprintf("Backup failed: %v", err))
		os.Exit(1)
	}

	// Check for errors
	if bkp.HasError() {
		log.Error("Important! 1 or more errors were detected, please check!")
	}

	log.Info(fmt.Sprintf("Stopped %s backup", *section))
	endedAt := time.Now()

	// Send report if configured and not in dry-run mode
	if !*dryRun && len(sectionConfig.Report.Recipient) > 0 {
		rpt := report.New(startedAt, endedAt, bkp.GetSavedFiles())
		if err := rpt.Send(sectionConfig.Report, *section, bkp.HasError()); err != nil {
			log.Error(fmt.Sprintf("Failed to send report: %v", err))
		} else {
			log.Info("Report sent successfully")
		}
	}

	// Exit with error code if backup had errors
	if bkp.HasError() {
		os.Exit(1)
	}
}
