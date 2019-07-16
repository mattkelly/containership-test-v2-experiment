package e2e_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/containership/csctl/cloud"

	"github.com/mattkelly/containership-test-v2-experiment/context"
	provisiontests "github.com/mattkelly/containership-test-v2-experiment/tests/provision"
)

var testContext *context.TestContextDef

func TestIntegration(t *testing.T) {
	// Hook up gomega to ginkgo
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// Run only on first node
	token := os.Getenv("CONTAINERSHIP_TOKEN")
	if token == "" {
		// TODO logging?
		fmt.Println("please specify a Containership Cloud token via CONTAINERSHIP_TOKEN env var")
	}

	clientset, err := cloud.New(cloud.Config{
		Token:            token,
		APIBaseURL:       "https://stage-api.containership.io",
		AuthBaseURL:      "https://stage-auth.containership.io",
		ProvisionBaseURL: "https://stage-provision.containership.io",
	})
	Expect(err).NotTo(HaveOccurred())

	testContext = &context.TestContextDef{
		Clientset:      clientset,
		OrganizationID: testOrganizationID,
	}

	return nil
}, func(_ []byte) {
	// Run on all nodes after first node
	provisiontests.TestContext = testContext
})

var _ = SynchronizedAfterSuite(func() {
	// Run on all nodes
}, func() {
	// Run only on last node
})
