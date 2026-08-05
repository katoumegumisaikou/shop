// Package logger provides the application's structured logger.
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a logger for the current environment.
// Production logs are JSON for log collectors; other environments remain readable in a terminal.
func New(env, level string) (*zap.Logger, error) {
	var cfg zap.Config
	if env == "prod" || env == "production" {
		cfg = zap.NewProductionConfig()
		cfg.Encoding = "json"
		cfg.OutputPaths = []string{"stdout", "/var/log/shop/api.log"}
		cfg.ErrorOutputPaths = []string{"stderr"}
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.OutputPaths = []string{"stdout"}
		cfg.ErrorOutputPaths = []string{"stderr"}
	}

	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if level != "" {
		if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
			return nil, err
		}
	}

	return cfg.Build()
}

// Sync flushes buffered log entries. It is intended for deferred shutdown cleanup.
func Sync(log *zap.Logger) {
	if log != nil {
		_ = log.Sync()
	}
}
