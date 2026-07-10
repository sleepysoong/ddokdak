package store

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteChannelStore is an SQLite-backed implementation of ChannelStore.
type SQLiteChannelStore struct {
	db *sql.DB
}

// NewSQLiteChannelStore creates and returns a new SQLiteChannelStore.
func NewSQLiteChannelStore(dbPath string) (*SQLiteChannelStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS channels (
			guild_id TEXT,
			channel_id TEXT,
			PRIMARY KEY (guild_id, channel_id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create channels table: %w", err)
	}

	return &SQLiteChannelStore{db: db}, nil
}

// AddChannel registers a channel for a given guild.
func (s *SQLiteChannelStore) AddChannel(guildID, channelID string) error {
	_, err := s.db.Exec("INSERT INTO channels (guild_id, channel_id) VALUES (?, ?)", guildID, channelID)
	if err != nil {
		return fmt.Errorf("channel %s is already registered in guild %s or insert failed: %w", channelID, guildID, err)
	}
	return nil
}

// RemoveChannel unregisters a channel from a given guild.
func (s *SQLiteChannelStore) RemoveChannel(guildID, channelID string) error {
	res, err := s.db.Exec("DELETE FROM channels WHERE guild_id = ? AND channel_id = ?", guildID, channelID)
	if err != nil {
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("channel %s is not registered in guild %s", channelID, guildID)
	}

	return nil
}

// IsRegistered checks whether a channel is registered for a given guild.
func (s *SQLiteChannelStore) IsRegistered(guildID, channelID string) bool {
	var dummy int
	err := s.db.QueryRow("SELECT 1 FROM channels WHERE guild_id = ? AND channel_id = ?", guildID, channelID).Scan(&dummy)
	return err == nil
}

// GetChannels returns all registered channel IDs for a given guild.
func (s *SQLiteChannelStore) GetChannels(guildID string) []string {
	rows, err := s.db.Query("SELECT channel_id FROM channels WHERE guild_id = ?", guildID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			result = append(result, id)
		}
	}
	return result
}

// Close closes the database connection.
func (s *SQLiteChannelStore) Close() error {
	return s.db.Close()
}
