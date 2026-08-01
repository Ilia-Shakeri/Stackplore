package httpapi

import "sync/atomic"

type ReadinessState string

const (
	ReadinessReady    ReadinessState = "ready"
	ReadinessNotReady ReadinessState = "not_ready"
)

type ReadinessCheck struct {
	Name  string         `json:"name"`
	State ReadinessState `json:"state"`
}

type ReadinessReport struct {
	State  ReadinessState   `json:"state"`
	Checks []ReadinessCheck `json:"checks"`
}

type Readiness struct {
	ready atomic.Bool
}

func NewReadiness() *Readiness {
	return &Readiness{}
}

func (readiness *Readiness) SetReady(ready bool) {
	if readiness == nil {
		return
	}
	readiness.ready.Store(ready)
}

func (readiness *Readiness) IsReady() bool {
	return readiness != nil && readiness.ready.Load()
}

func (readiness *Readiness) Report() ReadinessReport {
	serviceState := ReadinessNotReady
	if readiness.IsReady() {
		serviceState = ReadinessReady
	}
	return ReadinessReport{
		State: serviceState,
		Checks: []ReadinessCheck{
			{Name: "configuration", State: ReadinessReady},
			{Name: "service", State: serviceState},
		},
	}
}
