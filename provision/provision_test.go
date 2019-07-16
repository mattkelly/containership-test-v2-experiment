package provision

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"text/template"
	"time"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/containership/csctl/cloud"
	"github.com/containership/csctl/cloud/provision/types"

	"github.com/mattkelly/containership-test-v2-experiment/constants"
	"github.com/mattkelly/containership-test-v2-experiment/util"
)

// The provisionContext is different from the context required for other tests,
// thus we split it out here
type provisionContext struct {
	ContainershipClientset cloud.Interface
	KubernetesClientset    kubernetes.Interface

	// AuthToken is only required because we can't pull the token back out of
	// the Containership clientset to use it again
	AuthToken string

	KubeconfigFilename string

	// These will be initialized at different times; however, once they are
	// set, they should never be mutated again.
	OrganizationID string
	TemplateID     string
	ClusterID      string
}

var context *provisionContext

// Flags
var (
	templateFilename string
	clusterFilename  string

	kubernetesVersion string
)

func init() {
	// These are the base files to use
	flag.StringVar(&templateFilename, "template", "", "path to template file to use")
	flag.StringVar(&clusterFilename, "cluster", "", "path to cluster file to use")

	// These override values in the base files
	flag.StringVar(&kubernetesVersion, "kubernetes-version", "", "Kubernetes version to provision")
}

func TestProvision(t *testing.T) {
	// Hook up gomega to ginkgo
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provision Suite")
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

	context = &provisionContext{
		ContainershipClientset: clientset,
		AuthToken:              token,
		KubeconfigFilename:     kubeconfigFilename,
		OrganizationID:         constants.TestOrganizationID,
	}

	return nil
}, func(_ []byte) {
	// Run on all nodes after first one
})

var _ = Describe("Provisioning a cluster", func() {
	It("should successfully create the template", func() {
		By("building template create request from file")
		// TODO this should be reading a yaml.go template for which we template
		// in values. Currently just reads a json file and then we override
		// values.
		req, err := readCreateTemplateRequestFromFile(templateFilename)
		Expect(err).NotTo(HaveOccurred())
		Expect(req).NotTo(BeNil())

		// Override defaults
		for name, nodePool := range req.Configuration.Variable {
			nodePool.Default.KubernetesVersion = &kubernetesVersion
		}

		By("POSTing the template create request")
		resp, err := context.ContainershipClientset.Provision().
			Templates(context.OrganizationID).
			Create(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Set template ID in global context - should never be mutated after this
		context.TemplateID = string(resp.ID)
	})

	It("should successfully initiate provisioning", func() {
		By("building cluster create request from file")
		req, err := readCreateCKEClusterRequestFromFile(clusterFilename)
		Expect(err).NotTo(HaveOccurred())
		Expect(req).NotTo(BeNil())

		// Override defaults
		req.TemplateID = types.UUID(context.TemplateID)

		By("POSTing the cluster create request")
		resp, err := context.ContainershipClientset.Provision().
			CKEClusters(context.OrganizationID).
			Create(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Set cluster ID in global context - should never be mutated after this
		context.ClusterID = string(resp.ID)
	})

	It("should successfully write kubeconfig", func() {
		Expect(writeKubeconfig(context.KubeconfigFilename,
			context.OrganizationID,
			context.ClusterID,
			context.AuthToken)).
			Should(Succeed())
	})

	It("should successfully initialize a Kubernetes clientset", func() {
		cfg, err := clientcmd.BuildConfigFromFlags("", context.KubeconfigFilename)
		Expect(err).NotTo(HaveOccurred())

		kubeClientset, err := kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		// Set Kubernetes clientset in global context - should never be mutated after this
		context.KubernetesClientset = kubeClientset
	})

	It("should eventually attach properly (report as running)", func() {
		Expect(waitForClusterRunning()).Should(Succeed())
	})

	It("should eventually have all node pools report as running", func() {
		Expect(waitForAllNodePoolsRunning()).Should(Succeed())
	})

	It("should eventually have a reachable API server", func() {
		Expect(waitForKubernetesAPIReady()).Should(Succeed())
	})

	It("should have all nodes ready in Kubernetes API", func() {
		Expect(waitForKubernetesNodesReady()).Should(Succeed())
	})
})

func readCreateTemplateRequestFromFile(filename string) (*types.CreateTemplateRequest, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "opening file")
	}
	defer f.Close()

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.Wrap(err, "reading file")
	}

	req := &types.CreateTemplateRequest{}

	err = json.Unmarshal(bytes, req)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling file into request type")
	}

	return req, nil
}

func readCreateCKEClusterRequestFromFile(filename string) (*types.CreateCKEClusterRequest, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "opening file")
	}
	defer f.Close()

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.Wrap(err, "reading file")
	}

	req := &types.CreateCKEClusterRequest{}

	err = json.Unmarshal(bytes, req)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling file into request type")
	}

	return req, nil
}

func waitForClusterRunning() error {
	return wait.PollImmediate(1*time.Second, 20*time.Minute, func() (bool, error) {
		cluster, err := context.ContainershipClientset.Provision().
			CKEClusters(context.OrganizationID).
			Get(context.ClusterID)
		if err != nil {
			return false, errors.Wrap(err, "GETing cluster")
		}

		status := *cluster.Status.Type
		switch status {
		case "RUNNING":
			return true, nil
		case "PROVISIONING":
			return false, nil
		default:
			return false, errors.Errorf("cluster entered unexpected state %q", status)
		}
	})
}

func waitForAllNodePoolsRunning() error {
	return wait.PollImmediate(constants.DefaultPollInterval,
		constants.DefaultTimeout,
		func() (bool, error) {
			pools, err := context.ContainershipClientset.Provision().
				NodePools(context.OrganizationID, context.ClusterID).
				List()
			if err != nil {
				return false, errors.Wrap(err, "GETing node pools")
			}

			running := true
			for _, pool := range pools {
				status := *pool.Status.Type
				switch status {
				case "RUNNING":
					continue
				case "UPDATING":
					running = false
					break
				default:
					return false, errors.Errorf("node pool %q entered unexpected state %q", pool.ID, status)
				}
			}

			return running, nil
		})
}

func waitForKubernetesAPIReady() error {
	return wait.PollImmediate(constants.DefaultPollInterval,
		constants.DefaultTimeout,
		func() (bool, error) {
			_, err := context.KubernetesClientset.CoreV1().
				Pods(corev1.NamespaceDefault).
				List(metav1.ListOptions{})
			if err != nil {
				// Ignore auth errors because we're aggressively polling
				// the cluster before the roles and bindings may be synced
				if util.IsRetryableAPIError(err) || util.IsAuthError(err) {
					return false, nil
				}

				return false, errors.Wrap(err, "listing pods in default namespace to check API health")
			}

			return true, nil
		})
}

func waitForKubernetesNodesReady() error {
	return wait.PollImmediate(constants.DefaultPollInterval,
		constants.DefaultTimeout,
		func() (bool, error) {
			nodeList, err := context.KubernetesClientset.CoreV1().
				Nodes().
				List(metav1.ListOptions{})
			if err != nil {
				if util.IsRetryableAPIError(err) {
					return false, nil
				}

				return false, errors.Wrap(err, "listing nodes")
			}

			for _, node := range nodeList.Items {
				if !util.IsNodeReady(node) {
					return false, nil
				}
			}

			return true, nil
		})
}

func writeKubeconfig(filename, organizationID, clusterID, authToken string) error {
	const kubeconfigTemplate = `
apiVersion: v1
clusters:
- cluster:
    server: https://stage-proxy.containership.io/v3/organizations/{{.OrganizationID}}/clusters/{{.ClusterID}}/k8sapi/proxy
  name: cs-e2e-test-cluster
contexts:
- context:
    cluster: cs-e2e-test-cluster
    user: cs-e2e-test-user
  name: cs-e2e-test-ctx
current-context: cs-e2e-test-ctx
kind: Config
preferences: {}
users:
- name: cs-e2e-test-user
  user:
    token: {{.AuthToken}}
`

	tmpl := template.Must(template.New("kubeconfig").Parse(kubeconfigTemplate))

	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	values := struct {
		OrganizationID string
		ClusterID      string
		AuthToken      string
	}{
		OrganizationID: organizationID,
		ClusterID:      clusterID,
		AuthToken:      authToken,
	}

	return tmpl.Execute(f, values)
}
