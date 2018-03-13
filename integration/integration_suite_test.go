package integration_test

import (
	"log"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var execPath string

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	srcPath := "github.com/cloudfoundry/service-metrics"
	var err error
	execPath, err = gexec.Build(srcPath)
	if err != nil {
		log.Fatalf("executable %s could not be built: %s", srcPath, err)
	}
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func runCmd(
	origin string,
	sourceID string,
	debugLog bool,
	agentAddress string,
	metricsInterval string,
	metricsCmd string,
	caPath string,
	certPath string,
	keyPath string,
	metricsCmdArgs ...string,
) *gexec.Session {
	cmdArgs := []string{
		"--origin", origin,
		"--source-id", sourceID,
		"--agent-addr", agentAddress,
		"--metrics-interval", metricsInterval,
		"--metrics-cmd", metricsCmd,
		"--ca", caPath,
		"--cert", certPath,
		"--key", keyPath,
	}

	if debugLog {
		cmdArgs = append(cmdArgs, "--debug")
	}

	for _, arg := range metricsCmdArgs {
		cmdArgs = append(cmdArgs, "--metrics-cmd-arg", arg)
	}

	cmd := exec.Command(execPath, cmdArgs...)

	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	return session
}
