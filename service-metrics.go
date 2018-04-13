package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/service-metrics/metrics"
)

var (
	origin          string
	sourceID        string
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
	flag.StringVar(&sourceID, "source-id", "", "Source ID to be applied to all envelopes.")
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

	if sourceID == "" {
		sourceID = origin
	}

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
		loggregator.WithTag("origin", origin),
	)
	if err != nil {
		logger.Error("Failed to initialize loggregator client", err)
		os.Exit(1)
	}

	egressClient := metrics.NewEgressClient(loggregatorClient, sourceID)
	processor := metrics.NewProcessor(
		logger,
		egressClient,
		NewCommandLineExecutor(logger),
	)

	processor.Process(metricsCmd, metricsCmdArgs...)
	for {
		select {
		case <-time.After(metricsInterval):
			processor.Process(metricsCmd, metricsCmdArgs...)
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

type logWrapper struct {
	lager.Logger
}

func (l *logWrapper) Printf(f string, a ...interface{}) {
	l.Info(fmt.Sprintf(f, a...))
}
