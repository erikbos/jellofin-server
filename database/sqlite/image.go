package sqlite

import (
	"context"
	"database/sql"

	"github.com/erikbos/jellofin-server/database/model"
)

// HasImage checks if an image exists for the given itemID and type
func (s *SqliteRepo) HasImage(ctx context.Context, itemID, imageType string) (model.ImageMetadata, error) {
	const query = `SELECT mimetype, etag, updated, filesize FROM images WHERE itemid = ? AND type = ? LIMIT 1`
	var metadata model.ImageMetadata
	err := s.dbReadHandle.QueryRowContext(ctx, query, itemID, imageType).Scan(&metadata.MimeType, &metadata.Etag, &metadata.Updated, &metadata.FileSize)
	if err != nil {
		return model.ImageMetadata{}, err
	}
	return metadata, nil
}

// GetImage retrieves image data for the given itemID and type
func (s *SqliteRepo) GetImage(ctx context.Context, itemID, imageType string) (metadata model.ImageMetadata, data []byte, err error) {
	const query = `SELECT mimetype, etag, updated, filesize, data FROM images WHERE itemid = ? AND type = ?`
	err = s.dbReadHandle.QueryRowContext(ctx, query, itemID, imageType).Scan(&metadata.MimeType, &metadata.Etag, &metadata.Updated, &metadata.FileSize, &data)
	if err == sql.ErrNoRows {
		return model.ImageMetadata{}, nil, model.ErrNotFound
	}
	if err != nil {
		return model.ImageMetadata{}, nil, err
	}

	return metadata, data, nil
}

// StoreImage stores image data for the given itemID and type
func (s *SqliteRepo) StoreImage(ctx context.Context, itemID string, imageType string, metadata model.ImageMetadata, data []byte) error {
	const query = `REPLACE INTO images (itemid, type, mimetype, etag, updated, filesize, data) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.dbWriteHandle.ExecContext(ctx, query, itemID, imageType, metadata.MimeType, metadata.Etag, metadata.Updated, metadata.FileSize, data)
	// log.Printf("Stored image for itemID=%s, type=%s, size=%d bytes, err: %v", itemID, imageType, metadata.FileSize, err)
	return err
}

// DeleteImage deletes an image for the given itemID and type
func (s *SqliteRepo) DeleteImage(ctx context.Context, itemID, imageType string) error {
	const query = `DELETE FROM images WHERE itemid = ? AND type = ?`
	_, err := s.dbWriteHandle.ExecContext(ctx, query, itemID, imageType)
	return err
}
