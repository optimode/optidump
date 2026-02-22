package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"optidump/internal/config"
)

// Connection handles MySQL database connections and queries
type Connection struct {
	config    config.ServerConfig
	db        *sql.DB
	databases []string
	tables    map[string][]string
	command   string
	options   string
}

// NewConnection creates a new MySQL connection instance
func New(config config.ServerConfig) *Connection {
	return &Connection{
		config:    config,
		databases: make([]string, 0),
		tables:    make(map[string][]string),
		command:   "/usr/bin/mysqldump",
		options:   "--opt --routines --triggers --events --skip-lock-tables",
	}
}

// Connect establishes a connection to the MySQL server
func (m *Connection) Connect() error {
	var dsn string

	if m.config.Socket != "" {
		dsn = fmt.Sprintf("%s:%s@unix(%s)/",
			m.config.User, m.config.Password, m.config.Socket)
	} else {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/",
			m.config.User, m.config.Password, m.config.Host, m.config.Port)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping MySQL: %w", err)
	}

	m.db = db
	return nil
}

// Close closes the database connection
func (m *Connection) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// SetCommand sets the mysqldump command path
func (m *Connection) SetCommand(command string) {
	if command != "" {
		m.command = command
	}
}

// SetOptions sets the mysqldump options
func (m *Connection) SetOptions(options string) {
	if options != "" {
		m.options = options
	}
}

// LoadDatabases retrieves the list of all databases
func (m *Connection) LoadDatabases() error {
	rows, err := m.db.Query("SHOW DATABASES")
	if err != nil {
		return fmt.Errorf("failed to load databases: %w", err)
	}
	defer rows.Close()

	m.databases = make([]string, 0)
	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return err
		}
		m.databases = append(m.databases, database)
	}

	return rows.Err()
}

// LoadTables retrieves all tables for all loaded databases
func (m *Connection) LoadTables() error {
	for _, database := range m.databases {
		tables, err := m.getTablesForDatabase(database)
		if err != nil {
			return err
		}
		m.tables[database] = tables
	}
	return nil
}

// getTablesForDatabase retrieves tables for a specific database
func (m *Connection) getTablesForDatabase(database string) ([]string, error) {
	query := fmt.Sprintf("SHOW TABLES FROM `%s`", database)
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to load tables for database %s: %w", database, err)
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	return tables, rows.Err()
}

// GetDatabases returns the list of loaded databases
func (m *Connection) GetDatabases() []string {
	return m.databases
}

// GetTables returns the tables for a specific database
func (m *Connection) GetTables(database string) []string {
	return m.tables[database]
}

// HasDatabases returns true if databases are loaded
func (m *Connection) HasDatabases() bool {
	return len(m.databases) > 0
}

// GetConnectionString returns the connection string for mysqldump
func (m *Connection) GetConnectionString() string {
	var conn strings.Builder

	if m.config.Host == "" || m.config.Socket != "" {
		conn.WriteString(fmt.Sprintf(" --socket=%s", m.config.Socket))
	} else {
		conn.WriteString(fmt.Sprintf(" --host=%s --port=%d", m.config.Host, m.config.Port))
	}

	conn.WriteString(fmt.Sprintf(" --user=%s --password=%s", m.config.User, m.config.Password))
	return conn.String()
}

// GetTableDumpCommand returns the mysqldump command for a specific table
func (m *Connection) GetTableDumpCommand(database, table string) string {
	return fmt.Sprintf("%s %s %s %s %s",
		m.command, m.GetConnectionString(), m.options, database, table)
}

// GetDatabaseDumpCommand returns the mysqldump command for an entire database
func (m *Connection) GetDatabaseDumpCommand(database string) string {
	return fmt.Sprintf("%s %s %s %s",
		m.command, m.GetConnectionString(), m.options, database)
}
