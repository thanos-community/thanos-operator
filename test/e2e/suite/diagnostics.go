package suite

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
)

var diagnosticNamespaces = map[string]struct{}{}

var _ = ginkgo.AfterEach(func() {
	if !ginkgo.CurrentSpecReport().Failed() {
		return
	}
	for namespace := range diagnosticNamespaces {
		dumpNamespace(namespace)
	}
})

// dumpNamespace records workload state and logs before a failed test's namespace is deleted.
func dumpNamespace(namespace string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	run := func(args ...string) []byte {
		args = append([]string{"--namespace", namespace, "--request-timeout=10s"}, args...)
		cmd := exec.CommandContext(ctx, "kubectl", args...)
		output, err := cmd.CombinedOutput()
		_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "\n$ kubectl %s\n%s", strings.Join(args, " "), output)
		if err != nil {
			_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "\ndiagnostic command failed: %v\n", err)
			return nil
		}
		return output
	}

	run("get", "deployments,statefulsets,pods,pvc,endpointslices", "-o", "wide")
	run("get", "events", "--sort-by=.lastTimestamp")
	run("describe", "pods")
	run("get", "configmaps", "-l", manifests.ComponentLabel+"="+receive.RouterComponentName, "-o", "yaml")
	for _, pod := range strings.Fields(string(run("get", "pods", "-o", "name"))) {
		run("logs", pod, "--all-containers=true", "--prefix=true", "--tail=100")
		run("logs", pod, "--all-containers=true", "--prefix=true", "--tail=100", "--previous")
	}
}
