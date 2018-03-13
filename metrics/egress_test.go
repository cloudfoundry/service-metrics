package metrics_test

import (
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/service-metrics/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Egress", func() {
	var (
		m   metrics.Metrics
		l   *spyLogger
		in  *spyIngressClient
		c   *metrics.EgressClient
		env *loggregator_v2.Envelope
	)

	BeforeEach(func() {
		m = metrics.Metrics{
			{
				Key:   "metric-1",
				Value: 0.1,
				Unit:  "s",
			},
			{
				Key:   "metric-2",
				Value: 1.3,
				Unit:  "s",
			},
		}
		env = &loggregator_v2.Envelope{
			Timestamp: time.Now().UnixNano(),
			Message: &loggregator_v2.Envelope_Gauge{
				Gauge: &loggregator_v2.Gauge{
					Metrics: make(map[string]*loggregator_v2.GaugeValue),
				},
			},
			Tags: make(map[string]string),
		}

		l = newSpyLogger()
		in = newSpyIngressClient()
		c = metrics.NewEgressClient(in, "source-1")
	})

	Context("Emit", func() {
		It("passes logs to IngressClient", func() {
			c.SetInstanceID(3)

			c.Emit(m, l)

			Expect(in.emitCalled).To(BeTrue())

			for _, o := range in.opts {
				o(env)
			}

			Expect(env.GetGauge().Metrics).To(HaveKeyWithValue("metric-2",
				&loggregator_v2.GaugeValue{Value: 1.3, Unit: "s"}),
			)
			Expect(env.GetGauge().Metrics).To(HaveKeyWithValue("metric-1",
				&loggregator_v2.GaugeValue{Value: 0.1, Unit: "s"}),
			)
			Expect(env.SourceId).To(Equal("source-1"))
			Expect(env.InstanceId).To(Equal("3"))
			Expect(l.infoKey).To(Equal("sending-metrics"))
		})

		It("logs an error when source ID is not specified", func() {
			c = metrics.NewEgressClient(in, "")
			c.Emit(m, l)

			Expect(l.errAction).To(Equal("sending metrics failed"))
			Expect(l.errData).To(ConsistOf(
				lager.Data{
					"Emit": "failed",
				},
			))
			Expect(l.err).To(MatchError("You must set a source ID"))
		})
	})
})

type spyIngressClient struct {
	emitCalled bool
	opts       []loggregator.EmitGaugeOption
	env        *loggregator_v2.Envelope
}

func newSpyIngressClient() *spyIngressClient {
	return &spyIngressClient{}
}

func (s *spyIngressClient) EmitGauge(opts ...loggregator.EmitGaugeOption) {
	s.emitCalled = true
	s.opts = opts
}

type spyLogger struct {
	// map of action strings to slice of data called against the actions
	infoKey   string
	infoData  []lager.Data
	errAction string
	errData   []lager.Data
	err       error
	errCalled bool
}

func newSpyLogger() *spyLogger {
	return &spyLogger{}
}
func (l *spyLogger) Info(action string, data ...lager.Data) {
	l.infoKey = action
	l.infoData = data
}

func (l *spyLogger) Error(action string, err error, data ...lager.Data) {
	l.errAction = action
	l.errData = data
	l.err = err
	l.errCalled = true
}
