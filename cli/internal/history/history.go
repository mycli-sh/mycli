package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Timestamp  time.Time `json:"timestamp"`
	Slug       string    `json:"slug"`
	CommandID  string    `json:"command_id"`
	Version    int       `json:"version"`
	ExitCode   int       `json:"exit_code"`
	DurationMs int64     `json:"duration_ms"`
}

func historyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".my", "history.jsonl")
}

func Record(entry Entry) error {
	dir := filepath.Dir(historyPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(historyPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func List(n int) ([]Entry, error) {
	f, err := os.Open(historyPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue // skip malformed lines
		}
		all = append(all, e)
	}

	if n <= 0 || n >= len(all) {
		return all, nil
	}
	return all[len(all)-n:], nil
}
