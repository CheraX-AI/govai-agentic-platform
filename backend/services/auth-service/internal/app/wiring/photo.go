package wiring

import (
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/objectstorage"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
)

func Photo(c config.Photo, store *postgres.Store, native *handler.Protected, web *handler.WebSessionHandler) (func(), error) {
	if !c.Enabled || store == nil || (native == nil && web == nil) {
		return nil, profile.ErrUnavailable
	}
	objects, err := objectstorage.New(c.Storage)
	if err != nil {
		return nil, profile.ErrUnavailable
	}
	fail := func() (func(), error) { objects.Close(); return nil, profile.ErrUnavailable }
	service, err := profile.NewPhotoService(store, objects)
	if err != nil {
		return fail()
	}
	if native != nil {
		if err = native.EnablePhoto(service); err != nil {
			return fail()
		}
	}
	if web != nil {
		if err = web.EnablePhoto(service); err != nil {
			return fail()
		}
	}
	return objects.Close, nil
}
