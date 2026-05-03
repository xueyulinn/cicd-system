package objectstorage

import "context"

type Storage interface {
	UploadWorkspace(ctx context.Context, objectName, filePath string) error
	DownloadWorkspace(ctx context.Context, objectName, filePath string) error
}
