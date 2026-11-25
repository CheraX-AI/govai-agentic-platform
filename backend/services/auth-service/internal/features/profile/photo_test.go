package profile

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func photoJPEG() []byte {
	var b bytes.Buffer
	_ = jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil)
	return b.Bytes()
}

type photoFixture struct {
	owner                                    Caller
	details                                  Details
	data                                     map[string][]byte
	putFail, deleteFail, conflict, ambiguous bool
	events                                   []string
}

func (f *photoFixture) CitizenDetails(_ context.Context, issuer, subject string) (Details, error) {
	if issuer != f.owner.Issuer || subject != f.owner.Subject {
		return Details{}, ErrNotFound
	}
	return f.details, nil
}
func (f *photoFixture) SetCitizenPhoto(_ context.Context, issuer, subject string, revision int64, key string) (Details, error) {
	if issuer != f.owner.Issuer || subject != f.owner.Subject {
		return Details{}, ErrNotFound
	}
	if f.conflict || f.details.Revision != revision {
		return Details{}, ErrConflict
	}
	f.events = append(f.events, "commit")
	f.details.PhotoObjectKey = key
	f.details.Revision++
	if f.ambiguous {
		return Details{}, ErrUnavailable
	}
	return f.details, nil
}
func (f *photoFixture) Put(_ context.Context, key string, data []byte, _ string) error {
	if f.putFail {
		return ErrUnavailable
	}
	f.events = append(f.events, "put")
	f.data[key] = data
	return nil
}
func (f *photoFixture) Get(_ context.Context, key string) ([]byte, error) {
	v, ok := f.data[key]
	if !ok {
		return nil, ErrUnavailable
	}
	return v, nil
}
func (f *photoFixture) Delete(_ context.Context, key string) error {
	f.events = append(f.events, "delete")
	if f.deleteFail {
		return ErrUnavailable
	}
	delete(f.data, key)
	return nil
}

func TestPhotoValidation(t *testing.T) {
	var bounded photoBuffer
	if _, err := bounded.Write(make([]byte, MaxPhotoBytes+1)); !errors.Is(err, ErrPhotoSize) || bounded.Len() != MaxPhotoBytes {
		t.Fatal("encoded image buffer exceeded limit")
	}
	valid := photoJPEG()
	exact := append(append([]byte{}, valid...), make([]byte, MaxPhotoBytes-len(valid))...)
	if _, _, err := ValidatePhoto(bytes.NewReader(exact), "image/jpeg"); err != nil {
		t.Fatal("valid 2 MiB JPEG rejected", err)
	}
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	if _, _, err := ValidatePhoto(&pngData, "image/png"); err != nil {
		t.Fatal("PNG rejected")
	}
	var dimensions bytes.Buffer
	_ = png.Encode(&dimensions, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	oversized := dimensions.Bytes()
	binary.BigEndian.PutUint32(oversized[16:20], 100000)
	binary.BigEndian.PutUint32(oversized[20:24], 100000)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	if _, _, err := ValidatePhoto(bytes.NewReader(oversized), "image/png"); !errors.Is(err, ErrPhotoType) {
		t.Fatal("excessive decoded dimensions accepted")
	}
	for _, tc := range []struct {
		name, media string
		data        []byte
		expected    error
	}{
		{"oversized", "image/jpeg", make([]byte, 10<<20), ErrPhotoSize},
		{"unsupported", "image/gif", valid, ErrPhotoType},
		{"forged-type", "image/png", valid, ErrPhotoType},
		{"fake-jpeg", "image/jpeg", []byte("not an image"), ErrPhotoType},
		{"truncated", "image/jpeg", valid[:len(valid)/2], ErrPhotoType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := ValidatePhoto(bytes.NewReader(tc.data), tc.media); !errors.Is(err, tc.expected) {
				t.Fatal("invalid photo accepted")
			}
		})
	}
	cleaned, _, err := ValidatePhoto(bytes.NewReader(append(valid, []byte("private trailing metadata")...)), "image/jpeg")
	if err != nil || bytes.Contains(cleaned, []byte("private trailing metadata")) {
		t.Fatal("trailing metadata retained")
	}
}
func TestPhotoReplacementSafety(t *testing.T) {
	for _, mode := range []string{"success", "put-failure", "conflict", "ambiguous-commit", "cleanup-failure"} {
		t.Run(mode, func(t *testing.T) {
			old := "citizen-photos/" + strings.Repeat("a", 64) + ".jpg"
			owner := Caller{Issuer: "https://issuer.test", Subject: "citizen-a"}
			f := &photoFixture{owner: owner, details: Details{PhotoObjectKey: old}, data: map[string][]byte{old: photoJPEG()}, putFail: mode == "put-failure", conflict: mode == "conflict", ambiguous: mode == "ambiguous-commit", deleteFail: mode == "cleanup-failure"}
			service, _ := NewPhotoService(f, f)
			ctx := context.Background()
			if _, err := service.Replace(ctx, Caller{Issuer: owner.Issuer, Subject: "citizen-b"}, 0, bytes.NewReader(photoJPEG()), "image/jpeg"); !errors.Is(err, ErrNotFound) {
				t.Fatal("other citizen replacement accepted")
			}
			if _, _, _, err := service.Read(ctx, Caller{Issuer: owner.Issuer, Subject: "citizen-b"}); !errors.Is(err, ErrNotFound) {
				t.Fatal("other citizen read accepted")
			}
			saved, err := service.Replace(ctx, owner, 0, bytes.NewReader(photoJPEG()), "image/jpeg")
			if mode == "put-failure" || mode == "conflict" {
				if err == nil || f.details.PhotoObjectKey != old || f.data[old] == nil {
					t.Fatal("failed replacement changed live reference")
				}
				return
			}
			if mode == "ambiguous-commit" {
				if err == nil || f.data[f.details.PhotoObjectKey] == nil || f.data[old] == nil {
					t.Fatal("ambiguous commit deleted a potentially live object")
				}
				return
			}
			if err != nil || saved.Revision != 1 || saved.PhotoObjectKey == old || !ValidPhotoObjectKey(saved.PhotoObjectKey) {
				t.Fatal("replacement failed")
			}
			if strings.Join(f.events, ",") != "put,commit,delete" {
				t.Fatal("cleanup order invalid")
			}
			if mode == "success" && f.data[old] != nil {
				t.Fatal("old object retained")
			}
			if data, media, rev, err := service.Read(ctx, owner); err != nil || len(data) == 0 || media != "image/jpeg" || rev != 1 {
				t.Fatal("saved photo unreadable")
			}
		})
	}
}
