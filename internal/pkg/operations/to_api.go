package operations

import (
	"github.com/psyb0t/chatz/internal/pkg/http/api"
)

// ToAPI projects a secret-free operations snapshot to the public admin wire
// contract.
func ToAPI(snapshot Snapshot) api.AdminReadiness {
	response := api.AdminReadiness{
		AppVersion: snapshot.AppVersion,
		Backup:     backupToAPI(snapshot.Backup),
		DatabaseDriver: api.AdminReadinessDatabaseDriver(
			snapshot.DatabaseDriver,
		),
		MigrationDirty:   snapshot.Migration.Dirty,
		MigrationVersion: snapshot.Migration.Version,
		Upstreams:        upstreamHealthsToAPI(snapshot.UpstreamHealths),
	}
	if snapshot.Commit != "" {
		response.Commit = &snapshot.Commit
	}

	return response
}

func upstreamHealthsToAPI(healths []UpstreamHealth) []api.UpstreamHealth {
	responses := make([]api.UpstreamHealth, 0, len(healths))
	for _, health := range healths {
		response := api.UpstreamHealth{
			ConsecutiveFailures: health.ConsecutiveFailure,
			LastFailureClass:    optionalString(health.LastFailureClass),
			LastOperation:       optionalString(health.LastOperation),
			State:               api.UpstreamHealthState(health.State),
			Upstream:            health.Upstream,
		}
		if health.LastOperation != "" {
			latencyMilliseconds := health.LastLatency.Milliseconds()
			response.LastLatencyMs = &latencyMilliseconds
		}

		if !health.LastSuccessAt.IsZero() {
			response.LastSuccessAt = &health.LastSuccessAt
		}

		if !health.LastFailureAt.IsZero() {
			response.LastFailureAt = &health.LastFailureAt
		}

		responses = append(responses, response)
	}

	return responses
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func backupToAPI(status BackupStatus) api.BackupFreshness {
	response := api.BackupFreshness{
		State: api.BackupFreshnessState(status.State),
	}
	if !status.CompletedAt.IsZero() {
		response.CompletedAt = &status.CompletedAt
	}

	if status.Driver != "" {
		driver := api.BackupFreshnessDriver(status.Driver)
		response.Driver = &driver
	}

	return response
}
