package database

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Message struct {
	ID        int
	Role      string
	Content   string
	Timestamp time.Time
}

type Database struct {
	DB *sql.DB
}

func NewDatabase(dataSourceName string) (*Database, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	// データベースの初期化とテーブル作成
	if err = db.Ping(); err != nil {
		return nil, err
	}

	return &Database{DB: db}, nil
}

func (d *Database) Init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_,
	err := d.DB.Exec(schema)
	if err != nil {
		return err
	}
	log.Println("Database schema initialized.")
	return nil
}

func (d *Database) AddMessage(role, content string) error {
	stmt, err := d.DB.Prepare("INSERT INTO messages(role, content) VALUES(?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_,
	err = stmt.Exec(role, content)
	return err
}

func (d *Database) GetMessageHistory(limit int) ([]Message, error) {
	rows, err := d.DB.Query("SELECT id, role, content, timestamp FROM messages ORDER BY timestamp DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.Role, &msg.Content, &msg.Timestamp);
			err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}