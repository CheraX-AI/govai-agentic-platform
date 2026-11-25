package objectstorage

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestS3AdapterAndFailures(t *testing.T) {
	mode := "ok"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Error("unsigned request")
		}
		if r.URL.Path != "/profile-test/citizen-photos/object.jpg" {
			t.Error("unexpected key path")
		}
		if mode == "fail" {
			w.WriteHeader(503)
			_, _ = io.WriteString(w, "private storage details")
			return
		}
		if r.Method == "GET" {
			_, _ = io.WriteString(w, "synthetic photo")
		} else {
			w.WriteHeader(200)
		}
	}))
	defer server.Close()
	store, err := New(Config{server.URL, "profile-test", "test-region", "synthetic-key", "synthetic-secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if store.Put(ctx, "citizen-photos/object.jpg", []byte("synthetic photo"), "image/jpeg") != nil {
		t.Fatal("put failed")
	}
	if data, err := store.Get(ctx, "citizen-photos/object.jpg"); err != nil || string(data) != "synthetic photo" {
		t.Fatal("get failed")
	}
	if store.Delete(ctx, "citizen-photos/object.jpg") != nil {
		t.Fatal("delete failed")
	}
	mode = "fail"
	if err := store.Put(ctx, "citizen-photos/object.jpg", nil, "image/jpeg"); err != ErrStorage {
		t.Fatal("storage error not sanitized")
	}
	for _, endpoint := range []string{"http://remote.test", "https://user:password@remote.test", "https://remote.test/path", "https://remote.test?secret=x"} {
		if _, err := New(Config{endpoint, "profile-test", "region", "key", "secret"}); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
	}
}
func TestSeaweedFSIntegration(t *testing.T) {
	endpoint := os.Getenv("GOVAI_PHOTO_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("opt-in disposable SeaweedFS")
	}
	store, err := New(Config{endpoint, "govai-photo-test", "test-region", "synthetic-key", "synthetic-secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if _, err = store.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(store.bucket)}); err != nil {
		t.Fatal("test bucket creation", err)
	}
	key := "citizen-photos/integration.jpg"
	data := []byte("synthetic storage round trip")
	if store.Put(ctx, key, data, "image/jpeg") != nil {
		t.Fatal("SeaweedFS put")
	}
	read, err := store.Get(ctx, key)
	if err != nil || string(read) != string(data) {
		t.Fatal("SeaweedFS get")
	}
	if store.Delete(ctx, key) != nil {
		t.Fatal("SeaweedFS delete")
	}
	if _, err = store.Get(ctx, key); err == nil {
		t.Fatal("deleted object still returned")
	}
}
