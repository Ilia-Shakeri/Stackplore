package httpapi

import "testing"

func TestReadinessStateAndReport(t *testing.T) {
	t.Parallel()
	readiness := NewReadiness()
	if readiness.IsReady() {
		t.Fatal("new readiness is true")
	}
	initial := readiness.Report()
	if initial.State != ReadinessNotReady || len(initial.Checks) != 2 || initial.Checks[1].State != ReadinessNotReady {
		t.Errorf("initial report = %#v", initial)
	}

	readiness.SetReady(true)
	if !readiness.IsReady() {
		t.Fatal("readiness did not become true")
	}
	readyReport := readiness.Report()
	if readyReport.State != ReadinessReady || readyReport.Checks[0].State != ReadinessReady || readyReport.Checks[1].State != ReadinessReady {
		t.Errorf("ready report = %#v", readyReport)
	}

	initial.Checks[0].State = ReadinessNotReady
	if got := readiness.Report().Checks[0].State; got != ReadinessReady {
		t.Errorf("report shares mutable storage: %q", got)
	}
}

func TestNilReadinessStaysNotReady(t *testing.T) {
	t.Parallel()
	var readiness *Readiness
	readiness.SetReady(true)
	if readiness.IsReady() {
		t.Error("nil readiness became ready")
	}
}
