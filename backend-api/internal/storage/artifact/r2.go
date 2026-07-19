package artifact

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

var (
	accountIDPattern  = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
	bucketNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)
)

type R2Config struct {
	AccountID       string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Prefix          string
}

type objectTransfer interface {
	UploadObject(context.Context, *transfermanager.UploadObjectInput, ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error)
}

type objectClient interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type R2Store struct {
	bucket   string
	prefix   string
	transfer objectTransfer
	client   objectClient
}

func NewR2Store(config R2Config) (*R2Store, error) {
	accountID := strings.TrimSpace(config.AccountID)
	if !accountIDPattern.MatchString(accountID) {
		return nil, fmt.Errorf("R2 account ID must be 32 hexadecimal characters")
	}
	bucket := strings.TrimSpace(config.Bucket)
	if !bucketNamePattern.MatchString(bucket) {
		return nil, fmt.Errorf("R2 bucket must be 3-63 lowercase letters, numbers, or hyphens and cannot start or end with a hyphen")
	}
	if strings.TrimSpace(config.AccessKeyID) == "" || strings.TrimSpace(config.SecretAccessKey) == "" {
		return nil, fmt.Errorf("R2 access key ID and secret access key are required")
	}
	prefix := strings.TrimSpace(config.Prefix)
	if strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "..") || strings.Contains(prefix, `\`) {
		return nil, fmt.Errorf("R2 prefix must be a relative object key prefix without '..' or backslashes")
	}
	endpoint := "https://" + accountID + ".r2.cloudflarestorage.com"
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return nil, fmt.Errorf("build R2 endpoint: %w", err)
	}

	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "auto",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(config.AccessKeyID),
			strings.TrimSpace(config.SecretAccessKey),
			"",
		)),
	})
	transfer := transfermanager.New(client, func(options *transfermanager.Options) {
		options.PartSizeBytes = 8 << 20
		options.MultipartUploadThreshold = 16 << 20
		options.Concurrency = 3
		options.FailTimeout = 30 * time.Second
	})
	return newR2Store(bucket, prefix, transfer, client), nil
}

func newR2Store(bucket string, prefix string, transfer objectTransfer, client objectClient) *R2Store {
	return &R2Store{
		bucket:   strings.TrimSpace(bucket),
		prefix:   cleanPrefix(prefix),
		transfer: transfer,
		client:   client,
	}
}

func (store *R2Store) Put(ctx context.Context, input PutInput) (StoredObject, error) {
	if input.Body == nil {
		return StoredObject{}, fmt.Errorf("artifact body is required")
	}
	key := objectKey(store.prefix, input.TenantID, input.RunID, input.Filename)
	uploadInput := &transfermanager.UploadObjectInput{
		Bucket:       aws.String(store.bucket),
		Key:          aws.String(key),
		Body:         contextReader{ctx: ctx, reader: input.Body},
		CacheControl: aws.String("private, no-store"),
	}
	if input.SizeBytes >= 0 {
		uploadInput.ContentLength = aws.Int64(input.SizeBytes)
	}
	if input.ContentType != "" {
		uploadInput.ContentType = aws.String(input.ContentType)
	}
	if _, err := store.transfer.UploadObject(ctx, uploadInput); err != nil {
		return StoredObject{}, fmt.Errorf("upload artifact to R2: %w", err)
	}
	return StoredObject{Key: key}, nil
}

func (store *R2Store) Get(ctx context.Context, key string, rangeHeader string) (Object, error) {
	input := &s3.GetObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(key)}
	rangeHeader = strings.TrimSpace(rangeHeader)
	if strings.HasPrefix(rangeHeader, "bytes=") && !strings.ContainsAny(rangeHeader, "\r\n") {
		input.Range = aws.String(rangeHeader)
	}
	output, err := store.client.GetObject(ctx, input)
	if err != nil {
		if r2InvalidRange(err) {
			return Object{}, ErrInvalidRange
		}
		if r2ObjectNotFound(err) {
			return Object{}, ErrNotFound
		}
		return Object{}, fmt.Errorf("download artifact from R2: %w", err)
	}
	contentLength := int64(-1)
	if output.ContentLength != nil {
		contentLength = aws.ToInt64(output.ContentLength)
	}
	return Object{
		Body:          output.Body,
		ContentType:   aws.ToString(output.ContentType),
		ContentLength: contentLength,
		ContentRange:  aws.ToString(output.ContentRange),
		LastModified:  aws.ToTime(output.LastModified),
	}, nil
}

func r2InvalidRange(err error) bool {
	var apiError smithy.APIError
	return errors.As(err, &apiError) && apiError.ErrorCode() == "InvalidRange"
}

func (store *R2Store) Delete(ctx context.Context, key string) error {
	_, err := store.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(key),
	})
	if err != nil && !r2ObjectNotFound(err) {
		return fmt.Errorf("delete artifact from R2: %w", err)
	}
	return nil
}

func r2ObjectNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var apiError smithy.APIError
	if !errors.As(err, &apiError) {
		return false
	}
	switch apiError.ErrorCode() {
	case "NoSuchKey", "NotFound", "404":
		return true
	default:
		return false
	}
}
