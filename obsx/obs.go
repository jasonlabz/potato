package obsx

import (
	"archive/zip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/jasonlabz/potato/configx"
	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
)

// ---- constants ----

const (
	DefaultMaxRetries       = 3
	DefaultTimeout          = 30 * time.Second
	Closed            int32 = 1

	// DirSuffix is the key suffix for directory markers in object storage.
	DirSuffix = "/"
)

// ---- sentinel errors ----

var (
	ErrConfigIsNil      = errors.New("obs config is nil")
	ErrEndpointIsEmpty  = errors.New("obs endpoint is empty")
	ErrAccessKeyIsEmpty = errors.New("obs access_key is empty")
	ErrSecretKeyIsEmpty = errors.New("obs secret_key is empty")
	ErrClientNotFound   = errors.New("obs client not found")
	ErrOperatorClosed   = errors.New("obs operator is closed")
	ErrBucketEmpty      = errors.New("obs bucket is empty")
)

// ---- global operator ----

var operator *Operator

func init() {
	appConf := configx.GetConfig()
	if !appConf.OBS.Enable {
		return
	}
	cfg := &Config{
		Name:               appConf.OBS.Name,
		Endpoint:           appConf.OBS.Endpoint,
		Region:             appConf.OBS.Region,
		AccessKey:          appConf.OBS.AccessKey,
		SecretKey:          appConf.OBS.SecretKey,
		Bucket:             appConf.OBS.Bucket,
		InsecureSkipVerify: appConf.OBS.InsecureSkipVerify,
		MaxRetries:         appConf.OBS.MaxRetries,
		Timeout:            appConf.OBS.Timeout,
	}
	if cfg.Name == "" {
		cfg.Name = "default"
	}
	op, err := NewOperator(cfg)
	if err == nil {
		operator = op
		return
	}
	zapx.GetLogger().WithError(err).Error(context.Background(), "init obs operator error")
	if appConf.OBS.Strict {
		panic(fmt.Errorf("init obs operator error: %v", err))
	}
}

// GetOperator returns the global OBS operator instance.
func GetOperator() *Operator {
	return operator
}

// ---- Config ----

// Config holds connection configuration for an object storage endpoint.
type Config struct {
	Name               string `json:"name" yaml:"name"`
	Endpoint           string `json:"endpoint" yaml:"endpoint"`
	Region             string `json:"region" yaml:"region"`
	AccessKey          string `json:"access_key" yaml:"access_key"`
	SecretKey          string `json:"secret_key" yaml:"secret_key"`
	Bucket             string `json:"bucket" yaml:"bucket"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	MaxRetries         int    `json:"max_retries" yaml:"max_retries"`
	Timeout            int64  `json:"timeout" yaml:"timeout"` // seconds
}

// Validate fills defaults and checks required fields.
func (c *Config) Validate() error {
	if c == nil {
		return ErrConfigIsNil
	}
	if c.Endpoint == "" {
		return ErrEndpointIsEmpty
	}
	if c.AccessKey == "" {
		return ErrAccessKeyIsEmpty
	}
	if c.SecretKey == "" {
		return ErrSecretKeyIsEmpty
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = DefaultMaxRetries
	}
	if c.Timeout <= 0 {
		c.Timeout = int64(DefaultTimeout.Seconds())
	}
	if c.Name == "" {
		c.Name = c.Endpoint
	}
	return nil
}

// ---- data types ----

// ObjectInfo holds metadata about an object.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
	StorageClass string
	IsDir        bool
}

// BucketInfo holds bucket metadata.
type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

// ListOptions controls ListObjects behavior.
type ListOptions struct {
	Prefix     string
	Recursive  bool
	MaxKeys    int
	StartAfter string
}

// ---- Client interface (low-level, mirrors SDK capabilities) ----

// Client is the low-level object storage client interface.
type Client interface {
	BucketExists(ctx context.Context, bucket string) (bool, error)
	MakeBucket(ctx context.Context, bucket, region string) error
	RemoveBucket(ctx context.Context, bucket string) error
	ListBuckets(ctx context.Context) ([]BucketInfo, error)

	PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error)
	RemoveObject(ctx context.Context, bucket, key string) error
	RemoveObjects(ctx context.Context, bucket string, keys ...string) <-chan minio.RemoveObjectError
	CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error
	StatObject(ctx context.Context, bucket, key string) (*ObjectInfo, error)

	ListObjects(ctx context.Context, bucket string, opts ListOptions) <-chan ObjectInfo

	PresignedGetObject(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	PresignedPutObject(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)

	Close() error
}

// ---- minioClient ----

type minioClient struct {
	client *minio.Client
	config *Config
}

func newMinioClient(cfg *Config) (*minioClient, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify},
	}
	c, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Region:    cfg.Region,
		Secure:    strings.HasPrefix(cfg.Endpoint, "https"),
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &minioClient{client: c, config: cfg}, nil
}

func (c *minioClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	ok, err := c.client.BucketExists(ctx, bucket)
	return ok, err
}

func (c *minioClient) MakeBucket(ctx context.Context, bucket, region string) error {
	return c.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region})
}

func (c *minioClient) RemoveBucket(ctx context.Context, bucket string) error {
	return c.client.RemoveBucket(ctx, bucket)
}

func (c *minioClient) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	buckets, err := c.client.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]BucketInfo, 0, len(buckets))
	for _, b := range buckets {
		result = append(result, BucketInfo{Name: b.Name, CreationDate: b.CreationDate})
	}
	return result, nil
}

func (c *minioClient) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	_, err := c.client.PutObject(ctx, bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (c *minioClient) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	obj, err := c.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, err
	}
	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, err
	}
	return obj, &ObjectInfo{
		Key:          stat.Key,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		ETag:         stat.ETag,
		ContentType:  stat.ContentType,
		StorageClass: stat.StorageClass,
	}, nil
}

func (c *minioClient) RemoveObject(ctx context.Context, bucket, key string) error {
	return c.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

func (c *minioClient) RemoveObjects(ctx context.Context, bucket string, keys ...string) <-chan minio.RemoveObjectError {
	ch := make(chan minio.ObjectInfo)
	go func() {
		defer close(ch)
		for _, k := range keys {
			ch <- minio.ObjectInfo{Key: k}
		}
	}()
	return c.client.RemoveObjects(ctx, bucket, ch, minio.RemoveObjectsOptions{})
}

func (c *minioClient) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	src := minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey}
	dst := minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey}
	_, err := c.client.CopyObject(ctx, dst, src)
	return err
}

func (c *minioClient) StatObject(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	stat, err := c.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &ObjectInfo{
		Key:          stat.Key,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		ETag:         stat.ETag,
		ContentType:  stat.ContentType,
		StorageClass: stat.StorageClass,
	}, nil
}

func (c *minioClient) ListObjects(ctx context.Context, bucket string, opts ListOptions) <-chan ObjectInfo {
	minioOpts := minio.ListObjectsOptions{
		Prefix:     opts.Prefix,
		Recursive:  opts.Recursive,
		MaxKeys:    opts.MaxKeys,
		StartAfter: opts.StartAfter,
	}
	out := make(chan ObjectInfo, 100)
	go func() {
		defer close(out)
		for obj := range c.client.ListObjects(ctx, bucket, minioOpts) {
			if obj.Err != nil {
				continue
			}
			out <- ObjectInfo{
				Key:          obj.Key,
				Size:         obj.Size,
				LastModified: obj.LastModified,
				ETag:         obj.ETag,
				ContentType:  obj.ContentType,
				StorageClass: obj.StorageClass,
				IsDir:        strings.HasSuffix(obj.Key, DirSuffix),
			}
		}
	}()
	return out
}

func (c *minioClient) PresignedGetObject(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	u, err := c.client.PresignedGetObject(ctx, bucket, key, expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (c *minioClient) PresignedPutObject(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	u, err := c.client.PresignedPutObject(ctx, bucket, key, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (c *minioClient) Close() error { return nil }

// ---- Operator ----

// Operator manages multiple object storage clients and provides high-level
// convenience methods for common business patterns.
//
// Web request flow (no local disk):
//
//	PutObject(ctx, name, bucket, key, fileHeader, fileHeader.Size, mime)  // frontend upload
//	GetObject(ctx, name, bucket, key) → (ReadCloser, ObjectInfo, error)  // stream to HTTP response
//
// Local file flow (background tasks):
//
//	FPutObject / FGetObject / FGetDirectory / FPutDirectory
type Operator struct {
	clients map[string]Client
	mu      sync.RWMutex
	closed  int32
	l       log.Logger
}

// NewOperator creates an Operator and registers the initial client.
func NewOperator(cfg *Config) (*Operator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	op := &Operator{
		clients: make(map[string]Client),
		l:       zapx.GetLogger(),
	}
	if err := op.RegisterClient(cfg); err != nil {
		return nil, err
	}
	return op, nil
}

// RegisterClient creates and registers a new client by config.
func (op *Operator) RegisterClient(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	client, err := newMinioClient(cfg)
	if err != nil {
		return fmt.Errorf("create obs client [%s] failed: %w", cfg.Name, err)
	}
	op.mu.Lock()
	defer op.mu.Unlock()
	op.clients[cfg.Name] = client
	return nil
}

// GetClient returns a client by name. Empty name returns the first one.
func (op *Operator) GetClient(name string) (Client, error) {
	op.mu.RLock()
	defer op.mu.RUnlock()
	if atomic.LoadInt32(&op.closed) == Closed {
		return nil, ErrOperatorClosed
	}
	if name == "" {
		for _, c := range op.clients {
			return c, nil
		}
		return nil, ErrClientNotFound
	}
	c, ok := op.clients[name]
	if !ok {
		return nil, ErrClientNotFound
	}
	return c, nil
}

// RemoveClient removes and closes a client by name.
func (op *Operator) RemoveClient(name string) error {
	op.mu.Lock()
	defer op.mu.Unlock()
	c, ok := op.clients[name]
	if !ok {
		return nil
	}
	delete(op.clients, name)
	return c.Close()
}

// SetLogger sets the operator's logger.
func (op *Operator) SetLogger(l log.Logger) { op.l = l }

func (op *Operator) logger() log.Logger {
	if op.l == nil {
		op.l = zapx.GetLogger()
	}
	return op.l
}

// Close closes all clients.
func (op *Operator) Close(ctx context.Context) error {
	op.mu.Lock()
	defer op.mu.Unlock()
	if atomic.LoadInt32(&op.closed) == Closed {
		return nil
	}
	atomic.StoreInt32(&op.closed, Closed)
	var errs []error
	for name, c := range op.clients {
		if err := c.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close client [%s]: %w", name, err))
		}
	}
	op.clients = make(map[string]Client)
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// ================================================================
// Web-first API — operates on io.Reader / io.ReadCloser.
// These are the primary methods for frontend-initiated requests.
// ================================================================

// ---- Bucket ----

// BucketExists checks whether a bucket exists.
func (op *Operator) BucketExists(ctx context.Context, name, bucket string) (bool, error) {
	c, err := op.GetClient(name)
	if err != nil {
		return false, err
	}
	return c.BucketExists(ctx, bucket)
}

// MakeBucket creates a bucket.
func (op *Operator) MakeBucket(ctx context.Context, name, bucket, region string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	return retry(ctx, op.logger(), func() error { return c.MakeBucket(ctx, bucket, region) })
}

// RemoveBucket deletes a bucket.
func (op *Operator) RemoveBucket(ctx context.Context, name, bucket string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	return c.RemoveBucket(ctx, bucket)
}

// ListBuckets lists all buckets.
func (op *Operator) ListBuckets(ctx context.Context, name string) ([]BucketInfo, error) {
	c, err := op.GetClient(name)
	if err != nil {
		return nil, err
	}
	return c.ListBuckets(ctx)
}

// ---- Object (stream-based, for web requests) ----

// PutObject uploads data from an io.Reader to object storage.
//
// Web usage — controller receives multipart upload:
//
//	fileHeader, _ := c.FormFile("file")
//	f, _ := fileHeader.Open()
//	defer f.Close()
//	operator.PutObject(ctx, "default", "my-bucket", "path/file.csv", f, fileHeader.Size, "text/csv")
func (op *Operator) PutObject(ctx context.Context, name, bucket, key string, body io.Reader, size int64, contentType string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	return retry(ctx, op.logger(), func() error {
		return c.PutObject(ctx, bucket, key, body, size, contentType)
	})
}

// GetObject downloads an object and returns its body as io.ReadCloser.
// Caller must close the returned ReadCloser.
//
// Web usage — controller streams to HTTP response:
//
//	body, info, _ := operator.GetObject(ctx, "default", bucket, key)
//	defer body.Close()
//	c.Header("Content-Type", info.ContentType)
//	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(key)))
//	io.Copy(c.Writer, body)
func (op *Operator) GetObject(ctx context.Context, name, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	c, err := op.GetClient(name)
	if err != nil {
		return nil, nil, err
	}
	return c.GetObject(ctx, bucket, key)
}

// RemoveObject deletes a single object.
func (op *Operator) RemoveObject(ctx context.Context, name, bucket, key string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	return c.RemoveObject(ctx, bucket, key)
}

// RemoveObjects batch-deletes objects.
func (op *Operator) RemoveObjects(ctx context.Context, name, bucket string, keys ...string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	for err := range c.RemoveObjects(ctx, bucket, keys...) {
		if err.Err != nil {
			return err.Err
		}
	}
	return nil
}

// CopyObject copies an object server-side.
func (op *Operator) CopyObject(ctx context.Context, name, srcBucket, srcKey, dstBucket, dstKey string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	return c.CopyObject(ctx, srcBucket, srcKey, dstBucket, dstKey)
}

// StatObject returns object metadata without downloading the body.
func (op *Operator) StatObject(ctx context.Context, name, bucket, key string) (*ObjectInfo, error) {
	c, err := op.GetClient(name)
	if err != nil {
		return nil, err
	}
	return c.StatObject(ctx, bucket, key)
}

// RenameObject renames/moves an object (copy + delete source).
// If srcKey ends with "/" the entire directory is renamed recursively.
func (op *Operator) RenameObject(ctx context.Context, name, bucket, srcKey, dstKey string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	if strings.HasSuffix(srcKey, DirSuffix) {
		for info := range c.ListObjects(ctx, bucket, ListOptions{Prefix: srcKey, Recursive: true}) {
			relKey := strings.TrimPrefix(info.Key, srcKey)
			if relKey == "" {
				continue
			}
			newKey := strings.TrimRight(dstKey, DirSuffix) + DirSuffix + relKey
			if err := c.CopyObject(ctx, bucket, info.Key, bucket, newKey); err != nil {
				return fmt.Errorf("copy %s -> %s: %w", info.Key, newKey, err)
			}
		}
		return op.DeleteDirectory(ctx, name, bucket, srcKey)
	}
	if err := c.CopyObject(ctx, bucket, srcKey, bucket, dstKey); err != nil {
		return err
	}
	return c.RemoveObject(ctx, bucket, srcKey)
}

// ---- Directory ----

// CreateDirectory creates a directory marker (empty object ending with "/").
func (op *Operator) CreateDirectory(ctx context.Context, name, bucket, prefix string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	dirKey := strings.TrimRight(prefix, DirSuffix) + DirSuffix
	return c.PutObject(ctx, bucket, dirKey, strings.NewReader(""), 0, "")
}

// DeleteDirectory recursively deletes all objects under prefix.
func (op *Operator) DeleteDirectory(ctx context.Context, name, bucket, prefix string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	prefix = strings.TrimRight(prefix, DirSuffix) + DirSuffix
	var keys []string
	for info := range c.ListObjects(ctx, bucket, ListOptions{Prefix: prefix, Recursive: true}) {
		keys = append(keys, info.Key)
	}
	if len(keys) == 0 {
		return nil
	}
	for err := range c.RemoveObjects(ctx, bucket, keys...) {
		if err.Err != nil {
			return err.Err
		}
	}
	return nil
}

// ExistDirectory checks whether any objects exist under the prefix.
func (op *Operator) ExistDirectory(ctx context.Context, name, bucket, prefix string) (bool, error) {
	c, err := op.GetClient(name)
	if err != nil {
		return false, err
	}
	prefix = strings.TrimRight(prefix, DirSuffix) + DirSuffix
	if _, err := c.StatObject(ctx, bucket, prefix); err == nil {
		return true, nil
	}
	for range c.ListObjects(ctx, bucket, ListOptions{Prefix: prefix, MaxKeys: 1, Recursive: true}) {
		return true, nil
	}
	return false, nil
}

// ---- List ----

// ListObjects returns a channel for streaming large listings.
func (op *Operator) ListObjects(ctx context.Context, name, bucket string, opts ListOptions) (<-chan ObjectInfo, error) {
	c, err := op.GetClient(name)
	if err != nil {
		return nil, err
	}
	return c.ListObjects(ctx, bucket, opts), nil
}

// ListAllObjects collects all objects under prefix into a slice.
func (op *Operator) ListAllObjects(ctx context.Context, name, bucket, prefix string) ([]ObjectInfo, error) {
	c, err := op.GetClient(name)
	if err != nil {
		return nil, err
	}
	var result []ObjectInfo
	for info := range c.ListObjects(ctx, bucket, ListOptions{Prefix: prefix, Recursive: true}) {
		result = append(result, info)
	}
	return result, nil
}

// ---- Presigned URLs ----

// PresignedGetObject generates a pre-signed URL for downloading (temporary access).
func (op *Operator) PresignedGetObject(ctx context.Context, name, bucket, key string, expiry time.Duration) (string, error) {
	c, err := op.GetClient(name)
	if err != nil {
		return "", err
	}
	return c.PresignedGetObject(ctx, bucket, key, expiry)
}

// PresignedPutObject generates a pre-signed URL for uploading (temporary access).
func (op *Operator) PresignedPutObject(ctx context.Context, name, bucket, key string, expiry time.Duration) (string, error) {
	c, err := op.GetClient(name)
	if err != nil {
		return "", err
	}
	return c.PresignedPutObject(ctx, bucket, key, expiry)
}

// ================================================================
// File-based API — F-prefix methods operate on local filesystem.
// Use these for background tasks, cron jobs, or scripts.
// For web requests, use PutObject / GetObject instead.
// ================================================================

// FPutObject uploads a local file to object storage.
func (op *Operator) FPutObject(ctx context.Context, name, bucket, key, filePath, contentType string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	return c.PutObject(ctx, bucket, key, f, fi.Size(), contentType)
}

// FGetObject downloads an object to a local file.
func (op *Operator) FGetObject(ctx context.Context, name, bucket, key, filePath string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(filePath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	body, info, err := c.GetObject(ctx, bucket, key)
	if err != nil {
		return err
	}
	defer body.Close()

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create %s: %w", filePath, err)
	}
	defer f.Close()

	_, err = io.Copy(f, body)
	_ = info // suppress unused
	return err
}

// FGetDirectory downloads all objects under prefix to a local directory.
func (op *Operator) FGetDirectory(ctx context.Context, name, bucket, prefix, localDir string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	prefix = strings.TrimRight(prefix, DirSuffix) + DirSuffix
	for info := range c.ListObjects(ctx, bucket, ListOptions{Prefix: prefix, Recursive: true}) {
		if info.IsDir {
			continue
		}
		relPath := strings.TrimPrefix(info.Key, prefix)
		localPath := filepath.Join(localDir, relPath)
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return err
		}
		body, _, getErr := c.GetObject(ctx, bucket, info.Key)
		if getErr != nil {
			return fmt.Errorf("get %s: %w", info.Key, getErr)
		}
		f, createErr := os.Create(localPath)
		if createErr != nil {
			body.Close()
			return fmt.Errorf("create %s: %w", localPath, createErr)
		}
		_, copyErr := io.Copy(f, body)
		body.Close()
		f.Close()
		if copyErr != nil {
			return fmt.Errorf("write %s: %w", localPath, copyErr)
		}
	}
	return nil
}

// FGetDirectoryAsZip downloads all objects under prefix and packs them into a zip file.
// The returned file path points to the generated zip. Caller should clean it up.
//
// Web usage — controller sends zip to frontend:
//
//	zipPath, _ := operator.FGetDirectoryAsZip(ctx, "default", bucket, prefix)
//	defer os.Remove(zipPath)
//	c.File(zipPath)
func (op *Operator) FGetDirectoryAsZip(ctx context.Context, name, bucket, prefix string) (zipPath string, err error) {
	c, err := op.GetClient(name)
	if err != nil {
		return "", err
	}
	prefix = strings.TrimRight(prefix, DirSuffix) + DirSuffix

	tmpFile, err := os.CreateTemp("", "obs-download-*.zip")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)
	defer zw.Close()

	for info := range c.ListObjects(ctx, bucket, ListOptions{Prefix: prefix, Recursive: true}) {
		if info.IsDir {
			continue
		}
		body, _, getErr := c.GetObject(ctx, bucket, info.Key)
		if getErr != nil {
			return tmpFile.Name(), fmt.Errorf("get %s: %w", info.Key, getErr)
		}
		relPath := strings.TrimPrefix(info.Key, prefix)
		w, createErr := zw.Create(relPath)
		if createErr != nil {
			body.Close()
			return tmpFile.Name(), fmt.Errorf("zip create %s: %w", relPath, createErr)
		}
		if _, copyErr := io.Copy(w, body); copyErr != nil {
			body.Close()
			return tmpFile.Name(), fmt.Errorf("zip write %s: %w", relPath, copyErr)
		}
		body.Close()
	}

	zw.Close()
	tmpFile.Close()
	return tmpFile.Name(), nil
}

// FPutDirectory recursively uploads a local directory to object storage.
func (op *Operator) FPutDirectory(ctx context.Context, name, bucket, prefix, localDir string) error {
	c, err := op.GetClient(name)
	if err != nil {
		return err
	}
	prefix = strings.TrimRight(prefix, DirSuffix) + DirSuffix
	return filepath.Walk(localDir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(localDir, path)
		key := prefix + filepath.ToSlash(relPath)
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return c.PutObject(ctx, bucket, key, f, fi.Size(), "")
	})
}

// ---- retry helper ----

func retry(ctx context.Context, l log.Logger, fn func() error) error {
	var err error
	for i := 0; i < DefaultMaxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if i < DefaultMaxRetries-1 {
			backoff := time.Duration(1<<uint(i)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return err
}
