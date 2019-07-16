package context

import (
	"k8s.io/client-go/kubernetes"

	"github.com/containership/csctl/cloud"
)

// The E2e test context holds state for the entire
// e2e test run; it is not specific to any suite.
type E2eTest struct {
	// These should be fully initialized immediately
	ContainershipClientset cloud.Interface
	KubernetesClientset    kubernetes.Interface

	// These will be initialized at different times by different suites; however,
	// once they are set, they should never be mutated again.
	OrganizationID string
	ClusterID      string
}
