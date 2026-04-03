package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type nopCloser struct{}

func (nopCloser) Close() error {
	return nil
}

func New(levelName, filePath string) (*slog.Logger, io.Closer, error) {
	level, err := parseLevel(levelName)
	if err != nil {
		return nil, nil, err
	}

	var writer io.Writer = os.Stderr
	var closer io.Closer = nopCloser{}

	if filePath != "" {
		file, openErr := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if openErr != nil {
			return nil, nil, fmt.Errorf("open log file: %w", openErr)
		}

		writer = file
		closer = file
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level})
	return slog.New(handler), closer, nil
}

func parseLevel(levelName string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(levelName)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "off":
		return slog.Level(100), nil
	default:
		return 0, fmt.Errorf("unknown log level %q", levelName)
	}
}
