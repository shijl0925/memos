package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/usememos/memos/api"
	metric "github.com/usememos/memos/plugin/metrics"
	"github.com/usememos/memos/server/profile"
	"github.com/usememos/memos/store"
	"github.com/usememos/memos/store/db"
)

type Server struct {
	app App
	db  *sql.DB

	ID        string
	Profile   *profile.Profile
	Store     *store.Store
	Collector *MetricCollector
}

func NewServer(ctx context.Context, profile *profile.Profile) (*Server, error) {
	db := db.NewDB(profile)
	if err := db.Open(ctx); err != nil {
		return nil, errors.Wrap(err, "cannot open db")
	}

	s := &Server{
		app:     newEchoApp(),
		db:      db.DBInstance,
		Profile: profile,
	}
	storeInstance := store.New(db.DBInstance, profile)
	s.Store = storeInstance

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

	serverID, err := s.getSystemServerID(ctx)
	if err != nil {
		return nil, err
	}
	s.ID = serverID

	secretSessionName := "usememos"
	if profile.Mode == "prod" {
		secretSessionName, err = s.getSystemSecretSessionName(ctx)
		if err != nil {
			return nil, err
		}
	}
	s.app.UseSession(secretSessionName)

	embedFrontend(s.app)

	// Register MetricCollector to server.
	s.registerMetricCollector()

	rootGroup := s.app.Group("")
	s.registerRSSRoutes(rootGroup)

	publicGroup := s.app.Group("/o")
	s.registerResourcePublicRoutes(publicGroup)
	registerGetterPublicRoutes(publicGroup)

	apiGroup := s.app.Group("/api")
	apiGroup.Use(aclMiddleware(s))
	s.registerSystemRoutes(apiGroup)
	s.registerAuthRoutes(apiGroup)
	s.registerUserRoutes(apiGroup)
	s.registerMemoRoutes(apiGroup)
	s.registerShortcutRoutes(apiGroup)
	s.registerResourceRoutes(apiGroup)
	s.registerTagRoutes(apiGroup)

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	if err := s.createServerStartActivity(ctx); err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	s.Collector.Identify(ctx)
	return s.app.Start(fmt.Sprintf(":%d", s.Profile.Port))
}

func (s *Server) Shutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.app.Shutdown(ctx); err != nil {
		fmt.Printf("failed to shutdown server, error: %v\n", err)
	}

	// Close database connection
	if err := s.db.Close(); err != nil {
		fmt.Printf("failed to close database, error: %v\n", err)
	}

	fmt.Printf("memos stopped properly\n")
}

func (s *Server) createServerStartActivity(ctx context.Context) error {
	payload := api.ActivityServerStartPayload{
		ServerID: s.ID,
		Profile:  s.Profile,
	}
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal activity payload")
	}
	activity, err := s.Store.CreateActivity(ctx, &api.ActivityCreate{
		CreatorID: api.UnknownID,
		Type:      api.ActivityServerStart,
		Level:     api.ActivityInfo,
		Payload:   string(payloadStr),
	})
	if err != nil || activity == nil {
		return errors.Wrap(err, "failed to create activity")
	}
	s.Collector.Collect(ctx, &metric.Metric{
		Name: string(activity.Type),
	})
	return err
}
