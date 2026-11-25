package config

import "testing"

func TestPhotoConfiguration(t *testing.T) {
	t.Setenv("PROFILE_PHOTO_ENABLED", "")
	if c, err := LoadPhoto(false); err != nil || c.Enabled {
		t.Fatal("photo not optional")
	}
	t.Setenv("PROFILE_PHOTO_ENABLED", "true")
	if _, err := LoadPhoto(false); err == nil {
		t.Fatal("photos enabled without profile API")
	}
	if _, err := LoadPhoto(true); err == nil {
		t.Fatal("photos enabled without storage config")
	}
	for name, value := range map[string]string{"PROFILE_PHOTO_S3_ENDPOINT": "http://127.0.0.1:8333", "PROFILE_PHOTO_S3_BUCKET": "profile-test", "PROFILE_PHOTO_S3_REGION": "test-region", "PROFILE_PHOTO_S3_ACCESS_KEY": "synthetic-key", "PROFILE_PHOTO_S3_SECRET_KEY": "synthetic-secret"} {
		t.Setenv(name, value)
	}
	if c, err := LoadPhoto(true); err != nil || !c.Enabled {
		t.Fatal("valid photo config rejected")
	}
	t.Setenv("PROFILE_PHOTO_S3_SECRET_KEY", "")
	if _, err := LoadPhoto(true); err == nil {
		t.Fatal("missing secret accepted")
	}
}
