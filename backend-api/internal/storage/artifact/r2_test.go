package artifact

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type fakeTransfer struct {
	input *transfermanager.UploadObjectInput
	body  []byte
	err   error
}

func (transfer *fakeTransfer) UploadObject(_ context.Context, input *transfermanager.UploadObjectInput, _ ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	transfer.input = input
	if transfer.err != nil {
		return nil, transfer.err
	}
	transfer.body, _ = io.ReadAll(input.Body)
	return &transfermanager.UploadObjectOutput{}, nil
}

type fakeObjectClient struct {
	getInput    *s3.GetObjectInput
	getOutput   *s3.GetObjectOutput
	getErr      error
	deleteInput *s3.DeleteObjectInput
	deleteErr   error
}

func (client *fakeObjectClient) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	client.getInput = input
	return client.getOutput, client.getErr
}

func (client *fakeObjectClient) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	client.deleteInput = input
	return &s3.DeleteObjectOutput{}, client.deleteErr
}

func TestR2StoreLifecycleMapping(t *testing.T) {
	transfer := &fakeTransfer{}
	lastModified := time.Now().UTC().Truncate(time.Second)
	client := &fakeObjectClient{getOutput: &s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("stored")),
		ContentType:   aws.String("text/plain"),
		ContentLength: aws.Int64(6),
		ContentRange:  aws.String("bytes 0-5/12"),
		LastModified:  &lastModified,
	}}
	store := newR2Store("private-bucket", "browser-agent/artifacts", transfer, client)

	stored, err := store.Put(context.Background(), PutInput{
		TenantID: "tenant-a", RunID: "run-a", Filename: "report.txt",
		ContentType: "text/plain", SizeBytes: 6, Body: bytes.NewReader([]byte("stored")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored.Key, "browser-agent/artifacts/tenants/tenant-a/runs/run-a/") {
		t.Fatalf("unexpected R2 key: %q", stored.Key)
	}
	if got := aws.ToString(transfer.input.Bucket); got != "private-bucket" {
		t.Fatalf("upload bucket = %q", got)
	}
	if !bytes.Equal(transfer.body, []byte("stored")) {
		t.Fatalf("upload body = %q", transfer.body)
	}

	object, err := store.Get(context.Background(), stored.Key, "bytes=0-5")
	if err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(client.getInput.Range); got != "bytes=0-5" {
		t.Fatalf("download range = %q", got)
	}
	if object.ContentRange != "bytes 0-5/12" || object.ContentLength != 6 || !object.LastModified.Equal(lastModified) {
		t.Fatalf("unexpected downloaded object: %#v", object)
	}
	object.Body.Close()

	if err := store.Delete(context.Background(), stored.Key); err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(client.deleteInput.Key); got != stored.Key {
		t.Fatalf("deleted key = %q, want %q", got, stored.Key)
	}
}

func TestR2StoreMapsObjectErrors(t *testing.T) {
	store := newR2Store("bucket", "", &fakeTransfer{}, &fakeObjectClient{
		getErr: &smithy.GenericAPIError{Code: "NoSuchKey", Message: "missing"},
	})
	if _, err := store.Get(context.Background(), "missing", ""); err != ErrNotFound {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}

	store.client = &fakeObjectClient{getErr: &smithy.GenericAPIError{Code: "InvalidRange", Message: "invalid"}}
	if _, err := store.Get(context.Background(), "key", "bytes=20-10"); err != ErrInvalidRange {
		t.Fatalf("Get() error = %v, want ErrInvalidRange", err)
	}
}

func TestR2StorePreservesUnknownContentLength(t *testing.T) {
	store := newR2Store("bucket", "", &fakeTransfer{}, &fakeObjectClient{getOutput: &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader("streamed")),
	}})
	object, err := store.Get(context.Background(), "key", "")
	if err != nil {
		t.Fatal(err)
	}
	defer object.Body.Close()
	if object.ContentLength != -1 {
		t.Fatalf("ContentLength = %d, want -1", object.ContentLength)
	}
}

func TestNewR2StoreValidatesConfiguration(t *testing.T) {
	_, err := NewR2Store(R2Config{AccountID: "bad", Bucket: "bucket", AccessKeyID: "key", SecretAccessKey: "secret"})
	if err == nil {
		t.Fatal("expected invalid account ID error")
	}
	_, err = NewR2Store(R2Config{AccountID: strings.Repeat("a", 32), Bucket: "", AccessKeyID: "key", SecretAccessKey: "secret"})
	if err == nil {
		t.Fatal("expected missing bucket error")
	}
	_, err = NewR2Store(R2Config{AccountID: strings.Repeat("a", 32), Bucket: "Invalid_Bucket", AccessKeyID: "key", SecretAccessKey: "secret"})
	if err == nil {
		t.Fatal("expected invalid bucket name error")
	}
	_, err = NewR2Store(R2Config{AccountID: strings.Repeat("a", 32), Bucket: "valid-bucket", Prefix: "../other", AccessKeyID: "key", SecretAccessKey: "secret"})
	if err == nil {
		t.Fatal("expected invalid object prefix error")
	}
}
