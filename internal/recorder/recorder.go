package recorder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/john/chatlog/internal/message"
)

// fileWriter manages a single JSONL file
type fileWriter struct {
	file          *os.File
	writer        *bufio.Writer
	createdAt     time.Time
	bytesWritten  int64
	messageBuffer []message.Message
	platform      string
	channel       string
	filename      string
}

// Recorder handles buffering and writing chat messages to disk
type Recorder struct {
	outputDir       string
	bufferSize      int
	rotateMinutes   int
	rotateMegabytes int64

	currentFiles map[string]*fileWriter // key: "platform_channel"
	mu           sync.Mutex
}

// New creates a new recorder
func New(outputDir string, bufferSize, rotateMinutes, rotateMegabytes int) *Recorder {
	return &Recorder{
		outputDir:       outputDir,
		bufferSize:      bufferSize,
		rotateMinutes:   rotateMinutes,
		rotateMegabytes: int64(rotateMegabytes) * 1024 * 1024,
		currentFiles:    make(map[string]*fileWriter),
	}
}

// Start begins recording messages
func (r *Recorder) Start(ctx context.Context, messageChan <-chan message.Message, fileChan chan<- string) error {
	// Create output directory
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Set up ticker for rotation checks
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case msg := <-messageChan:
			if err := r.recordMessage(msg); err != nil {
				log.Printf("Error recording message: %v", err)
			}

		case <-ticker.C:
			r.checkRotation(fileChan)

		case <-ctx.Done():
			log.Println("Recorder shutting down, flushing buffers...")
			r.flushAll(fileChan)
			return ctx.Err()
		}
	}
}

// recordMessage records a single message
func (r *Recorder) recordMessage(msg message.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s_%s", msg.Platform, msg.Channel)
	fw := r.currentFiles[key]

	// Create new file writer if needed
	if fw == nil {
		var err error
		fw, err = r.createFileWriter(msg.Platform, msg.Channel)
		if err != nil {
			return fmt.Errorf("create file writer: %w", err)
		}
		r.currentFiles[key] = fw
	}

	// Add message to buffer
	fw.messageBuffer = append(fw.messageBuffer, msg)

	// Flush if buffer is full
	if len(fw.messageBuffer) >= r.bufferSize {
		if err := r.flushFileWriter(fw); err != nil {
			return fmt.Errorf("flush buffer: %w", err)
		}
	}

	return nil
}

// createFileWriter creates a new file writer
func (r *Recorder) createFileWriter(platform, channel string) (*fileWriter, error) {
	timestamp := time.Now().UTC().Format("20060102_1504")
	filename := fmt.Sprintf("%s_%s_%s.jsonl", platform, channel, timestamp)
	filepath := filepath.Join(r.outputDir, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}

	log.Printf("Created new log file: %s", filename)

	return &fileWriter{
		file:          file,
		writer:        bufio.NewWriter(file),
		createdAt:     time.Now(),
		bytesWritten:  0,
		messageBuffer: make([]message.Message, 0, r.bufferSize),
		platform:      platform,
		channel:       channel,
		filename:      filename,
	}, nil
}

// flushFileWriter writes buffered messages to disk
func (r *Recorder) flushFileWriter(fw *fileWriter) error {
	for _, msg := range fw.messageBuffer {
		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error marshaling message: %v", err)
			continue
		}

		n, err := fw.writer.Write(data)
		if err != nil {
			return fmt.Errorf("write message: %w", err)
		}
		fw.bytesWritten += int64(n)

		if err := fw.writer.WriteByte('\n'); err != nil {
			return fmt.Errorf("write newline: %w", err)
		}
		fw.bytesWritten += 1
	}

	// Clear buffer
	fw.messageBuffer = fw.messageBuffer[:0]

	// Flush to disk
	return fw.writer.Flush()
}

// checkRotation checks if any files need rotation
func (r *Recorder) checkRotation(fileChan chan<- string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, fw := range r.currentFiles {
		needsRotation := false

		// Check time-based rotation
		if time.Since(fw.createdAt).Minutes() >= float64(r.rotateMinutes) {
			needsRotation = true
			log.Printf("Rotating file %s (time limit)", fw.filename)
		}

		// Check size-based rotation
		if fw.bytesWritten >= r.rotateMegabytes {
			needsRotation = true
			log.Printf("Rotating file %s (size limit)", fw.filename)
		}

		if needsRotation {
			r.rotateFile(key, fw, fileChan)
		}
	}
}

// rotateFile closes current file and creates a new one
func (r *Recorder) rotateFile(key string, fw *fileWriter, fileChan chan<- string) {
	// Flush remaining buffer
	if err := r.flushFileWriter(fw); err != nil {
		log.Printf("Error flushing file writer during rotation: %v", err)
	}

	// Close file
	if err := fw.writer.Flush(); err != nil {
		log.Printf("Error flushing writer during rotation: %v", err)
	}
	if err := fw.file.Close(); err != nil {
		log.Printf("Error closing file during rotation: %v", err)
	}

	// Send filepath to uploader
	filepath := filepath.Join(r.outputDir, fw.filename)
	select {
	case fileChan <- filepath:
		log.Printf("Queued file for upload: %s", fw.filename)
	default:
		log.Printf("Warning: upload queue full, file will be uploaded later: %s", fw.filename)
	}

	// Create new file
	newFw, err := r.createFileWriter(fw.platform, fw.channel)
	if err != nil {
		log.Printf("Error creating new file writer: %v", err)
		delete(r.currentFiles, key)
		return
	}

	r.currentFiles[key] = newFw
}

// flushAll flushes all file writers and closes files
func (r *Recorder) flushAll(fileChan chan<- string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, fw := range r.currentFiles {
		// Flush buffer
		if err := r.flushFileWriter(fw); err != nil {
			log.Printf("Error flushing file writer: %v", err)
		}

		// Close file
		if err := fw.writer.Flush(); err != nil {
			log.Printf("Error flushing writer: %v", err)
		}
		if err := fw.file.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}

		// Send to uploader
		filepath := filepath.Join(r.outputDir, fw.filename)
		select {
		case fileChan <- filepath:
			log.Printf("Queued final file for upload: %s", fw.filename)
		default:
			log.Printf("Warning: upload queue full for final file: %s", fw.filename)
		}

		delete(r.currentFiles, key)
	}

	log.Println("All files flushed and closed")
}
