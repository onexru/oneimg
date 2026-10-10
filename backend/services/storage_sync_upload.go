package services

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/storage"
	"oneimg/backend/utils/securestorage"
	storageSettings "oneimg/backend/utils/settings"
)

func uploadStorageArtifact(ctx context.Context, bucket models.Buckets, a localStorageArtifact) (map[string]any, error) {
	metadata := map[string]any{}
	if bucket.Type == "default" {
		return metadata, nil
	}
	setting, err := storageSettings.GetSettings()
	if err != nil {
		return nil, err
	}
	target, err := storage.New(setting, bucket)
	if err != nil {
		return nil, err
	}
	source := storage.NewLocal(config.UploadRoot())
	for _, thumb := range []bool{false, true} {
		key, size, contentType, filename := a.URL, a.FileSize, a.MimeType, a.FileName
		if thumb {
			key, size, contentType, filename = a.Thumbnail, a.ThumbnailSize, "image/webp", "thumbnail_"+a.FileName
		}
		if key == "" {
			continue
		}
		file, err := source.Get(ctx, storage.Ref{Key: key})
		if err != nil {
			return metadata, err
		}
		reader := bufio.NewReader(file)
		prefix, err := reader.Peek(64)
		if err != nil && !errors.Is(err, io.EOF) {
			file.Close()
			return metadata, err
		}
		if securestorage.IsEncrypted(prefix) {
			contentType = "application/octet-stream"
		}
		info, err := target.Upload(ctx, storage.Ref{Key: key}, reader, storage.UploadOptions{Size: size, ContentType: contentType, FileName: filename})
		closeErr := file.Close()
		storage.MergeArtifactMetadata(metadata, info, thumb)
		if err != nil {
			return metadata, fmt.Errorf("upload artifact (thumbnail=%t): %w", thumb, err)
		}
		if closeErr != nil {
			return metadata, closeErr
		}
	}
	return metadata, nil
}
