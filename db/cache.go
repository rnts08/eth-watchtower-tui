package db

import (
	"time"
)

// EnsureCacheTable creates the cache table if it doesn't exist.
func (d *DB) EnsureCacheTable() error {
	query := `CREATE TABLE IF NOT EXISTS cache (
		key TEXT PRIMARY KEY,
		value TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := d.conn.Exec(query)
	return err
}

// SaveCacheEntry saves a single cache entry to the database.
func (d *DB) SaveCacheEntry(key string, value []byte) error {
	query := `INSERT INTO cache (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
			  ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP;`
	_, err := d.conn.Exec(query, key, string(value))
	return err
}

// LoadCache loads all cache entries from the database.
func (d *DB) LoadCache() (map[string][]byte, error) {
	rows, err := d.conn.Query("SELECT key, value FROM cache")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]byte)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		result[key] = []byte(value)
	}
	return result, nil
}

// ClearCache removes all entries from the cache table.
func (d *DB) ClearCache() error {
	_, err := d.conn.Exec("DELETE FROM cache")
	return err
}

// PruneCache removes cache entries older than the specified duration.
func (d *DB) PruneCache(maxAge time.Duration) error {
	cutoff := time.Now().UTC().Add(-maxAge)
	// SQLite default CURRENT_TIMESTAMP format is "2006-01-02 15:04:05"
	cutoffStr := cutoff.Format("2006-01-02 15:04:05")
	_, err := d.conn.Exec("DELETE FROM cache WHERE updated_at < ?", cutoffStr)
	return err
}
