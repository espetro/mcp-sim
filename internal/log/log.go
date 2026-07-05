package log

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// New returns a configured slog.Logger.
// level: debug|info|warn|error (default info)
// format: text|json (default text)
func New(level, format string) *slog.Logger {
	var handler slog.Handler
	var opts slog.HandlerOptions

	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, &opts)
	default:
		handler = slog.NewTextHandler(os.Stderr, &opts)
	}

	logger := slog.New(handler)
	switch level {
	case "debug":
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case "warn":
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	case "error":
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	default:
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	return logger
}

// LoggerFromContext returns the logger stored in context, or a no-op logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(logKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// WithLogger returns a context that carries the given logger.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, logKey{}, l)
}

type logKey struct{}
