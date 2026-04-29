package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	ninja "github.com/shijl0925/gin-ninja"
	"github.com/usememos/memos/api"
	"github.com/usememos/memos/server/profile"
	"github.com/usememos/memos/service"
	"github.com/usememos/memos/store"
	"github.com/usememos/memos/store/db"
)

type Server struct {
	app App
	db  *sql.DB

	ID      string
	Profile *profile.Profile
	Store   *store.Store
	Service *service.Service
}

func NewServer(ctx context.Context, profile *profile.Profile) (*Server, error) {
	db := db.NewDB(profile)
	if err := db.Open(ctx); err != nil {
		return nil, errors.Wrap(err, "cannot open db")
	}

	s := &Server{app: newApp(), db: db.DBInstance, Profile: profile}
	s.Store = store.New(db.DBInstance, profile)
	s.Service = service.New(s.Store, profile)

	s.app.UseLogger(`{"time":"${time_rfc3339}",` +
		`"method":"${method}","uri":"${uri}",` +
		`"status":${status},"error":"${error}"}` + "\n")
	s.app.UseGzip()
	s.app.UseCSRF("cookie:_csrf", s.DefaultAuthSkipper)
	s.app.UseCORS()
	s.app.UseSecure(SecureConfig{
		Skipper:            DefaultGetRequestSkipper,
		XSSProtection:      "1; mode=block",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "SAMEORIGIN",
		HSTSPreloadEnabled: false,
	})
	s.app.UseTimeout(30*time.Second, "Request timeout")

	serverID, err := s.Service.GetSystemServerID(ctx)
	if err != nil {
		return nil, err
	}
	s.ID = serverID

	embedFrontend(s.app)

	secret := "usememos"
	if profile.Mode == "prod" {
		secret, err = s.Service.GetSystemSecretSession(ctx)
		if err != nil {
			return nil, err
		}
	}

	s.app.AddController("", serverController{
		registerAPI: func(r *ninja.Router) {
			s.registerRSSRoutes(r)
		},
	})

	s.app.AddController("/o", serverController{
		middlewares: []MiddlewareFunc{JWTMiddleware(s, secret)},
		registerAPI: func(r *ninja.Router) {
			registerGetterPublicRoutes(r)
			s.registerResourcePublicRoutes(r)
		},
	})

	s.app.AddController("/api", serverController{
		middlewares: []MiddlewareFunc{JWTMiddleware(s, secret)},
		registerAPI: func(r *ninja.Router) {
			s.registerSystemRoutes(r)
			s.registerAuthRoutes(r, secret)
			s.registerUserRoutes(r)
			s.registerMemoRoutes(r)
			s.registerShortcutRoutes(r)
			s.registerResourceRoutes(r)
			s.registerTagRoutes(r)
			s.registerStorageRoutes(r)
			s.registerIdentityProviderRoutes(r)
			s.registerOpenAIRoutes(r)
		},
	})

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	if err := s.createServerStartActivity(ctx); err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return s.app.Start(fmt.Sprintf(":%d", s.Profile.Port))
}

func (s *Server) Shutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.app.Shutdown(ctx); err != nil {
		fmt.Printf("failed to shutdown server, error: %v\n", err)
	}
	if err := s.db.Close(); err != nil {
		fmt.Printf("failed to close database, error: %v\n", err)
	}

	fmt.Printf("memos stopped properly\n")
}

func (s *Server) createServerStartActivity(ctx context.Context) error {
	payload := api.ActivityServerStartPayload{ServerID: s.ID, Profile: s.Profile}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	_, err = s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: api.UnknownID,
		Type:      api.ActivityServerStart,
		Level:     api.ActivityInfo,
		Payload:   string(payloadBytes),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	return nil
}
