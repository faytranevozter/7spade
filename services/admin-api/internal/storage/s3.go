package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Signer struct {
	client            *s3.Client
	presigner         *s3.PresignClient
	bucket, publicURL string
}

func NewSigner(endpoint, region, bucket, accessKey, secretKey, publicURL string, usePathStyle bool) (*Signer, error) {
	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return nil, nil
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")), awsconfig.WithBaseEndpoint(endpoint))
	if err != nil {
		return nil, fmt.Errorf("storage config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = usePathStyle })
	return &Signer{client: client, presigner: s3.NewPresignClient(client), bucket: bucket, publicURL: strings.TrimRight(publicURL, "/")}, nil
}
func (s *Signer) PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (string, error) {
	result, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(contentType), ContentLength: aws.Int64(size)}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
func (s *Signer) HeadObject(ctx context.Context, key string) (string, int64, error) {
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return "", 0, err
	}
	return aws.ToString(result.ContentType), aws.ToInt64(result.ContentLength), nil
}
func (s *Signer) PublicURL(key string) string {
	if s.publicURL == "" {
		return ""
	}
	return s.publicURL + "/" + key
}
