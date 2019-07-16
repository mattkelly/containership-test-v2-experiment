package util

import (
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/kubernetes"
)

// This function is borrowed from kubernetes/kubernetes/test/utils
// It's hard to pull in that package, so not bothering to right now
func IsRetryableAPIError(err error) bool {
	// These errors may indicate a transient error that we can retry in tests.
	if apierrs.IsInternalError(err) || apierrs.IsTimeout(err) || apierrs.IsServerTimeout(err) ||
		apierrs.IsTooManyRequests(err) || utilnet.IsProbableEOF(err) || utilnet.IsConnectionReset(err) {
		return true
	}

	// If the error sends the Retry-After header, we respect it as an explicit confirmation we should retry.
	if _, shouldRetry := apierrs.SuggestsClientDelay(err); shouldRetry {
		return true
	}

	return false
}

// IsAuthError returns true if the error is an authentication
// or authorization error, else false
func IsAuthError(err error) bool {
	return apierrs.IsForbidden(err) || apierrs.IsUnauthorized(err)
}

// IsNodeReady returns true if the given node has a Ready status, else false.
// See https://kubernetes.io/docs/concepts/nodes/node/#condition for more info.
func IsNodeReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// GetClusterIDFromKubernetes gets the Containership cluster ID
// from the cluster-id label
func GetClusterIDFromKubernetes(kubeClientset kubernetes.Interface) (string, error) {
	nodeList, err := kubeClientset.CoreV1().
		Nodes().
		List(metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrap(err, "listing nodes to get cluster ID")
	}

	// Any node will do
	node := nodeList.Items[0]
	clusterID, ok := node.Labels["containership.io/cluster-id"]
	if !ok {
		return "", errors.Errorf("node %q is missing cluster-id label", node.Name)
	}

	return clusterID, nil
}
