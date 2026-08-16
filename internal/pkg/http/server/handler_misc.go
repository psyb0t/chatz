package server

import (
	"context"

	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/psyb0t/chatz/internal/pkg/operations"
	"github.com/psyb0t/chatz/internal/pkg/upstreams"
	"github.com/psyb0t/ctxerrors"
)

// GetHealth is the liveness probe.
func (s *Server) GetHealth(
	_ context.Context,
	_ api.GetHealthRequestObject,
) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Status: "ok"}, nil
}

// GetAdminReadiness returns the truthful operational snapshot for an admin.
func (s *Server) GetAdminReadiness(
	ctx context.Context,
	_ api.GetAdminReadinessRequestObject,
) (api.GetAdminReadinessResponseObject, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	if s.deps.Readiness == nil {
		return nil, ctxerrors.New("admin readiness is not configured")
	}

	snapshot, err := s.deps.Readiness.Snapshot(ctx)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "read admin readiness")
	}

	return api.GetAdminReadiness200JSONResponse(operations.ToAPI(snapshot)), nil
}

// ListModels returns the merged model list across all configured upstreams.
func (s *Server) ListModels(
	ctx context.Context,
	_ api.ListModelsRequestObject,
) (api.ListModelsResponseObject, error) {
	if _, err := requireUser(ctx); err != nil {
		return nil, err
	}

	out := api.ListModels200JSONResponse{}

	if s.deps.Models != nil {
		for _, m := range s.deps.Models.Models() {
			out = append(out, upstreams.ModelToAPI(m))
		}
	}

	return out, nil
}

// ListUpstreamHealth returns redacted per-upstream health for administrators.
func (s *Server) ListUpstreamHealth(
	ctx context.Context,
	_ api.ListUpstreamHealthRequestObject,
) (api.ListUpstreamHealthResponseObject, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	if s.deps.Models == nil {
		return api.ListUpstreamHealth200JSONResponse{}, nil
	}

	return api.ListUpstreamHealth200JSONResponse(
		upstreams.HealthsToAPI(s.deps.Models.Health()),
	), nil
}
