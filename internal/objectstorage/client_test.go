package objectstorage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestNewMinioClientInvalidConfig(t *testing.T) {
	config := LoadConfig()
	config.AccessKeyID = ""
	_, err := NewMinioClient(config)
	if err == nil || !strings.Contains(err.Error(), "invalid config") {
		t.Fatalf("expected invalid config err got: %v", err)
	}
}

func TestNewMinioClientValidConfig(t *testing.T) {
	config := LoadConfig()
	c, err := NewMinioClient(config)
	if err != nil {
		t.Fatalf("expected created minio client got: %v", err)
	}

	if c.client == nil {
		t.Fatalf("missing minio client")
	}
}

type fakeObjectAPI struct {
	bucketExists bool

	gotBucket       string
	gotObjectName   string
	gotFilePath     string
	gotBucketCtx    context.Context
	gotPutObjectCtx context.Context
	gotGetObjectCtx context.Context

	bucketExistsErr error
	makeBucketErr   error
	putObjectErr    error
	getObjectErr    error

	makeBucketCalled bool
	putObjectCalled  bool
	getObjectCalled  bool
}

func (f *fakeObjectAPI) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	f.gotBucketCtx = ctx
	f.gotBucket = bucketName
	return f.bucketExists, f.bucketExistsErr
}

func (f *fakeObjectAPI) MakeBucket(ctx context.Context, bucketName string, _ minio.MakeBucketOptions) error {
	f.makeBucketCalled = true
	f.gotBucketCtx = ctx
	f.gotBucket = bucketName
	return f.makeBucketErr
}

func (f *fakeObjectAPI) FPutObject(ctx context.Context, bucketName, objectName, filePath string, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	f.putObjectCalled = true
	f.gotPutObjectCtx = ctx
	f.gotBucket = bucketName
	f.gotObjectName = objectName
	f.gotFilePath = filePath
	return minio.UploadInfo{}, f.putObjectErr
}

func (f *fakeObjectAPI) FGetObject(ctx context.Context, bucketName, objectName, filePath string, _ minio.GetObjectOptions) error {
	f.getObjectCalled = true
	f.gotGetObjectCtx = ctx
	f.gotBucket = bucketName
	f.gotObjectName = objectName
	f.gotFilePath = filePath
	return f.getObjectErr
}

func TestEnsureBucketExistingBucket(t *testing.T) {
	ctx := context.Background()
	fake := &fakeObjectAPI{bucketExists: true}
	c := &minioClient{
		config: LoadConfig(),
		client: fake,
	}

	if err := c.ensureBucket(ctx, "test-bucket"); err != nil {
		t.Fatalf("expected existing bucket to pass, got: %v", err)
	}
	if fake.gotBucketCtx != ctx {
		t.Fatalf("expected ensureBucket to pass context through")
	}
	if fake.makeBucketCalled {
		t.Fatalf("did not expect MakeBucket for existing bucket")
	}
}

func TestEnsureBucketMissingBucketCreatesIt(t *testing.T) {
	ctx := context.Background()
	fake := &fakeObjectAPI{bucketExists: false}
	c := &minioClient{
		config: LoadConfig(),
		client: fake,
	}

	if err := c.ensureBucket(ctx, "test-bucket"); err != nil {
		t.Fatalf("expected missing bucket to be created, got: %v", err)
	}
	if !fake.makeBucketCalled {
		t.Fatalf("expected MakeBucket to be called")
	}
}

func TestEnsureBucketBucketExistsError(t *testing.T) {
	rawErr := errors.New("bucket exists failed")
	c := &minioClient{
		config: LoadConfig(),
		client: &fakeObjectAPI{bucketExistsErr: rawErr},
	}

	err := c.ensureBucket(context.Background(), "test-bucket")
	if !errors.Is(err, rawErr) {
		t.Fatalf("expected bucket exists error %v, got %v", rawErr, err)
	}
}

func TestEnsureBucketMakeBucketError(t *testing.T) {
	rawErr := errors.New("make bucket failed")
	c := &minioClient{
		config: LoadConfig(),
		client: &fakeObjectAPI{makeBucketErr: rawErr},
	}

	err := c.ensureBucket(context.Background(), "test-bucket")
	if !errors.Is(err, rawErr) {
		t.Fatalf("expected make bucket error %v, got %v", rawErr, err)
	}
}

func TestUploadWorkspace(t *testing.T) {
	ctx := context.WithValue(context.Background(), "request_id", "req-1")
	fake := &fakeObjectAPI{bucketExists: true}
	c := &minioClient{
		config: Config{WorkspaceBucket: "workspace"},
		client: fake,
	}

	if err := c.UploadWorkspace(ctx, "workspace-unit-test", "test.txt"); err != nil {
		t.Fatalf("expected uploaded object got: %v", err)
	}
	if !fake.putObjectCalled {
		t.Fatalf("expected FPutObject to be called")
	}
	if fake.gotPutObjectCtx != ctx {
		t.Fatalf("expected upload to pass context through")
	}
	if fake.gotBucket != "workspace" {
		t.Fatalf("expected workspace bucket, got: %q", fake.gotBucket)
	}
	if fake.gotObjectName != "workspace-unit-test" {
		t.Fatalf("expected object name %q, got %q", "workspace-unit-test", fake.gotObjectName)
	}
	if fake.gotFilePath != "test.txt" {
		t.Fatalf("expected file path %q, got %q", "test.txt", fake.gotFilePath)
	}
}

func TestUploadWorkspaceEnsureBucketError(t *testing.T) {
	rawErr := errors.New("bucket exists failed")
	c := &minioClient{
		config: Config{WorkspaceBucket: "workspace"},
		client: &fakeObjectAPI{bucketExistsErr: rawErr},
	}

	err := c.UploadWorkspace(context.Background(), "workspace-unit-test", "test.txt")
	if !errors.Is(err, rawErr) {
		t.Fatalf("expected ensure bucket error %v, got %v", rawErr, err)
	}
}

func TestUploadWorkspacePutObjectError(t *testing.T) {
	rawErr := errors.New("put object failed")
	c := &minioClient{
		config: Config{WorkspaceBucket: "workspace"},
		client: &fakeObjectAPI{bucketExists: true, putObjectErr: rawErr},
	}

	err := c.UploadWorkspace(context.Background(), "workspace-unit-test", "test.txt")
	if !errors.Is(err, rawErr) {
		t.Fatalf("expected put object error %v, got %v", rawErr, err)
	}
}

func TestDownloadWorkspace(t *testing.T) {
	ctx := context.WithValue(context.Background(), "request_id", "req-1")
	fake := &fakeObjectAPI{bucketExists: true}
	c := &minioClient{
		config: Config{WorkspaceBucket: "workspace"},
		client: fake,
	}

	if err := c.DownloadWorkspace(ctx, "workspace-unit-test", "download.tar.gz"); err != nil {
		t.Fatalf("expected downloaded object got: %v", err)
	}
	if !fake.getObjectCalled {
		t.Fatalf("expected FGetObject to be called")
	}
	if fake.gotGetObjectCtx != ctx {
		t.Fatalf("expected download to pass context through")
	}
	if fake.gotBucket != "workspace" {
		t.Fatalf("expected workspace bucket, got: %q", fake.gotBucket)
	}
	if fake.gotObjectName != "workspace-unit-test" {
		t.Fatalf("expected object name %q, got %q", "workspace-unit-test", fake.gotObjectName)
	}
	if fake.gotFilePath != "download.tar.gz" {
		t.Fatalf("expected file path %q, got %q", "download.tar.gz", fake.gotFilePath)
	}
}

func TestDownloadWorkspaceGetObjectError(t *testing.T) {
	rawErr := errors.New("get object failed")
	c := &minioClient{
		config: Config{WorkspaceBucket: "workspace"},
		client: &fakeObjectAPI{bucketExists: true, getObjectErr: rawErr},
	}

	err := c.DownloadWorkspace(context.Background(), "workspace-unit-test", "download.tar.gz")
	if !errors.Is(err, rawErr) {
		t.Fatalf("expected get object error %v, got %v", rawErr, err)
	}
}

func TestBuildObjectName(t *testing.T) {
	commit := "ABCDEF0123456789ABCDEF0123456789ABCDEF01"

	want := "workspaces/pack-v1/commits/abcdef0123456789abcdef0123456789abcdef01/workspace.tar.gz"

	c := &minioClient{}
	got := c.BuildObjectName(commit)
	if got != want {
		t.Fatalf("BuildObjectName() = %q, want %q", got, want)
	}
}
