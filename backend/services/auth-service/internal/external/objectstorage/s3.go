package objectstorage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var ErrStorage = errors.New("profile object storage unavailable")

type Config struct{ Endpoint, Bucket, Region, AccessKey, SecretKey string }

func (Config) String() string               { return "[REDACTED]" }
func (Config) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (Config) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (c Config) Validate() error {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Opaque != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return ErrStorage
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return ErrStorage
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`).MatchString(c.Bucket) || c.Region == "" || len(c.Region) > 64 || c.AccessKey == "" || c.SecretKey == "" {
		return ErrStorage
	}
	return nil
}

type Store struct {
	client *s3.Client
	bucket string
	http   *http.Client
}

func New(c Config) (*Store, error) {
	if c.Validate() != nil {
		return nil, ErrStorage
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	sdk := s3.New(s3.Options{Region: c.Region, BaseEndpoint: aws.String(strings.TrimRight(c.Endpoint, "/")), UsePathStyle: true, Credentials: credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""), HTTPClient: client, RetryMaxAttempts: 1, RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired, ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired})
	return &Store{sdk, c.Bucket, client}, nil
}
func (s *Store) Close() { s.http.CloseIdleConnections() }
func (s *Store) Put(ctx context.Context, key string, data []byte, media string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(data), ContentType: aws.String(media)})
	if err != nil {
		return ErrStorage
	}
	return nil
}
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, ErrStorage
	}
	defer result.Body.Close()
	data, err := io.ReadAll(io.LimitReader(result.Body, profile.MaxPhotoBytes+1))
	if err != nil || len(data) > profile.MaxPhotoBytes {
		return nil, ErrStorage
	}
	return data, nil
}
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return ErrStorage
	}
	return nil
}
