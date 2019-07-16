package constants

import (
	"time"
)

const (
	// TODO this should be passed in
	TestOrganizationID = "62e4e86f-fe2e-4740-a814-a950bf377daf"

	StageAPIBaseURL       = "https://stage-api.containership.io"
	StageAuthBaseURL      = "https://stage-auth.containership.io"
	StageProvisionBaseURL = "https://stage-provision.containership.io"
)

const (
	// Faster feedback is better. We have nothing to lose by just polling
	// rapidly in e2e tests.
	DefaultPollInterval = 500 * time.Millisecond
	DefaultTimeout      = 5 * time.Minute

	// A namespace delete can take a long time. This matches the equivalent
	// Kubernetes e2e constant at the time of writing.
	NamespaceDeleteTimeout = 15 * time.Minute
)
