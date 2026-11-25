package config

import (
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/objectstorage"
	"os"
)

type Photo struct {
	Enabled bool
	Storage objectstorage.Config
}

func LoadPhoto(profileEnabled bool) (Photo, error) {
	switch os.Getenv("PROFILE_PHOTO_ENABLED") {
	case "", "false":
		return Photo{}, nil
	case "true":
		if !profileEnabled {
			return Photo{}, ErrProfileConfig
		}
		c := objectstorage.Config{Endpoint: os.Getenv("PROFILE_PHOTO_S3_ENDPOINT"), Bucket: os.Getenv("PROFILE_PHOTO_S3_BUCKET"), Region: os.Getenv("PROFILE_PHOTO_S3_REGION"), AccessKey: os.Getenv("PROFILE_PHOTO_S3_ACCESS_KEY"), SecretKey: os.Getenv("PROFILE_PHOTO_S3_SECRET_KEY")}
		if c.Validate() != nil {
			return Photo{}, ErrProfileConfig
		}
		return Photo{Enabled: true, Storage: c}, nil
	default:
		return Photo{}, ErrProfileConfig
	}
}
