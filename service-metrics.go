package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/service-metrics/metrics"
)

var (
	origin          string
	agentAddr       string
	metricsInterval time.Duration
	metricsCmd      string
	metricsCmdArgs  multiFlag
	debug           bool
	caPath          string
	certPath        string
	keyPath         string
)

func main() {
	flag.StringVar(&origin, "origin", "", "Required. Source name for metrics emitted by this process, e.g. service-name")
	flag.StringVar(&agentAddr, "agent-addr", "", "Required. Loggregator agent address, e.g. localhost:2346")
	flag.StringVar(&metricsCmd, "metrics-cmd", "", "Required. Path to metrics command")
	flag.StringVar(&caPath, "ca", "", "Required. Path to CA certificate")
	flag.StringVar(&certPath, "cert", "", "Required. Path to client TLS certificate")
	flag.StringVar(&keyPath, "key", "", "Required. Path to client TLS private key")
	flag.Var(&metricsCmdArgs, "metrics-cmd-arg", "Argument to pass on to metrics-cmd (multi-valued)")
	flag.DurationVar(&metricsInterval, "metrics-interval", time.Minute, "Interval to run metrics-cmd")
	flag.BoolVar(&debug, "debug", false, "Output debug logging")

	flag.Parse()

	assertFlag("origin", origin)
	assertFlag("agent-addr", agentAddr)
	assertFlag("metrics-cmd", metricsCmd)
	assertFlag("ca", caPath)
	assertFlag("cert", certPath)
	assertFlag("key", keyPath)

	stdoutLogLevel := lager.INFO
	if debug {
		stdoutLogLevel = lager.DEBUG
	}

	logger := lager.NewLogger("service-metrics")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, stdoutLogLevel))
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.ERROR))

	tlsConfig, err := loggregator.NewIngressTLSConfig(caPath, certPath, keyPath)
	if err != nil {
		logger.Error("Failed to load TLS config", err)
		os.Exit(1)
	}

	loggregatorClient, err := loggregator.NewIngressClient(tlsConfig,
		loggregator.WithAddr(agentAddr),
		loggregator.WithLogger(&logWrapper{logger}),
	)
	if err != nil {
		logger.Error("Failed to initialize loggregator client", err)
		os.Exit(1)
	}

	egressClient := metrics.NewEgressClient(loggregatorClient, origin)

	process(logger, egressClient, metricsCmd, metricsCmdArgs...)
	for {
		select {
		case <-time.After(metricsInterval):
			process(logger, egressClient, metricsCmd, metricsCmdArgs...)
		}
	}
}

type multiFlag []string

func (m *multiFlag) String() string {
	return fmt.Sprint(metricsCmdArgs)
}

func (m *multiFlag) Set(value string) error {
	if metricsCmdArgs == nil {
		metricsCmdArgs = multiFlag{}
	}

	metricsCmdArgs = append(metricsCmdArgs, value)

	return nil
}

func assertFlag(name, value string) {
	if value == "" {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\nMust provide --%s", name)
		os.Exit(1)
	}
}

func process(logger metrics.Logger, egressClient *metrics.EgressClient, metricsCmd string, metricsCmdArgs ...string) {
	action := "executing-metrics-cmd"

	logger.Info(action, lager.Data{
		"event": "starting",
	})

	cmd := exec.Command(metricsCmd, metricsCmdArgs...)
	out, err := cmd.CombinedOutput()

	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			logger.Error(action, err, lager.Data{
				"event":  "failed",
				"output": "no metrics command has been configured, cannot collect metrics",
			})
			os.Exit(1)
		}

		exitStatus := cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
		if exitStatus == 10 {
			logger.Info(action, lager.Data{
				"event":  "not yet ready to emit metrics",
				"output": string(out),
			})
			return
		}

		logger.Error(action, err, lager.Data{
			"event":  "failed",
			"output": string(out),
		})
		os.Exit(0)
	}

	logger.Info(action, lager.Data{
		"event": "done",
	})

	var parsedMetrics metrics.Metrics

	decoder := json.NewDecoder(bytes.NewReader(out))
	err = decoder.Decode(&parsedMetrics)
	if err != nil {
		logger.Error("parsing-metrics-output", err, lager.Data{
			"event":  "failed",
			"output": string(out),
		})
		os.Exit(1)
	}

	egressClient.Emit(parsedMetrics, logger)
}

type logWrapper struct {
	lager.Logger
}

func (l *logWrapper) Printf(f string, a ...interface{}) {
	l.Info(fmt.Sprintf(f, a...))
}
