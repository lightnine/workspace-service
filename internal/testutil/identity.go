package testutil

import "git.woa.com/leondli/workspace-service/internal/domain/identity"

// RequestContext returns a valid identity for unit tests.
func RequestContext() identity.RequestContext {
	return identity.RequestContext{
		OwnerUIN:    "100001",
		UIN:         "200001",
		AppID:       "260073493",
		WorkspaceID: "ws-test",
	}.Normalize()
}
