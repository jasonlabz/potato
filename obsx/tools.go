package obsx

import (
	"fmt"
	"strings"
)

// ParseOBSPath parses an OBS/S3 path into its components.
// Input "obs://bucket/path/to/file" returns protocol="obs", bucket="bucket", key="path/to/file/".
// Supports both "obs://" and "s3://" protocols.
func ParseOBSPath(path string) (protocol, bucket, key string) {
	path = strings.Trim(path, "/")
	cutProtocol := strings.SplitN(path, "://", 2)
	if len(cutProtocol) != 2 {
		return
	}
	protocol = cutProtocol[0]

	cutBucket := strings.SplitN(cutProtocol[1], "/", 2)
	if len(cutBucket) == 0 {
		return
	}
	bucket = cutBucket[0]
	if len(cutBucket) == 2 && cutBucket[1] != "" {
		key = cutBucket[1]
		if !strings.HasSuffix(key, "/") {
			key = key + "/"
		}
	}
	return
}

// SpliceOBSContentKey joins a prefix and suffix into a directory-style key.
func SpliceOBSContentKey(prefix, suffix string) string {
	if suffix == "" || suffix == "/" {
		return prefix
	}
	return fmt.Sprintf("%s/%s/", strings.Trim(prefix, "/"), strings.Trim(suffix, "/"))
}

// SpliceOBSFileKey joins a prefix and filename into a file key.
func SpliceOBSFileKey(prefix, fileName string) string {
	return fmt.Sprintf("%s/%s", strings.Trim(prefix, "/"), fileName)
}

// SpliceOBSFileKeyBySlice joins path segments into a directory-style key.
func SpliceOBSFileKeyBySlice(contents []string) string {
	return fmt.Sprintf("%s/", strings.Join(contents, "/"))
}

// TrimOBSKeyByPrefix removes a content prefix from an object key.
// e.g. TrimOBSKeyByPrefix("/mazhen04/a.py", "mazhen04") -> "a.py"
func TrimOBSKeyByPrefix(key, contentPrefix string) string {
	key = strings.TrimPrefix(key, "/")
	contentPrefix = strings.TrimPrefix(contentPrefix, "/")

	res := strings.TrimPrefix(key, contentPrefix)
	return strings.TrimPrefix(res, "/")
}

// StandardizeOBSPath ensures the path has the proper format.
func StandardizeOBSPath(path string) string {
	if path == "" {
		return path
	}
	if !strings.Contains(path, "://") {
		return ""
	}
	return fmt.Sprintf("%s/", strings.TrimSuffix(path, "/"))
}
