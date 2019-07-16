package scale

import (
	"os"
	"testing"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/containership/csctl/cloud"
	"github.com/containership/csctl/cloud/provision/types"

	"github.com/mattkelly/containership-test-v2-experiment/constants"
	testcontext "github.com/mattkelly/containership-test-v2-experiment/tests/context"
	"github.com/mattkelly/containership-test-v2-experiment/util"
)

type scaleContext struct {
	*testcontext.E2eTest

	// Node pool ID of the pool we're currently operating on.
	// Required to operate on the same pool across multiple It blocks (in order
	// to ideally end up back at the same state - i.e. scale a pool up and then
	// scale it back down)
	currentNodePoolID string
}

var context *scaleContext

func TestScale(t *testing.T) {
	// Hook up gomega to ginkgo
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scale Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// Run only on first node
	token := os.Getenv("CONTAINERSHIP_TOKEN")
	Expect(token).NotTo(BeEmpty(), "please specify a Containership Cloud token via CONTAINERSHIP_TOKEN env var")

	kubeconfigFilename := os.Getenv("KUBECONFIG")
	Expect(kubeconfigFilename).NotTo(BeEmpty(), "please set KUBECONFIG environment variable")

	clientset, err := cloud.New(cloud.Config{
		Token:            token,
		APIBaseURL:       constants.StageAPIBaseURL,
		AuthBaseURL:      constants.StageAuthBaseURL,
		ProvisionBaseURL: constants.StageProvisionBaseURL,
	})
	Expect(err).NotTo(HaveOccurred())

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigFilename)
	Expect(err).NotTo(HaveOccurred())

	kubeClientset, err := kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	clusterID, err := util.GetClusterIDFromKubernetes(kubeClientset)
	Expect(err).NotTo(HaveOccurred())

	context = &scaleContext{
		E2eTest: &testcontext.E2eTest{
			ContainershipClientset: clientset,
			KubernetesClientset:    kubeClientset,
			OrganizationID:         constants.TestOrganizationID,
			ClusterID:              clusterID,
		},
	}

	return nil
}, func(_ []byte) {
	// Run on all nodes after first one
})

var _ = Describe("Scaling a worker node pool", func() {
	It("should successfully request to scale up by one", func() {
		By("listing node pools")
		nodePools, err := context.ContainershipClientset.Provision().
			NodePools(context.OrganizationID, context.ClusterID).
			List()
		Expect(err).NotTo(HaveOccurred())

		// Any worker pool will do
		var pool *types.NodePool
		for _, p := range nodePools {
			if *p.KubernetesMode == "worker" {
				pool = &p
				break
			}
		}
		if pool == nil {
			// There are no worker pools - that's fine
			Skip("no worker pools to test scale up on")
		}

		// Save the pool that we're operating on in the context
		context.currentNodePoolID = string(pool.ID)

		targetCount := *pool.Count + 1
		req := types.NodePoolScaleRequest{
			Count: &targetCount,
		}

		_, err = context.ContainershipClientset.Provision().
			NodePools(context.OrganizationID, context.ClusterID).
			Scale(string(pool.ID), &req)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should go into UPDATING state", func() {
		Expect(waitForNodePoolUpdating(context.currentNodePoolID)).Should(Succeed())
		// TODO check count in cloud
	})

	It("should return to RUNNING state", func() {
		Expect(waitForNodePoolRunning(context.currentNodePoolID)).Should(Succeed())
		// TODO check for new node in Kubernetes and cloud
	})

	It("should successfully request to scale down by one", func() {
		pool, err := context.ContainershipClientset.Provision().
			NodePools(context.OrganizationID, context.ClusterID).
			Get(context.currentNodePoolID)
		Expect(err).NotTo(HaveOccurred())

		targetCount := *pool.Count - 1
		req := types.NodePoolScaleRequest{
			Count: &targetCount,
		}

		_, err = context.ContainershipClientset.Provision().
			NodePools(context.OrganizationID, context.ClusterID).
			Scale(string(pool.ID), &req)
		Expect(err).NotTo(HaveOccurred())

		Expect(waitForNodePoolUpdating(context.currentNodePoolID)).Should(Succeed())

		Expect(waitForNodePoolRunning(context.currentNodePoolID)).Should(Succeed())
	})

	It("should go into UPDATING state", func() {
		Expect(waitForNodePoolUpdating(context.currentNodePoolID)).Should(Succeed())
		// TODO this transition can be missed because a delete happens so quickly
		// TODO check count in cloud
	})

	It("should return to RUNNING state", func() {
		Expect(waitForNodePoolRunning(context.currentNodePoolID)).Should(Succeed())
		// TODO check for node deleted in Kubernetes and cloud
	})
})

func waitForNodePoolUpdating(id string) error {
	return wait.PollImmediate(constants.DefaultPollInterval,
		constants.DefaultTimeout,
		func() (bool, error) {
			pool, err := context.ContainershipClientset.Provision().
				NodePools(context.OrganizationID, context.ClusterID).
				Get(id)
			if err != nil {
				return false, errors.Wrapf(err, "GETing node pool %q", id)
			}

			status := *pool.Status.Type
			switch status {
			case "RUNNING":
				return false, nil
			case "UPDATING":
				return true, nil
			default:
				return false, errors.Errorf("node pool %q entered unexpected state %q", pool.ID, status)
			}
		})
}

func waitForNodePoolRunning(id string) error {
	return wait.PollImmediate(constants.DefaultPollInterval,
		constants.DefaultTimeout,
		func() (bool, error) {
			pool, err := context.ContainershipClientset.Provision().
				NodePools(context.OrganizationID, context.ClusterID).
				Get(id)
			if err != nil {
				return false, errors.Wrapf(err, "GETing node pool %q", id)
			}

			status := *pool.Status.Type
			switch status {
			case "UPDATING":
				return false, nil
			case "RUNNING":
				return true, nil
			default:
				return false, errors.Errorf("node pool %q entered unexpected state %q", pool.ID, status)
			}
		})
}
