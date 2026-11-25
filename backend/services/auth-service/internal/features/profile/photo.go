package profile

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"
)

const MaxPhotoBytes = 2 << 20
const MaxPhotoPixels = 16_000_000

var ErrPhotoSize = errors.New("profile photo too large")
var ErrPhotoType = errors.New("unsupported profile photo")

type PhotoObjects interface {
	Put(context.Context, string, []byte, string) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}
type PhotoPersistence interface {
	CitizenDetails(context.Context, string, string) (Details, error)
	SetCitizenPhoto(context.Context, string, string, int64, string) (Details, error)
}
type PhotoService struct {
	store   PhotoPersistence
	objects PhotoObjects
	slots   chan struct{}
}

// Stop encoding at the storage limit rather than growing a buffer to the full
// uncompressed/noisy image size before rejecting it.
type photoBuffer struct{ buffer bytes.Buffer }

func (b *photoBuffer) Len() int      { return b.buffer.Len() }
func (b *photoBuffer) Bytes() []byte { return b.buffer.Bytes() }
func (b *photoBuffer) Write(p []byte) (int, error) {
	remaining := MaxPhotoBytes - b.Len()
	if len(p) > remaining {
		n, _ := b.buffer.Write(p[:remaining])
		return n, ErrPhotoSize
	}
	return b.buffer.Write(p)
}

func NewPhotoService(store PhotoPersistence, objects PhotoObjects) (*PhotoService, error) {
	if store == nil || objects == nil {
		return nil, ErrUnavailable
	}
	return &PhotoService{store, objects, make(chan struct{}, 2)}, nil
}

// ValidatePhoto checks actual encoded content and dimensions before decoding.
// Re-encoding strips EXIF/other metadata and trailing non-image payloads.
func ValidatePhoto(reader io.Reader, media string) ([]byte, string, error) {
	if media != "image/jpeg" && media != "image/png" {
		return nil, "", ErrPhotoType
	}
	if reader == nil {
		return nil, "", ErrPhotoType
	}
	raw, err := io.ReadAll(io.LimitReader(reader, MaxPhotoBytes+1))
	if len(raw) > MaxPhotoBytes {
		return nil, "", ErrPhotoSize
	}
	if err != nil {
		return nil, "", ErrPhotoType
	}
	if http.DetectContentType(raw) != media {
		return nil, "", ErrPhotoType
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > MaxPhotoPixels {
		return nil, "", ErrPhotoType
	}
	if (media == "image/jpeg" && format != "jpeg") || (media == "image/png" && format != "png") {
		return nil, "", ErrPhotoType
	}
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", ErrPhotoType
	}
	var encoded photoBuffer
	if media == "image/jpeg" {
		err = jpeg.Encode(&encoded, decoded, &jpeg.Options{Quality: 90})
	} else {
		err = png.Encode(&encoded, decoded)
	}
	if err != nil {
		if errors.Is(err, ErrPhotoSize) {
			return nil, "", ErrPhotoSize
		}
		return nil, "", ErrPhotoType
	}
	if encoded.Len() > MaxPhotoBytes {
		return nil, "", ErrPhotoSize
	}
	return encoded.Bytes(), media, nil
}

func ValidPhotoObjectKey(key string) bool {
	if !strings.HasPrefix(key, "citizen-photos/") || len(key) > 128 {
		return false
	}
	name := strings.TrimPrefix(key, "citizen-photos/")
	suffix := ".jpg"
	if strings.HasSuffix(name, ".png") {
		suffix = ".png"
	}
	stem := strings.TrimSuffix(name, suffix)
	decoded, err := hex.DecodeString(stem)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == stem && name == stem+suffix
}

func (s *PhotoService) Replace(ctx context.Context, caller Caller, revision int64, reader io.Reader, media string) (Details, error) {
	if caller.Issuer == "" || caller.Subject == "" {
		return Details{}, ErrNotFound
	}
	before, err := s.store.CitizenDetails(ctx, caller.Issuer, caller.Subject)
	if err != nil {
		return Details{}, err
	}
	if revision < 0 || revision > MaxClientRevision {
		return Details{}, ErrInvalid
	}
	if before.Revision != revision {
		return Details{}, ErrConflict
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return Details{}, ErrUnavailable
	}
	data, media, err := ValidatePhoto(reader, media)
	if err != nil {
		return Details{}, err
	}
	var random [32]byte
	if _, err = rand.Read(random[:]); err != nil {
		return Details{}, ErrUnavailable
	}
	suffix := ".jpg"
	if media == "image/png" {
		suffix = ".png"
	}
	key := "citizen-photos/" + hex.EncodeToString(random[:]) + suffix
	if err = s.objects.Put(ctx, key, data, media); err != nil {
		return Details{}, ErrUnavailable
	}
	saved, err := s.store.SetCitizenPhoto(ctx, caller.Issuer, caller.Subject, revision, key)
	if err != nil {
		// Only a definite no-write conflict permits deleting the candidate. For a
		// connection/commit ambiguity it might be the live reference: retain it.
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) {
			s.cleanup(key)
		}
		return Details{}, err
	}
	// Never delete the old object before the new reference is committed.
	if before.PhotoObjectKey != "" && before.PhotoObjectKey != key && ValidPhotoObjectKey(before.PhotoObjectKey) {
		s.cleanup(before.PhotoObjectKey)
	}
	return saved, nil
}
func (s *PhotoService) cleanup(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = s.objects.Delete(ctx, key)
}

func (s *PhotoService) Read(ctx context.Context, caller Caller) ([]byte, string, int64, error) {
	if caller.Issuer == "" || caller.Subject == "" {
		return nil, "", 0, ErrNotFound
	}
	details, err := s.store.CitizenDetails(ctx, caller.Issuer, caller.Subject)
	if err != nil {
		return nil, "", 0, err
	}
	if details.PhotoObjectKey == "" {
		return nil, "", 0, ErrNotFound
	}
	if !ValidPhotoObjectKey(details.PhotoObjectKey) {
		return nil, "", 0, ErrUnavailable
	}
	data, err := s.objects.Get(ctx, details.PhotoObjectKey)
	if err != nil || len(data) > MaxPhotoBytes {
		return nil, "", 0, ErrUnavailable
	}
	media := http.DetectContentType(data)
	if media != "image/jpeg" && media != "image/png" {
		return nil, "", 0, ErrUnavailable
	}
	return data, media, details.Revision, nil
}
