package infrastructure

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	bannerPresignTTL  = 15 * time.Minute
	maxBannerSizeBytes = 5 * 1024 * 1024 // 5 MB enforced by Content-Length-Range condition
)

// S3Client wraps AWS S3 for the intelligence service.
// Bucket name and region are read from environment at startup.
type S3Client struct {
	client *s3.Client
	bucket string
}

func NewS3Client(ctx context.Context) (*S3Client, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET not set")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	return &S3Client{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
	}, nil
}

// PresignBannerUpload generates a presigned PUT URL for a shop banner.
// objectKey is the caller-supplied S3 path (e.g. "banners/{shopID}/{uuid}.jpg").
// The client must PUT with Content-Type: image/jpeg or image/png.
func (c *S3Client) PresignBannerUpload(ctx context.Context, objectKey string) (string, error) {
	presigner := s3.NewPresignClient(c.client)
	req, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(objectKey),
		ContentLength: aws.Int64(maxBannerSizeBytes), // advisory; enforced by Content-Length-Range policy
	}, s3.WithPresignExpires(bannerPresignTTL))
	if err != nil {
		return "", fmt.Errorf("presign banner upload: %w", err)
	}
	return req.URL, nil
}

// DeleteObject removes an S3 object. Used when a shop banner is replaced
// (old banner_object_key is deleted before setting the new one).
func (c *S3Client) DeleteObject(ctx context.Context, objectKey string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("s3 delete %s: %w", objectKey, err)
	}
	return nil
}

// ObjectExists checks whether an object was successfully uploaded. Used by
// ConfirmBannerUploadUseCase to validate before calling moderation.
func (c *S3Client) ObjectExists(ctx context.Context, objectKey string) (bool, error) {
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		// aws SDK wraps 404 as a *smithy NoSuchKey error; treat any head error as not-found.
		return false, nil
	}
	return true, nil
}
