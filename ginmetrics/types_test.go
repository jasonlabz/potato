package ginmetrics

import "testing"

func TestAddSummaryMetric(t *testing.T) {
	m := GetMonitor()
	// monitor 与 prometheus registry 是全局的，-count>1 时只注册一次。
	if m.GetMetric("potato_test_summary").Name == "" {
		err := m.AddMetric(&Metric{
			Type:       Summary,
			Name:       "potato_test_summary",
			Labels:     []string{"k"},
			Objectives: map[float64]float64{0.5: 0.05},
		})
		if err != nil {
			t.Fatalf("AddMetric() error = %v", err)
		}
	}
	if err := m.GetMetric("potato_test_summary").Observe([]string{"v"}, 1); err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
}
