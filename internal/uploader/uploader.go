package uploader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Uploader handles uploading completed log files to S3
type Uploader struct {
	s3Client     *s3.Client
	bucket       string
	deleteAfter  bool
	maxRetries   int
}

// flyTokenRetriever implements stscreds.IdentityTokenRetriever for Fly.io OIDC
type flyTokenRetriever struct {
	socketPath string
	audience   string
}

// GetIdentityToken fetches an OIDC token from Fly.io's Unix socket API
func (f *flyTokenRetriever) GetIdentityToken() ([]byte, error) {
	// Create HTTP client with Unix socket transport
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", f.socketPath)
			},
		},
		Timeout: 5 * time.Second,
	}

	// Prepare request body
	reqBody, err := json.Marshal(map[string]string{
		"aud": f.audience,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Make POST request to Fly.io API
	resp, err := client.Post("http://localhost/v1/tokens/oidc", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read and return token
	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token: %w", err)
	}

	return token, nil
}

// New creates a new S3 uploader using OIDC authentication
func New(ctx context.Context, bucket, region, roleARN string, deleteAfter bool, maxRetries int) (*Uploader, error) {
	// Load default AWS config
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// If roleARN is provided, assume role using OIDC credentials
	if roleARN != "" {
		// Create STS client
		stsClient := sts.NewFromConfig(cfg)

		// Create Fly.io token retriever
		tokenRetriever := &flyTokenRetriever{
			socketPath: "/.fly/api",
			audience:   "sts.amazonaws.com",
		}

		// Create credentials provider that assumes role with web identity
		credProvider := stscreds.NewWebIdentityRoleProvider(
			stsClient,
			roleARN,
			tokenRetriever,
		)

		// Update config with new credentials
		cfg.Credentials = aws.NewCredentialsCache(credProvider)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(cfg)

	return &Uploader{
		s3Client:    s3Client,
		bucket:      bucket,
		deleteAfter: deleteAfter,
		maxRetries:  maxRetries,
	}, nil
}

// NewWithStaticCredentials creates a new S3 uploader using static credentials (legacy)
func NewWithStaticCredentials(ctx context.Context, bucket, region, accessKeyID, secretAccessKey string, deleteAfter bool, maxRetries int) (*Uploader, error) {
	// Create credentials provider
	credProvider := credentials.NewStaticCredentialsProvider(
		accessKeyID,
		secretAccessKey,
		"",
	)

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(cfg)

	return &Uploader{
		s3Client:    s3Client,
		bucket:      bucket,
		deleteAfter: deleteAfter,
		maxRetries:  maxRetries,
	}, nil
}

// ScanAndUploadExisting scans a directory for existing .jsonl files and uploads them
func (u *Uploader) ScanAndUploadExisting(ctx context.Context, outputDir string) error {
	log.Printf("Scanning %s for existing files to upload...", outputDir)

	// Read directory
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	// Find all .jsonl files
	var filesToUpload []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .jsonl files
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			filePath := filepath.Join(outputDir, entry.Name())
			filesToUpload = append(filesToUpload, filePath)
		}
	}

	if len(filesToUpload) == 0 {
		log.Println("No existing files found to upload")
		return nil
	}

	log.Printf("Found %d existing file(s) to upload", len(filesToUpload))

	// Upload each file in a goroutine
	for _, filePath := range filesToUpload {
		go u.uploadWithRetry(ctx, filePath)
	}

	return nil
}

// Start begins monitoring for files to upload
func (u *Uploader) Start(ctx context.Context, fileChan <-chan string) error {
	for {
		select {
		case localPath := <-fileChan:
			// Upload in a goroutine so we don't block
			go u.uploadWithRetry(ctx, localPath)

		case <-ctx.Done():
			log.Println("Uploader shutting down...")
			return ctx.Err()
		}
	}
}

// uploadWithRetry uploads a file with retry logic
func (u *Uploader) uploadWithRetry(ctx context.Context, localPath string) {
	filename := filepath.Base(localPath)

	s3Key, err := generateS3Key(filename)
	if err != nil {
		log.Printf("Error generating S3 key for %s: %v", filename, err)
		return
	}

	for attempt := 0; attempt <= u.maxRetries; attempt++ {
		err := u.uploadFile(ctx, localPath, s3Key)
		if err == nil {
			log.Printf("Successfully uploaded %s to s3://%s/%s", filename, u.bucket, s3Key)

			// Delete local file if configured
			if u.deleteAfter {
				if err := os.Remove(localPath); err != nil {
					log.Printf("Error deleting local file %s: %v", localPath, err)
				} else {
					log.Printf("Deleted local file %s", localPath)
				}
			}
			return
		}

		if attempt < u.maxRetries {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			log.Printf("Upload attempt %d/%d failed for %s: %v. Retrying in %v",
				attempt+1, u.maxRetries, filename, err, backoff)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
		}
	}

	log.Printf("Failed to upload %s after %d attempts", filename, u.maxRetries)
}

// uploadFile uploads a specific file to S3
func (u *Uploader) uploadFile(ctx context.Context, localPath, s3Key string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	_, err = u.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(s3Key),
		Body:   file,
	})

	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}

	return nil
}

// generateS3Key generates an S3 key from a filename
// Input: twitch_ludwig_20251230_1030.jsonl
// Output: 2025/12/30/twitch/ludwig/twitch_ludwig_20251230_1030.jsonl
func generateS3Key(filename string) (string, error) {
	// Remove extension for parsing
	nameWithoutExt := strings.TrimSuffix(filename, ".jsonl")

	// Parse filename: platform_channel_YYYYMMDD_HHMM
	// Channel names may contain underscores, so parse from the end
	parts := strings.Split(nameWithoutExt, "_")
	if len(parts) < 4 {
		return "", fmt.Errorf("invalid filename format: %s", filename)
	}

	platform := parts[0]
	// The last two parts are always date and time
	dateStr := parts[len(parts)-2]  // YYYYMMDD
	timeStr := parts[len(parts)-1]  // HHMM
	// Everything in between is the channel name
	channel := strings.Join(parts[1:len(parts)-2], "_")

	// Parse date
	timestamp := dateStr + "_" + timeStr
	t, err := time.Parse("20060102_1504", timestamp)
	if err != nil {
		return "", fmt.Errorf("parse timestamp: %w", err)
	}

	// Generate S3 key
	s3Key := fmt.Sprintf("%04d/%02d/%02d/%s/%s/%s",
		t.Year(), t.Month(), t.Day(), platform, channel, filename)

	return s3Key, nil
}
