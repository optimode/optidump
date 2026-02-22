package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"optidump/internal/config"
	"optidump/internal/database"
	"optidump/internal/logger"
	"optidump/internal/report"
)

// Command represents a single backup command to execute
type Command struct {
	DumpCommand string
	Directory   string
	Filename    string
}

// Backup handles the backup process
type Backup struct {
	logger      *logger.Logger
	backupList  map[string][]string
	commandList []Command
	savedFiles  map[string]*report.SavedFile
	hasError    bool
}

// New creates a new backup instance
func New(log *logger.Logger) *Backup {
	return &Backup{
		logger:      log,
		backupList:  make(map[string][]string),
		commandList: make([]Command, 0),
		savedFiles:  make(map[string]*report.SavedFile),
		hasError:    false,
	}
}

// Backup performs the complete backup process
func (b *Backup) Backup(sectionName string, section config.SectionConfig, dryRun bool) error {
	// Create timestamped destination directory
	timestamp := time.Now().Format("2006-01-02-15-04")
	destination := filepath.Join(section.Backup.Destination, timestamp)

	if !dryRun {
		if err := os.MkdirAll(destination, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	// Create database connection
	conn := database.New(section.Server)

	if section.Backup.Command != "" {
		conn.SetCommand(section.Backup.Command)
	}
	if section.Backup.Options != "" {
		conn.SetOptions(section.Backup.Options)
	}

	if err := conn.Connect(); err != nil {
		b.logger.Error(fmt.Sprintf("Cannot make a database connection with %s: %v", sectionName, err))
		return err
	}
	defer conn.Close()

	// Load databases and tables
	if err := conn.LoadDatabases(); err != nil {
		return fmt.Errorf("failed to load databases: %w", err)
	}
	if err := conn.LoadTables(); err != nil {
		return fmt.Errorf("failed to load tables: %w", err)
	}

	// Apply filters
	if section.Only != nil && len(section.Only) > 0 {
		b.applyOnly(conn, section)
	} else if conn.HasDatabases() {
		b.applyAllDatabases(conn)
	}

	if section.Exclude != nil && len(section.Exclude) > 0 {
		b.applyExclude(section)
	}

	// Generate backup commands
	if len(b.backupList) > 0 {
		b.makeBackupCommands(conn, section, destination)
	}

	// Dry run mode - just print what would be done
	if dryRun {
		b.printBackupList()
		b.printBackupCommands()
		return nil
	}

	// Execute backups
	b.doBackup()

	// Compress if needed
	if section.Backup.Compression != "" && section.Backup.Compression != "false" {
		b.doCompression(section.Backup.Compression)
	}

	return nil
}

// applyOnly applies the "only" filter to include specific databases/tables
func (b *Backup) applyOnly(conn *database.Connection, section config.SectionConfig) {
	for database, value := range section.Only {
		if b.backupList[database] == nil {
			b.backupList[database] = make([]string, 0)
		}

		switch v := value.(type) {
		case []interface{}:
			// Specific tables listed
			for _, tableInterface := range v {
				if table, ok := tableInterface.(string); ok {
					b.backupList[database] = append(b.backupList[database], table)
				}
			}
		case nil:
			// All tables in database
			tables := conn.GetTables(database)
			b.backupList[database] = append(b.backupList[database], tables...)
		}
	}
}

// applyExclude applies the "exclude" filter to remove specific databases/tables
func (b *Backup) applyExclude(section config.SectionConfig) {
	for database, value := range section.Exclude {
		if _, exists := b.backupList[database]; !exists {
			continue
		}

		switch v := value.(type) {
		case []interface{}:
			// Exclude specific tables
			for _, tableInterface := range v {
				if table, ok := tableInterface.(string); ok {
					b.removeTable(database, table)
				}
			}
		case nil:
			// Exclude entire database
			delete(b.backupList, database)
		}
	}
}

// removeTable removes a specific table from the backup list
func (b *Backup) removeTable(database, table string) {
	tables := b.backupList[database]
	for i, t := range tables {
		if t == table {
			b.backupList[database] = append(tables[:i], tables[i+1:]...)
			break
		}
	}
}

// applyAllDatabases adds all databases and tables to the backup list
func (b *Backup) applyAllDatabases(conn *database.Connection) {
	for _, database := range conn.GetDatabases() {
		tables := conn.GetTables(database)
		if len(tables) > 0 {
			if b.backupList[database] == nil {
				b.backupList[database] = make([]string, 0)
			}
			b.backupList[database] = append(b.backupList[database], tables...)
		}
	}
}

// makeBackupCommands generates the backup commands based on mode
func (b *Backup) makeBackupCommands(conn *database.Connection, section config.SectionConfig, destination string) {
	switch section.Backup.Mode {
	case "file_per_table":
		for database, tables := range b.backupList {
			if len(tables) > 0 {
				for _, table := range tables {
					destDir := filepath.Join(destination, database)
					filename := table + ".sql"
					dumpCmd := conn.GetTableDumpCommand(database, table)
					b.commandList = append(b.commandList, Command{
						DumpCommand: dumpCmd,
						Directory:   destDir,
						Filename:    filename,
					})
				}
			}
		}
	case "file_per_database":
		for database := range b.backupList {
			filename := database + ".sql"
			dumpCmd := conn.GetDatabaseDumpCommand(database)
			b.commandList = append(b.commandList, Command{
				DumpCommand: dumpCmd,
				Directory:   destination,
				Filename:    filename,
			})
		}
	}
}

// doBackup executes all backup commands
func (b *Backup) doBackup() {
	if len(b.commandList) == 0 {
		b.logger.Info("No backup commands to execute")
		return
	}

	for _, cmd := range b.commandList {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(cmd.Directory, 0755); err != nil {
			b.logger.Error(fmt.Sprintf("Failed to create directory %s: %v", cmd.Directory, err))
			b.hasError = true
			continue
		}

		fullPath := filepath.Join(cmd.Directory, cmd.Filename)
		b.savedFiles[fullPath] = &report.SavedFile{}

		// Log the encrypted command
		b.logger.Info(fmt.Sprintf("Backup: %s > %s", b.encryptDumpCommand(cmd.DumpCommand), fullPath))

		// Execute mysqldump
		outFile, err := os.Create(fullPath)
		if err != nil {
			b.logger.Error(fmt.Sprintf("Failed to create output file %s: %v", fullPath, err))
			b.hasError = true
			continue
		}

		// Execute command and redirect output to file
		execCmd := exec.Command("sh", "-c", cmd.DumpCommand)
		execCmd.Stdout = outFile
		execCmd.Stderr = os.Stderr

		if err := execCmd.Run(); err != nil {
			outFile.Close()
			b.logger.Error(fmt.Sprintf("Backup failed for %s: %v", fullPath, err))
			b.hasError = true
			continue
		}
		outFile.Close()

		// Check if file was created and get size
		if fileInfo, err := os.Stat(fullPath); err == nil {
			b.savedFiles[fullPath].SuccessfulSave = true
			b.savedFiles[fullPath].SQLSize = fileInfo.Size()
		} else {
			b.hasError = true
		}
	}
}

// doCompression compresses all successfully saved files
func (b *Backup) doCompression(compressionMode string) {
	validModes := map[string]bool{"gz": true, "bz2": true}
	if !validModes[compressionMode] {
		b.logger.Error(fmt.Sprintf("Unknown compression mode: %s", compressionMode))
		return
	}

	for sqlFile, info := range b.savedFiles {
		if !info.SuccessfulSave {
			continue
		}

		archiveFile := fmt.Sprintf("%s.tar.%s", sqlFile, compressionMode)
		b.logger.Info(fmt.Sprintf("Compress: %s => %s", sqlFile, archiveFile))

		if err := b.compress(sqlFile, archiveFile, compressionMode); err != nil {
			b.logger.Error(fmt.Sprintf("Compression failed for %s: %v", sqlFile, err))
			b.savedFiles[sqlFile].SuccessfulCompression = false
			b.hasError = true
			continue
		}

		// Get compressed file size
		if fileInfo, err := os.Stat(archiveFile); err == nil {
			b.savedFiles[sqlFile].SuccessfulCompression = true
			b.savedFiles[sqlFile].CompressedFile = archiveFile
			b.savedFiles[sqlFile].CompressedSize = fileInfo.Size()
		} else {
			b.logger.Error(fmt.Sprintf("Failed to stat compressed file %s: %v", archiveFile, err))
			b.hasError = true
		}
	}
}

// compress creates a compressed tar archive of a file
func (b *Backup) compress(sourceFile, archiveFile, mode string) error {
	// Open source file
	src, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer src.Close()

	// Create archive file
	archive, err := os.Create(archiveFile)
	if err != nil {
		return err
	}
	defer archive.Close()

	// Create compression writer
	var compWriter io.WriteCloser
	switch mode {
	case "gz":
		compWriter = gzip.NewWriter(archive)
	case "bz2":
		// Note: bzip2 in Go standard library only supports reading
		// For production use, consider using a third-party library like github.com/dsnet/compress/bzip2
		return fmt.Errorf("bzip2 compression not fully supported in this implementation")
	default:
		return fmt.Errorf("unsupported compression mode: %s", mode)
	}
	defer compWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(compWriter)
	defer tarWriter.Close()

	// Get file info
	fileInfo, err := src.Stat()
	if err != nil {
		return err
	}

	// Create tar header
	header := &tar.Header{
		Name:    filepath.Base(sourceFile),
		Size:    fileInfo.Size(),
		Mode:    int64(fileInfo.Mode()),
		ModTime: fileInfo.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Copy file content to tar
	if _, err := io.Copy(tarWriter, src); err != nil {
		return err
	}

	// Remove original SQL file after successful compression
	if err := os.Remove(sourceFile); err != nil {
		b.logger.Error(fmt.Sprintf("Failed to remove original file %s: %v", sourceFile, err))
	}

	return nil
}

// encryptDumpCommand masks sensitive information in dump commands
func (b *Backup) encryptDumpCommand(command string) string {
	re1 := regexp.MustCompile(`\s+--password=\S+`)
	re2 := regexp.MustCompile(`\s+--user=\S+`)
	command = re1.ReplaceAllString(command, " --password=*****")
	command = re2.ReplaceAllString(command, " --user=*****")
	return command
}

// printBackupList prints the list of databases and tables to be backed up
func (b *Backup) printBackupList() {
	if len(b.backupList) == 0 {
		b.logger.Info("Backup list is empty")
		return
	}

	b.logger.Info("List of databases and tables to be backed up:")
	for database, tables := range b.backupList {
		b.logger.Info(fmt.Sprintf("Database: %s", database))
		for _, table := range tables {
			b.logger.Info(fmt.Sprintf("  %s.%s", database, table))
		}
	}
}

// printBackupCommands prints the list of backup commands to be executed
func (b *Backup) printBackupCommands() {
	if len(b.commandList) == 0 {
		b.logger.Info("Backup command list is empty")
		return
	}

	b.logger.Info("List of backup commands to be executed:")
	for _, cmd := range b.commandList {
		fullPath := filepath.Join(cmd.Directory, cmd.Filename)
		b.logger.Info(fmt.Sprintf("%s > %s", b.encryptDumpCommand(cmd.DumpCommand), fullPath))
	}
}

// HasError returns true if any errors occurred during backup
func (b *Backup) HasError() bool {
	return b.hasError
}

// GetSavedFiles returns information about all saved files
func (b *Backup) GetSavedFiles() map[string]*report.SavedFile {
	return b.savedFiles
}
