package backup

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"optidump/internal/config"
	"optidump/internal/database"
	"optidump/internal/logger"
	"optidump/internal/report"
)

func testLogger(t *testing.T) *logger.Logger {
	t.Helper()
	log, err := logger.New("", "debug", "text", true)
	if err != nil {
		t.Fatalf("failed to create test logger: %v", err)
	}
	return log
}

// --- New ---

func TestNew(t *testing.T) {
	log := testLogger(t)
	defer log.Close()

	b := New(log)

	if b == nil {
		t.Fatal("New() returned nil")
	}
	if b.backupList == nil {
		t.Error("backupList should be initialized")
	}
	if b.commandList == nil {
		t.Error("commandList should be initialized")
	}
	if b.savedFiles == nil {
		t.Error("savedFiles should be initialized")
	}
	if b.hasError {
		t.Error("hasError should be false initially")
	}
}

// --- HasError / GetSavedFiles ---

func TestHasError_Initial(t *testing.T) {
	log := testLogger(t)
	defer log.Close()

	b := New(log)
	if b.HasError() {
		t.Error("HasError() should be false initially")
	}
}

func TestGetSavedFiles_Empty(t *testing.T) {
	log := testLogger(t)
	defer log.Close()

	b := New(log)
	files := b.GetSavedFiles()
	if len(files) != 0 {
		t.Errorf("GetSavedFiles() should be empty, got %d", len(files))
	}
}

// --- encryptDumpCommand ---

func TestEncryptDumpCommand_MasksPassword(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	cmd := "/usr/bin/mysqldump --host=localhost --port=3306 --user=root --password=supersecret mydb users"
	result := b.encryptDumpCommand(cmd)

	if strings.Contains(result, "supersecret") {
		t.Errorf("password should be masked, got: %s", result)
	}
	if !strings.Contains(result, "--password=*****") {
		t.Errorf("password should be replaced with *****, got: %s", result)
	}
}

func TestEncryptDumpCommand_MasksUser(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	cmd := "/usr/bin/mysqldump --host=localhost --user=admin --password=secret mydb"
	result := b.encryptDumpCommand(cmd)

	if strings.Contains(result, "--user=admin") {
		t.Errorf("user should be masked, got: %s", result)
	}
	if !strings.Contains(result, "--user=*****") {
		t.Errorf("user should be replaced with *****, got: %s", result)
	}
}

func TestEncryptDumpCommand_PreservesOtherParts(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	cmd := "/usr/bin/mysqldump --host=localhost --port=3306 --user=root --password=secret --opt mydb"
	result := b.encryptDumpCommand(cmd)

	if !strings.Contains(result, "--host=localhost") {
		t.Errorf("host should be preserved, got: %s", result)
	}
	if !strings.Contains(result, "--port=3306") {
		t.Errorf("port should be preserved, got: %s", result)
	}
	if !strings.Contains(result, "--opt") {
		t.Errorf("options should be preserved, got: %s", result)
	}
	if !strings.Contains(result, "mydb") {
		t.Errorf("database should be preserved, got: %s", result)
	}
}

func TestEncryptDumpCommand_NoCreds(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	cmd := "/usr/bin/mysqldump --defaults-file=~/.my.cnf mydb"
	result := b.encryptDumpCommand(cmd)

	if result != cmd {
		t.Errorf("command without creds should be unchanged, got: %s", result)
	}
}

// --- removeTable ---

func TestRemoveTable(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	b.backupList["mydb"] = []string{"users", "orders", "products"}
	b.removeTable("mydb", "orders")

	tables := b.backupList["mydb"]
	if len(tables) != 2 {
		t.Errorf("expected 2 tables after removal, got %d", len(tables))
	}
	for _, table := range tables {
		if table == "orders" {
			t.Error("orders should have been removed")
		}
	}
}

func TestRemoveTable_NotFound(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	b.backupList["mydb"] = []string{"users", "orders"}
	b.removeTable("mydb", "nonexistent")

	if len(b.backupList["mydb"]) != 2 {
		t.Error("removing nonexistent table should not change the list")
	}
}

func TestRemoveTable_SingleElement(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	b.backupList["mydb"] = []string{"users"}
	b.removeTable("mydb", "users")

	if len(b.backupList["mydb"]) != 0 {
		t.Error("list should be empty after removing the only element")
	}
}

// --- compress (gz) ---

func TestCompress_Gz(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "test.sql")
	archivePath := filepath.Join(tmpDir, "test.sql.tar.gz")

	content := "CREATE TABLE users (id INT PRIMARY KEY);\nINSERT INTO users VALUES (1);\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := b.compress(srcPath, archivePath, "gz"); err != nil {
		t.Fatalf("compress() error = %v", err)
	}

	// Verify archive exists
	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("archive not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("archive should not be empty")
	}

	// Verify original was removed
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("original file should be removed after compression")
	}

	// Verify archive contents
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archiveFile.Close()

	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatalf("gzip.NewReader error = %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	header, err := tarReader.Next()
	if err != nil {
		t.Fatalf("tar.Next() error = %v", err)
	}

	if header.Name != "test.sql" {
		t.Errorf("tar entry name = %q, want %q", header.Name, "test.sql")
	}

	extracted, err := io.ReadAll(tarReader)
	if err != nil {
		t.Fatal(err)
	}
	if string(extracted) != content {
		t.Errorf("extracted content mismatch")
	}
}

func TestCompress_Bz2_NotSupported(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "test.sql")
	archivePath := filepath.Join(tmpDir, "test.sql.tar.bz2")

	os.WriteFile(srcPath, []byte("data"), 0644)

	err := b.compress(srcPath, archivePath, "bz2")
	if err == nil {
		t.Fatal("bz2 compression should return error (not fully supported)")
	}
}

func TestCompress_UnsupportedMode(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "test.sql")
	archivePath := filepath.Join(tmpDir, "test.sql.tar.xz")

	os.WriteFile(srcPath, []byte("data"), 0644)

	err := b.compress(srcPath, archivePath, "xz")
	if err == nil {
		t.Fatal("unsupported mode should return error")
	}
}

func TestCompress_SourceNotFound(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	err := b.compress("/nonexistent/file.sql", "/tmp/out.tar.gz", "gz")
	if err == nil {
		t.Fatal("compress with nonexistent source should return error")
	}
}

// --- makeBackupCommands ---

func TestMakeBackupCommands_FilePerTable(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	b.backupList["mydb"] = []string{"users", "orders"}

	conn := database.New(config.ServerConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "secret",
	})
	section := config.SectionConfig{
		Backup: config.BackupConfig{
			Mode:        "file_per_table",
			Destination: "/opt/backup",
		},
	}

	b.makeBackupCommands(conn, section, "/opt/backup/2025-01-01")

	if len(b.commandList) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(b.commandList))
	}

	for _, cmd := range b.commandList {
		if !strings.Contains(cmd.Directory, "mydb") {
			t.Errorf("directory should contain database name, got: %s", cmd.Directory)
		}
		if !strings.HasSuffix(cmd.Filename, ".sql") {
			t.Errorf("filename should end with .sql, got: %s", cmd.Filename)
		}
	}
}

func TestMakeBackupCommands_FilePerDatabase(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	b.backupList["mydb"] = []string{"users", "orders"}
	b.backupList["otherdb"] = []string{"items"}

	conn := database.New(config.ServerConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "secret",
	})
	section := config.SectionConfig{
		Backup: config.BackupConfig{
			Mode:        "file_per_database",
			Destination: "/opt/backup",
		},
	}

	b.makeBackupCommands(conn, section, "/opt/backup/2025-01-01")

	if len(b.commandList) != 2 {
		t.Fatalf("expected 2 commands (one per database), got %d", len(b.commandList))
	}

	for _, cmd := range b.commandList {
		if cmd.Directory != "/opt/backup/2025-01-01" {
			t.Errorf("directory should be the destination root, got: %s", cmd.Directory)
		}
	}
}

// --- doCompression ---

func TestDoCompression_SkipsFailedSaves(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	b.savedFiles["/tmp/failed.sql"] = &report.SavedFile{
		SuccessfulSave: false,
	}

	// Should not panic or error on failed saves
	b.doCompression("gz")
}

func TestDoCompression_InvalidMode(t *testing.T) {
	log := testLogger(t)
	defer log.Close()
	b := New(log)

	b.savedFiles["/tmp/test.sql"] = &report.SavedFile{
		SuccessfulSave: true,
	}

	// Should log error but not panic
	b.doCompression("zip")
}
