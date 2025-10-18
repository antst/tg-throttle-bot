// Package logging provides structured logging utilities for the bot.
package logging

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.SugaredLogger to provide structured logging
type Logger struct {
	*zap.SugaredLogger
}

// NewLogger creates a new structured logger
func NewLogger(logLevel string) (*Logger, error) {
	var level zapcore.Level
	switch logLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{SugaredLogger: logger.Sugar()}, nil
}

// NewDevelopmentLogger creates a logger for development with human-readable output
func NewDevelopmentLogger() (*Logger, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &Logger{SugaredLogger: logger.Sugar()}, nil
}

// WithContext adds context fields to logger (for future context-based logging)
func (l *Logger) WithContext(_ context.Context) *Logger {
	// Can be extended to extract fields from context (trace ID, user ID, etc.)
	return l
}

// Close flushes any buffered log entries
func (l *Logger) Close() error {
	return l.Sync()
}
