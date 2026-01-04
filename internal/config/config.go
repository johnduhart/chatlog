package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Twitch   TwitchConfig   `yaml:"twitch"`
	S3       S3Config       `yaml:"s3"`
	Recorder RecorderConfig `yaml:"recorder"`
	Uploader UploaderConfig `yaml:"uploader"`
}

// TwitchConfig holds Twitch-specific configuration
type TwitchConfig struct {
	Username string   `yaml:"username"`
	OAuth    string   `yaml:"oauth"`
	Channels []string `yaml:"channels"`
}

// S3Config holds S3 upload configuration
type S3Config struct {
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	RoleARN         string `yaml:"role_arn"`          // IAM role ARN for OIDC authentication
	AccessKeyID     string `yaml:"access_key_id"`     // Legacy: static credentials
	SecretAccessKey string `yaml:"secret_access_key"` // Legacy: static credentials
	Endpoint        string `yaml:"endpoint"`          // For S3-compatible services
}

// RecorderConfig holds recorder configuration
type RecorderConfig struct {
	OutputDir        string `yaml:"output_dir"`
	RotateMinutes    int    `yaml:"rotate_minutes"`
	RotateMegabytes  int    `yaml:"rotate_megabytes"`
	BufferSize       int    `yaml:"buffer_size"`
}

// UploaderConfig holds uploader configuration
type UploaderConfig struct {
	CheckIntervalSeconds int  `yaml:"check_interval_seconds"`
	DeleteAfterUpload    bool `yaml:"delete_after_upload"`
	MaxRetries           int  `yaml:"max_retries"`
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	// Read YAML file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Apply environment variable overrides
	if oauth := os.Getenv("TWITCH_OAUTH"); oauth != "" {
		cfg.Twitch.OAuth = oauth
	}
	if roleARN := os.Getenv("AWS_ROLE_ARN"); roleARN != "" {
		cfg.S3.RoleARN = roleARN
	}
	if keyID := os.Getenv("S3_ACCESS_KEY_ID"); keyID != "" {
		cfg.S3.AccessKeyID = keyID
	}
	if secretKey := os.Getenv("S3_SECRET_ACCESS_KEY"); secretKey != "" {
		cfg.S3.SecretAccessKey = secretKey
	}

	// Set defaults
	if cfg.Recorder.BufferSize == 0 {
		cfg.Recorder.BufferSize = 100
	}
	if cfg.Recorder.RotateMinutes == 0 {
		cfg.Recorder.RotateMinutes = 60
	}
	if cfg.Recorder.RotateMegabytes == 0 {
		cfg.Recorder.RotateMegabytes = 100
	}
	if cfg.Recorder.OutputDir == "" {
		cfg.Recorder.OutputDir = "./data"
	}
	if cfg.Uploader.CheckIntervalSeconds == 0 {
		cfg.Uploader.CheckIntervalSeconds = 60
	}
	if cfg.Uploader.MaxRetries == 0 {
		cfg.Uploader.MaxRetries = 3
	}
	// DeleteAfterUpload defaults to true if not explicitly set to false
	// (YAML zero value for bool is false, so we can't detect if it was intentionally set)

	// Validate required fields
	if cfg.Twitch.Username == "" {
		return nil, fmt.Errorf("twitch.username is required")
	}
	if cfg.Twitch.OAuth == "" {
		return nil, fmt.Errorf("twitch.oauth is required (or set TWITCH_OAUTH env var)")
	}
	if len(cfg.Twitch.Channels) == 0 {
		return nil, fmt.Errorf("at least one twitch channel is required")
	}
	if cfg.S3.Bucket == "" {
		return nil, fmt.Errorf("s3.bucket is required")
	}
	if cfg.S3.Region == "" {
		return nil, fmt.Errorf("s3.region is required")
	}
	// Either OIDC role or static credentials required
	if cfg.S3.RoleARN == "" && cfg.S3.AccessKeyID == "" {
		return nil, fmt.Errorf("either s3.role_arn (OIDC) or s3.access_key_id (legacy) is required")
	}
	// If using static credentials, both key and secret are required
	if cfg.S3.AccessKeyID != "" && cfg.S3.SecretAccessKey == "" {
		return nil, fmt.Errorf("s3.secret_access_key is required when using access_key_id")
	}

	return &cfg, nil
}
