package httpapi

import "testing"

func TestReadinessStateAndReport(t *testing.T) {
	t.Parallel()
	readiness := NewReadiness()
	if readiness.IsReady() {
		t.Fatal("new readiness is true")
	}
	initial := readiness.Report()
	if initial.State != ReadinessNotReady || len(initial.Checks) != 3 || initial.Checks[1].State != ReadinessNotReady || initial.Checks[2].State != ReadinessNotReady {
		t.Errorf("initial report = %#v", initial)
	}

	readiness.SetDatabaseReady(true)
	if readiness.IsReady() {
		t.Fatal("database readiness alone made service ready")
	}
	readiness.SetServiceReady(true)
	if !readiness.IsReady() {
		t.Fatal("readiness did not become true")
	}
	readyReport := readiness.Report()
	if readyReport.State != ReadinessReady || readyReport.Checks[0].State != ReadinessReady || readyReport.Checks[1].State != ReadinessReady || readyReport.Checks[2].State != ReadinessReady {
		t.Errorf("ready report = %#v", readyReport)
	}

	readiness.SetDatabaseReady(false)
	if readiness.IsReady() || readiness.Report().Checks[2].State != ReadinessReady {
		t.Fatal("database loss did not preserve service state while making readiness false")
	}

	initial.Checks[0].State = ReadinessNotReady
	if got := readiness.Report().Checks[0].State; got != ReadinessReady {
		t.Errorf("report shares mutable storage: %q", got)
	}
}

func TestNilReadinessStaysNotReady(t *testing.T) {
	t.Parallel()
	var readiness *Readiness
	readiness.SetDatabaseReady(true)
	readiness.SetServiceReady(true)
	if readiness.IsReady() {
		t.Error("nil readiness became ready")
	}
	if report := readiness.Report(); report.State != ReadinessNotReady || len(report.Checks) != 3 {
		t.Errorf("nil readiness report = %#v", report)
	}
}
