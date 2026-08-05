package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/faytranevozter/7spade/services/api/internal/config"
)

type S3Client struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

func New(cfg config.S3Config) (*S3Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("storage: S3_ENDPOINT is required")
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("storage: S3_ACCESS_KEY_ID is required")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("storage: S3_SECRET_ACCESS_KEY is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("storage: S3_BUCKET is required")
	}

	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               cfg.Endpoint,
			SigningRegion:     cfg.Region,
			HostnameImmutable: true,
		}, nil
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, "")),
		awsconfig.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &S3Client{
		client:    client,
		bucket:    cfg.Bucket,
		publicURL: cfg.PublicURL,
	}, nil
}

func (c *S3Client) PublicURL(key string) string {
	if c.publicURL == "" {
		return ""
	}
	base := c.publicURL
	if base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	return fmt.Sprintf("%s/%s", base, key)
}

func (c *S3Client) HealthCheck(ctx context.Context) error {
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.bucket),
	})
	if err != nil {
		return fmt.Errorf("storage: health check failed: %w", err)
	}
	return nil
}

func (c *S3Client) Bucket() string {
	return c.bucket
}

func (c *S3Client) PutObject(ctx context.Context, key, contentType, cacheControl string, body io.Reader) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(c.bucket),
		Key:          aws.String(key),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String(cacheControl),
	})
	if err != nil {
		return fmt.Errorf("storage: put object %q: %w", key, err)
	}
	return nil
}

func (c *S3Client) ConfigurePublicReadCORS(ctx context.Context, origins []string) error {
	_, err := c.client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(c.bucket),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: []types.CORSRule{{
				AllowedMethods: []string{"GET", "HEAD"},
				AllowedOrigins: origins,
				AllowedHeaders: []string{"*"},
				MaxAgeSeconds:  aws.Int32(86400),
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("storage: configure bucket CORS: %w", err)
	}
	return nil
}

func (c *S3Client) PresignPut(ctx context.Context, key string, contentType string, ttl time.Duration) (*url.URL, error) {
	presignClient := s3.NewPresignClient(c.client)
	req, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return nil, fmt.Errorf("storage: presign put failed: %w", err)
	}
	url, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("storage: parse presigned url: %w", err)
	}
	return url, nil
}
