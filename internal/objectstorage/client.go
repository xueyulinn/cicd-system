package objectstorage

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type objectAPI interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, options minio.MakeBucketOptions) error
	FPutObject(ctx context.Context, bucketName, objectName, filePath string, options minio.PutObjectOptions) (minio.UploadInfo, error)
	FGetObject(ctx context.Context, bucketName, objectName, filePath string, options minio.GetObjectOptions) error
}

type minioClient struct {
	config Config
	client objectAPI
}

func NewMinioClient(config Config) (*minioClient, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	return &minioClient{config: config, client: client}, nil
}

func (c *minioClient) UploadWorkspace(ctx context.Context, objectName, filePath string) error {
	if err := c.ensureBucket(ctx, c.config.WorkspaceBucket); err != nil {
		return err
	}

	_, err := c.client.FPutObject(ctx, c.config.WorkspaceBucket, objectName, filePath, minio.PutObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *minioClient) DownloadWorkspace(ctx context.Context, objectName, filePath string) error {
	if err := c.ensureBucket(ctx, c.config.WorkspaceBucket); err != nil {
		return err
	}

	err := c.client.FGetObject(
		ctx,
		c.config.WorkspaceBucket,
		objectName,
		filePath,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *minioClient) ensureBucket(ctx context.Context, bucket string) error {
	exists, err := c.client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}

	if !exists {
		err := c.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *minioClient) BuildObjectName(commit string) string {
	commit = strings.ToLower(strings.TrimSpace(commit))
	if commit == "" {
		commit = "unknown"
	}

	return fmt.Sprintf(
		"workspaces/pack-v1/commits/%s/workspace.tar.gz",
		commit,
	)
}
