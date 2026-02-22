package database

import (
	"optidump/internal/config"
	"strings"
	"testing"
)

func defaultServerConfig() config.ServerConfig {
	return config.ServerConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "secret",
	}
}

// --- New ---

func TestNew(t *testing.T) {
	conn := New(defaultServerConfig())

	if conn == nil {
		t.Fatal("New() returned nil")
	}
	if conn.config.Host != "localhost" {
		t.Errorf("Host = %q, want %q", conn.config.Host, "localhost")
	}
	if conn.databases == nil {
		t.Error("databases should be initialized")
	}
	if conn.tables == nil {
		t.Error("tables should be initialized")
	}
	if conn.command != "/usr/bin/mysqldump" {
		t.Errorf("command = %q, want %q", conn.command, "/usr/bin/mysqldump")
	}
	if conn.options != "--opt --routines --triggers --events --skip-lock-tables" {
		t.Errorf("options = %q, want default options", conn.options)
	}
}

// --- SetCommand ---

func TestSetCommand(t *testing.T) {
	conn := New(defaultServerConfig())

	conn.SetCommand("/usr/local/bin/mariadb-dump")
	if conn.command != "/usr/local/bin/mariadb-dump" {
		t.Errorf("command = %q, want %q", conn.command, "/usr/local/bin/mariadb-dump")
	}
}

func TestSetCommand_Empty(t *testing.T) {
	conn := New(defaultServerConfig())
	original := conn.command

	conn.SetCommand("")
	if conn.command != original {
		t.Errorf("empty SetCommand should not change command, got %q", conn.command)
	}
}

// --- SetOptions ---

func TestSetOptions(t *testing.T) {
	conn := New(defaultServerConfig())

	conn.SetOptions("--single-transaction --quick")
	if conn.options != "--single-transaction --quick" {
		t.Errorf("options = %q, want %q", conn.options, "--single-transaction --quick")
	}
}

func TestSetOptions_Empty(t *testing.T) {
	conn := New(defaultServerConfig())
	original := conn.options

	conn.SetOptions("")
	if conn.options != original {
		t.Errorf("empty SetOptions should not change options, got %q", conn.options)
	}
}

// --- GetConnectionString ---

func TestGetConnectionString_TCP(t *testing.T) {
	conn := New(config.ServerConfig{
		Host:     "db.example.com",
		Port:     3307,
		User:     "backup",
		Password: "pass123",
	})

	connStr := conn.GetConnectionString()

	if !strings.Contains(connStr, "--host=db.example.com") {
		t.Errorf("missing --host, got: %s", connStr)
	}
	if !strings.Contains(connStr, "--port=3307") {
		t.Errorf("missing --port, got: %s", connStr)
	}
	if !strings.Contains(connStr, "--user=backup") {
		t.Errorf("missing --user, got: %s", connStr)
	}
	if !strings.Contains(connStr, "--password=pass123") {
		t.Errorf("missing --password, got: %s", connStr)
	}
	if strings.Contains(connStr, "--socket") {
		t.Errorf("TCP connection should not have --socket, got: %s", connStr)
	}
}

func TestGetConnectionString_Socket(t *testing.T) {
	conn := New(config.ServerConfig{
		Host:     "localhost",
		Port:     3306,
		Socket:   "/var/run/mysqld/mysqld.sock",
		User:     "root",
		Password: "secret",
	})

	connStr := conn.GetConnectionString()

	if !strings.Contains(connStr, "--socket=/var/run/mysqld/mysqld.sock") {
		t.Errorf("missing --socket, got: %s", connStr)
	}
	if strings.Contains(connStr, "--host") {
		t.Errorf("socket connection should not have --host, got: %s", connStr)
	}
}

func TestGetConnectionString_EmptyHostUsesSocket(t *testing.T) {
	conn := New(config.ServerConfig{
		Host:     "",
		Socket:   "/tmp/mysql.sock",
		User:     "root",
		Password: "secret",
	})

	connStr := conn.GetConnectionString()

	if !strings.Contains(connStr, "--socket=/tmp/mysql.sock") {
		t.Errorf("empty host should use socket, got: %s", connStr)
	}
}

// --- GetTableDumpCommand ---

func TestGetTableDumpCommand(t *testing.T) {
	conn := New(defaultServerConfig())

	cmd := conn.GetTableDumpCommand("mydb", "users")

	if !strings.HasPrefix(cmd, "/usr/bin/mysqldump") {
		t.Errorf("command should start with mysqldump path, got: %s", cmd)
	}
	if !strings.Contains(cmd, "mydb") {
		t.Errorf("missing database name, got: %s", cmd)
	}
	if !strings.Contains(cmd, "users") {
		t.Errorf("missing table name, got: %s", cmd)
	}
	if !strings.Contains(cmd, "--opt") {
		t.Errorf("missing default options, got: %s", cmd)
	}
}

func TestGetTableDumpCommand_CustomCommand(t *testing.T) {
	conn := New(defaultServerConfig())
	conn.SetCommand("/usr/bin/mariadb-dump")

	cmd := conn.GetTableDumpCommand("mydb", "orders")

	if !strings.HasPrefix(cmd, "/usr/bin/mariadb-dump") {
		t.Errorf("should use custom command, got: %s", cmd)
	}
}

// --- GetDatabaseDumpCommand ---

func TestGetDatabaseDumpCommand(t *testing.T) {
	conn := New(defaultServerConfig())

	cmd := conn.GetDatabaseDumpCommand("mydb")

	if !strings.HasPrefix(cmd, "/usr/bin/mysqldump") {
		t.Errorf("command should start with mysqldump path, got: %s", cmd)
	}
	if !strings.Contains(cmd, "mydb") {
		t.Errorf("missing database name, got: %s", cmd)
	}
	// Should NOT contain a table name at the end
	parts := strings.Fields(cmd)
	lastPart := parts[len(parts)-1]
	if lastPart != "mydb" {
		t.Errorf("last argument should be database name, got: %s", lastPart)
	}
}

// --- HasDatabases ---

func TestHasDatabases_Empty(t *testing.T) {
	conn := New(defaultServerConfig())

	if conn.HasDatabases() {
		t.Error("HasDatabases() should be false for new connection")
	}
}

func TestHasDatabases_WithData(t *testing.T) {
	conn := New(defaultServerConfig())
	conn.databases = []string{"mydb"}

	if !conn.HasDatabases() {
		t.Error("HasDatabases() should be true when databases are loaded")
	}
}

// --- GetDatabases / GetTables ---

func TestGetDatabases(t *testing.T) {
	conn := New(defaultServerConfig())
	conn.databases = []string{"db1", "db2"}

	dbs := conn.GetDatabases()
	if len(dbs) != 2 {
		t.Errorf("GetDatabases() returned %d, want 2", len(dbs))
	}
}

func TestGetTables(t *testing.T) {
	conn := New(defaultServerConfig())
	conn.tables["mydb"] = []string{"users", "orders"}

	tables := conn.GetTables("mydb")
	if len(tables) != 2 {
		t.Errorf("GetTables(mydb) returned %d, want 2", len(tables))
	}
}

func TestGetTables_NonexistentDB(t *testing.T) {
	conn := New(defaultServerConfig())

	tables := conn.GetTables("nonexistent")
	if tables != nil {
		t.Errorf("GetTables for nonexistent DB should return nil, got: %v", tables)
	}
}

// --- Close ---

func TestClose_NilDB(t *testing.T) {
	conn := New(defaultServerConfig())

	if err := conn.Close(); err != nil {
		t.Errorf("Close() on nil db should not error, got: %v", err)
	}
}
