package artifact

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

var (
	ErrNotFound     = errors.New("artifact object not found")
	ErrInvalidRange = errors.New("artifact byte range is invalid")
)

type PutInput struct {
	TenantID    string
	RunID       string
	Filename    string
	ContentType string
	SizeBytes   int64
	Body        io.Reader
}

type StoredObject struct {
	Key string
}

type Object struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
	ContentRange  string
	LastModified  time.Time
}

type Store interface {
	Put(ctx context.Context, input PutInput) (StoredObject, error)
	Get(ctx context.Context, key string, rangeHeader string) (Object, error)
	Delete(ctx context.Context, key string) error
}

func objectKey(prefix string, tenantID string, runID string, filename string) string {
	parts := make([]string, 0, 6)
	if clean := cleanPrefix(prefix); clean != "" {
		parts = append(parts, clean)
	}
	parts = append(parts,
		"tenants", safeSegment(tenantID),
		"runs", safeSegment(runID),
		uuid.NewString()+"-"+SanitizeFilename(filename),
	)
	return path.Join(parts...)
}

func cleanPrefix(prefix string) string {
	segments := strings.FieldsFunc(strings.TrimSpace(prefix), func(r rune) bool {
		return r == '/' || r == '\\'
	})
	clean := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment = safeSegment(segment); segment != "" && segment != "unknown" {
			clean = append(clean, segment)
		}
	}
	return path.Join(clean...)
}

func SanitizeFilename(filename string) string {
	filename = strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/")
	filename = path.Base(filename)
	if filename == "" || filename == "." || filename == "/" {
		return "artifact.bin"
	}
	clean := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '.', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, filename)
	clean = strings.Trim(clean, ".-")
	if clean == "" {
		return "artifact.bin"
	}
	if len(clean) > 180 {
		clean = clean[:180]
	}
	return clean
}

func safeSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, value)
	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "unknown"
	}
	return clean
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-reader.ctx.Done():
		return 0, reader.ctx.Err()
	default:
		return reader.reader.Read(buffer)
	}
}
