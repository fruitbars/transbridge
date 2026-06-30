package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

var bboltBucket = []byte("translations")

type bboltItem struct {
	Data     string `json:"data"`
	ExpireAt int64  `json:"expire_at,omitempty"`
}

// BboltCache 实现本地持久化缓存。
type BboltCache struct {
	db         *bbolt.DB
	defaultTTL time.Duration
	permanent  bool
}

func NewBboltCache(opts BboltCacheOptions) (*BboltCache, error) {
	if opts.Path == "" {
		opts.Path = "data/transbridge_cache.db"
	}
	if opts.DefaultTTL <= 0 && !opts.Permanent {
		opts.DefaultTTL = 24 * time.Hour
	}

	dir := filepath.Dir(opts.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create bbolt cache directory: %w", err)
		}
	}

	db, err := bbolt.Open(opts.Path, 0600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt cache: %w", err)
	}

	c := &BboltCache{
		db:         db,
		defaultTTL: opts.DefaultTTL,
		permanent:  opts.Permanent,
	}

	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bboltBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize bbolt cache bucket: %w", err)
	}

	return c, nil
}

func (c *BboltCache) Get(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var item bboltItem
	if err := c.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bboltBucket)
		if bucket == nil {
			return ErrCacheMiss
		}
		data := bucket.Get([]byte(key))
		if data == nil {
			return ErrCacheMiss
		}
		return json.Unmarshal(data, &item)
	}); err != nil {
		if errors.Is(err, ErrCacheMiss) {
			return "", ErrCacheMiss
		}
		return "", err
	}

	if item.ExpireAt > 0 && item.ExpireAt <= time.Now().UnixNano() {
		_ = c.db.Update(func(tx *bbolt.Tx) error {
			bucket := tx.Bucket(bboltBucket)
			if bucket == nil {
				return nil
			}
			return bucket.Delete([]byte(key))
		})
		return "", ErrCacheMiss
	}

	return item.Data, nil
}

func (c *BboltCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	item := bboltItem{Data: value}
	if ttl < 0 || c.permanent {
		item.ExpireAt = 0
	} else if ttl == 0 {
		if c.defaultTTL > 0 {
			item.ExpireAt = time.Now().Add(c.defaultTTL).UnixNano()
		}
	} else {
		item.ExpireAt = time.Now().Add(ttl).UnixNano()
	}

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	return c.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bboltBucket)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(key), data)
	})
}

func (c *BboltCache) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return c.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket(bboltBucket); err != nil && !errors.Is(err, bbolt.ErrBucketNotFound) {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(bboltBucket)
		return err
	})
}

func (c *BboltCache) Close(ctx context.Context) error {
	return c.db.Close()
}
