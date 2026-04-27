package storage

import (
	"bytes"
	"fmt"
	"strings"
	"server/internal/config"

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
	_, err := r.Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(r.Bucket),
		Key:         aws.String(key),
		Body:        aws.ReadSeekCloser(bytes.NewReader(body)),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to R2: %w", err)
	}

	return fmt.Sprintf("%s/%s", r.CDN, key), nil
}

// Delete removes an object from R2 by its public URL or key
func (r *R2Client) Delete(urlOrKey string) error {
	key := urlOrKey
	// If it's a full URL, extract the key
	if strings.HasPrefix(urlOrKey, r.CDN) {
		key = strings.TrimPrefix(urlOrKey, r.CDN+"/")
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
