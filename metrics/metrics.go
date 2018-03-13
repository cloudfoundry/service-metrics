package metrics

type Metric struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type Metrics []Metric

func (m Metrics) Find(key string) (Metric, bool) {
	for _, metric := range m {
		if metric.Key == key {
			return metric, true
		}
	}

	return Metric{}, false
}
