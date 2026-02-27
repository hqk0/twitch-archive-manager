package r2

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appConfig "github.com/hqk0/twitch-archive-manager/internal/config"
)

type R2Client struct {
	S3Client *s3.Client
	Bucket   string
}

func NewR2Client(cfg *appConfig.Config) (*R2Client, error) {
	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.R2AccountID),
		}, nil
	})

	sdkConfig, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(sdkConfig)

	return &R2Client{
		S3Client: s3Client,
		Bucket:   cfg.R2BucketName,
	}, nil
}

func (c *R2Client) DownloadFile(ctx context.Context, key string, outputPath string) error {
	output, err := c.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer output.Body.Close()

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, output.Body)
	return err
}
