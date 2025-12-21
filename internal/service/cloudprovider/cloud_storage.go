package cloudprovider

import "context"

type CloudStorageProvider interface {
	UploadFile(ctx context.Context, cloudPath, localPath string) error
	DownloadFile(ctx context.Context, cloudPath, localPath string) error
	ListObjects(ctx context.Context, prefix string) ([]string, error)
	DeleteObject(ctx context.Context, key string) error
	TestConnection(ctx context.Context) error
	EnsureDir(ctx context.Context, path string) error
	GetCloudPath(userID, subPath string) string
}
