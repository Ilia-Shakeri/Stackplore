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
	databaseReady atomic.Bool
	serviceReady  atomic.Bool
}

func NewReadiness() *Readiness {
	return &Readiness{}
}

func (readiness *Readiness) SetDatabaseReady(ready bool) {
	if readiness == nil {
		return
	}
	readiness.databaseReady.Store(ready)
}

func (readiness *Readiness) SetServiceReady(ready bool) {
	if readiness == nil {
		return
	}
	readiness.serviceReady.Store(ready)
}

func (readiness *Readiness) IsReady() bool {
	return readiness != nil && readiness.databaseReady.Load() && readiness.serviceReady.Load()
}

func (readiness *Readiness) Report() ReadinessReport {
	databaseState := readinessState(readiness != nil && readiness.databaseReady.Load())
	serviceState := readinessState(readiness != nil && readiness.serviceReady.Load())
	return ReadinessReport{
		State: readinessState(readiness != nil && readiness.IsReady()),
		Checks: []ReadinessCheck{
			{Name: "configuration", State: ReadinessReady},
			{Name: "database", State: databaseState},
			{Name: "service", State: serviceState},
		},
	}
}

func readinessState(ready bool) ReadinessState {
	if ready {
		return ReadinessReady
	}
	return ReadinessNotReady
}
