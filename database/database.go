package database

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

type Message struct {
	ID        int64
	UserID    int64
	Username  string
	Text      string
	Role      string // "user" or "assistant"
	Timestamp time.Time
}

type User struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
	CreatedAt time.Time
}

func New(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	database := &Database{db: db}
	if err := database.init(); err != nil {
		return nil, err
	}

	return database, nil
}

func (d *Database) init() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			username TEXT,
			first_name TEXT,
			last_name TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			username TEXT,
			text TEXT,
			role TEXT DEFAULT 'user',
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users (id)
		)`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

func (d *Database) AddUser(userID int64, username, firstName, lastName string) error {
	query := `INSERT OR REPLACE INTO users (id, username, first_name, last_name) 
			  VALUES (?, ?, ?, ?)`
	_, err := d.db.Exec(query, userID, username, firstName, lastName)
	return err
}

func (d *Database) SaveMessage(userID int64, username, text, role string) error {
	query := `INSERT INTO messages (user_id, username, text, role) VALUES (?, ?, ?, ?)`
	_, err := d.db.Exec(query, userID, username, text, role)
	return err
}

func (d *Database) GetChatHistory(userID int64, limit int) ([]Message, error) {
	query := `SELECT id, user_id, username, text, role, timestamp 
			  FROM messages 
			  WHERE user_id = ? 
			  ORDER BY timestamp DESC 
			  LIMIT ?`

	rows, err := d.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.ID, &msg.UserID, &msg.Username, &msg.Text, &msg.Role, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	// Reverse to get chronological order (oldest first)
	for i := 0; i < len(messages)/2; i++ {
		j := len(messages) - 1 - i
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (d *Database) GetDailyStats() (int, error) {
	query := `SELECT COUNT(*) FROM messages 
			  WHERE DATE(timestamp) = DATE('now')`
	var count int
	err := d.db.QueryRow(query).Scan(&count)
	return count, err
}

func (d *Database) Close() error {
	return d.db.Close()
}
