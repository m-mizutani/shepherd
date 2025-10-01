package config_test

import (
	"testing"

	"github.com/m-mizutani/shepherd/pkg/cli/config"
)

func TestLogger_Configure(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr bool
	}{
		{
			name:    "Valid level: debug",
			level:   "debug",
			wantErr: false,
		},
		{
			name:    "Valid level: DEBUG (case insensitive)",
			level:   "DEBUG",
			wantErr: false,
		},
		{
			name:    "Valid level: info",
			level:   "info",
			wantErr: false,
		},
		{
			name:    "Valid level: INFO",
			level:   "INFO",
			wantErr: false,
		},
		{
			name:    "Valid level: warn",
			level:   "warn",
			wantErr: false,
		},
		{
			name:    "Valid level: WARN",
			level:   "WARN",
			wantErr: false,
		},
		{
			name:    "Valid level: error",
			level:   "error",
			wantErr: false,
		},
		{
			name:    "Valid level: ERROR",
			level:   "ERROR",
			wantErr: false,
		},
		{
			name:    "Invalid level: invalid",
			level:   "invalid",
			wantErr: true,
		},
		{
			name:    "Invalid level: empty string",
			level:   "",
			wantErr: true,
		},
		{
			name:    "Invalid level: random",
			level:   "random",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &config.Logger{
				Level: tt.level,
				JSON:  false,
			}

			result, err := logger.Configure()
			if (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("Configure() returned nil logger for valid input")
			}

			if tt.wantErr && err == nil {
				t.Error("Configure() should return error for invalid log level")
			}
		})
	}
}

func TestLogger_Configure_JSONFormat(t *testing.T) {
	tests := []struct {
		name string
		json bool
	}{
		{
			name: "JSON format enabled",
			json: true,
		},
		{
			name: "JSON format disabled",
			json: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &config.Logger{
				Level: "info",
				JSON:  tt.json,
			}

			result, err := logger.Configure()
			if err != nil {
				t.Errorf("Configure() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Error("Configure() returned nil logger")
			}

			// Verify logger can be used
			result.Info("test log message")
		})
	}
}

func TestLogger_Configure_LevelBehavior(t *testing.T) {
	// Test that different log levels actually work
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run("Level: "+level, func(t *testing.T) {
			logger := &config.Logger{
				Level: level,
				JSON:  false,
			}

			result, err := logger.Configure()
			if err != nil {
				t.Fatalf("Configure() unexpected error = %v", err)
			}

			// Test that logger can handle all log levels
			result.Debug("debug message")
			result.Info("info message")
			result.Warn("warn message")
			result.Error("error message")
		})
	}
}

func TestLogger_Flags(t *testing.T) {
	logger := &config.Logger{}
	flags := logger.Flags()

	if len(flags) != 2 {
		t.Errorf("Flags() returned %d flags, want 2", len(flags))
	}

	// Verify flag names
	flagNames := make(map[string]bool)
	for _, flag := range flags {
		switch f := flag.(type) {
		case interface{ Names() []string }:
			names := f.Names()
			if len(names) > 0 {
				flagNames[names[0]] = true
			}
		}
	}

	if !flagNames["log-level"] {
		t.Error("Missing log-level flag")
	}
	if !flagNames["log-json"] {
		t.Error("Missing log-json flag")
	}
}
