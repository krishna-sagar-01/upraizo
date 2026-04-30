package storage

import (
	"fmt"
	"io"
	"strings"
	"server/internal/config"

	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type R2Client struct {
	Client *s3.S3
	Bucket string
	CDN    string
}

func NewR2Client(cfg *config.Config) (*R2Client, error) {
	// Cloudflare R2 endpoint format
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.R2.AccountID)

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(cfg.R2.AccessKey, cfg.R2.SecretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String("auto"), // R2 doesn't use standard regions
		S3ForcePathStyle: aws.Bool(true),
	}

	sess, err := session.NewSession(s3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create R2 session: %w", err)
	}

	return &R2Client{
		Client: s3.New(sess),
		Bucket: cfg.R2.BucketName,
		CDN:    cfg.R2.PublicURL,
	}, nil
}

// Upload places a file into R2 and returns the full public URL
func (r *R2Client) Upload(key string, body []byte, contentType string) (string, error) {
	return r.UploadStream(key, strings.NewReader(string(body)), contentType)
}

// UploadStream uploads a stream to R2
func (r *R2Client) UploadStream(key string, body io.ReadSeeker, contentType string) (string, error) {
	// For S3, if we don't know the size, we might need to use a temporary file or buffer if the reader doesn't support seeking.
	// But Fiber's FormFile provides a reader that often works.
	_, err := r.Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(r.Bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to R2: %w", err)
	}

	return fmt.Sprintf("%s/%s", r.CDN, key), nil
}

// GetPresignedURL generates a signed URL for a client to upload a file directly to R2
func (r *R2Client) GetPresignedURL(key string, contentType string) (string, error) {
	req, _ := r.Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(r.Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})

	urlStr, err := req.Presign(15 * time.Minute) // URL valid for 15 minutes
	if err != nil {
		return "", fmt.Errorf("failed to presign URL: %w", err)
	}

	return urlStr, nil
}

// GetPresignedDownloadURL generates a signed URL for a client to download a private file.
func (r *R2Client) GetPresignedDownloadURL(key string, expires time.Duration) (string, error) {
	req, _ := r.Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(r.Bucket),
		Key:    aws.String(key),
	})

	urlStr, err := req.Presign(expires)
	if err != nil {
		return "", fmt.Errorf("failed to presign download URL: %w", err)
	}

	return urlStr, nil
}

// Delete removes an object from R2 by its public URL or key
func (r *R2Client) Delete(urlOrKey string) error {
	if urlOrKey == "" || urlOrKey == "pending_upload" {
		return nil
	}

	key := urlOrKey
	// If it's a full URL, extract the key
	cdnBase := strings.TrimSuffix(r.CDN, "/")
	if strings.HasPrefix(urlOrKey, cdnBase) {
		key = strings.TrimPrefix(urlOrKey, cdnBase+"/")
	}

	_, err := r.Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(r.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from R2: %w", err)
	}
	return nil
}
