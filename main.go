package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/john/chatlog/internal/config"
	"github.com/john/chatlog/internal/health"
	"github.com/john/chatlog/internal/kick"
	"github.com/john/chatlog/internal/message"
	"github.com/john/chatlog/internal/recorder"
	"github.com/john/chatlog/internal/twitch"
	"github.com/john/chatlog/internal/uploader"
)

func main() {
	log.Println("Chatlog starting...")

	// Get config path from environment variable or use default
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Configuration loaded successfully")

	// Log configured platforms
	if len(cfg.Twitch.Channels) > 0 {
		log.Printf("Monitoring %d Twitch channels: %v", len(cfg.Twitch.Channels), cfg.Twitch.Channels)
	}
	if cfg.Kick.Enabled && len(cfg.Kick.Channels) > 0 {
		log.Printf("Monitoring %d Kick channels: %v", len(cfg.Kick.Channels), cfg.Kick.Channels)
	}

	// Setup context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create communication channels
	messageChan := make(chan message.Message, cfg.Recorder.BufferSize)
	fileChan := make(chan string, 100)

	// Initialize platform connectors
	var twitchConn *twitch.Connector
	if len(cfg.Twitch.Channels) > 0 {
		twitchConn = twitch.New(cfg.Twitch.Username, cfg.Twitch.OAuth, cfg.Twitch.Channels)
	}

	var kickConn *kick.Connector
	if cfg.Kick.Enabled && len(cfg.Kick.Channels) > 0 {
		kickConn = kick.New(cfg.Kick.Channels)
	}

	rec := recorder.New(
		cfg.Recorder.OutputDir,
		cfg.Recorder.BufferSize,
		cfg.Recorder.RotateMinutes,
		cfg.Recorder.RotateMegabytes,
	)

	// Create uploader with appropriate authentication method
	var uploaderInstance *uploader.Uploader
	if cfg.S3.RoleARN != "" {
		// Use OIDC authentication
		log.Printf("Using OIDC authentication with role: %s", cfg.S3.RoleARN)
		uploaderInstance, err = uploader.New(
			ctx,
			cfg.S3.Bucket,
			cfg.S3.Region,
			cfg.S3.RoleARN,
			cfg.Uploader.DeleteAfterUpload,
			cfg.Uploader.MaxRetries,
		)
	} else {
		// Use legacy static credentials (deprecated)
		log.Println("WARNING: Using static AWS credentials (deprecated). Migrate to OIDC for better security.")
		uploaderInstance, err = uploader.NewWithStaticCredentials(
			ctx,
			cfg.S3.Bucket,
			cfg.S3.Region,
			cfg.S3.AccessKeyID,
			cfg.S3.SecretAccessKey,
			cfg.Uploader.DeleteAfterUpload,
			cfg.Uploader.MaxRetries,
		)
	}
	if err != nil {
		log.Fatalf("Failed to create uploader: %v", err)
	}

	// Scan for existing files and queue them for upload
	if err := uploaderInstance.ScanAndUploadExisting(ctx, cfg.Recorder.OutputDir); err != nil {
		log.Printf("Warning: Failed to scan for existing files: %v", err)
	}

	healthServer := health.New(":8080")

	// Start all components
	var wg sync.WaitGroup

	// Start Twitch connector (if configured)
	if twitchConn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := twitchConn.Start(ctx, messageChan); err != nil && err != context.Canceled {
				log.Printf("Twitch connector error: %v", err)
			}
		}()
	}

	// Start Kick connector (if configured)
	if kickConn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := kickConn.Start(ctx, messageChan); err != nil && err != context.Canceled {
				log.Printf("Kick connector error: %v", err)
			}
		}()
	}

	// Start recorder
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := rec.Start(ctx, messageChan, fileChan); err != nil && err != context.Canceled {
			log.Printf("Recorder error: %v", err)
		}
	}()

	// Start uploader
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := uploaderInstance.Start(ctx, fileChan); err != nil && err != context.Canceled {
			log.Printf("Uploader error: %v", err)
		}
	}()

	// Start health check server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := healthServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()

	log.Println("All components started successfully")

	// Wait for shutdown signal
	go func() {
		<-sigChan
		log.Println("Shutdown signal received, initiating graceful shutdown...")

		// Create shutdown context with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		// Stop health server
		if err := healthServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error shutting down health server: %v", err)
		}

		// Cancel main context to stop other components
		cancel()

		// Wait for components to finish with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.Println("All components stopped gracefully")
		case <-shutdownCtx.Done():
			log.Println("Shutdown timeout exceeded, forcing exit")
		}

		os.Exit(0)
	}()

	// Wait for shutdown
	wg.Wait()
	log.Println("Chatlog stopped")
}
