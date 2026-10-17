package domain

import "testing"

func reviewScaleRequest(t *testing.T) OperationRequest {
	t.Helper()
	identity := fixtureIdentity("operation_native")
	identity.Provider = ProviderKubernetes
	identity.SourceID = "source_kubernetes"
	identity.Kind = "kubernetes.deployment"
	identity.Scope = Scope{Kind: ScopeNamespace, Name: "shop"}
	resource, err := NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	replicas := 3
	return OperationRequest{Source: ProviderRef{Provider: ProviderKubernetes, SourceID: "source_kubernetes"}, Resource: resource, ExpectedVersion: "version_1", Action: OperationKubernetesScale, Parameters: OperationParameters{Replicas: &replicas}}
}

func TestOperationImpactAndReceiptBindExactAcceptedEffect(t *testing.T) {
	request := reviewScaleRequest(t)
	current := 2
	impact := OperationImpact{Request: request, DisplayName: "Shop", EffectCode: "operation.scale", RiskCode: "operation.availability_risk", CurrentReplicas: &current}
	if err := impact.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*OperationImpact){
		"missing request": func(i *OperationImpact) { i.Request.Source.SourceID = "" },
		"unsafe name":     func(i *OperationImpact) { i.DisplayName = "\x00" },
		"unsafe effect":   func(i *OperationImpact) { i.EffectCode = "plain" },
		"unsafe risk":     func(i *OperationImpact) { i.RiskCode = "plain" },
		"missing count":   func(i *OperationImpact) { i.CurrentReplicas = nil },
		"negative count":  func(i *OperationImpact) { n := -1; i.CurrentReplicas = &n },
		"count over cap":  func(i *OperationImpact) { n := MaxOperationReplicas + 1; i.CurrentReplicas = &n },
		"restart with count": func(i *OperationImpact) {
			i.Request.Action = OperationKubernetesRestart
			i.Request.Parameters = OperationParameters{}
		},
	} {
		t.Run(name, func(t *testing.T) { i := impact; change(&i); requireInvalid(t, i.Validate()) })
	}
	execution := OperationExecution{Request: request, OperationID: "operation_review"}
	result := OperationProviderResult{OperationID: execution.OperationID, Action: request.Action, ResourceID: request.Resource.ID, PreviousVersion: request.ExpectedVersion, CurrentVersion: "version_2", EffectCode: "operation.scaled"}
	if err := result.ValidateFor(execution); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*OperationProviderResult){
		"other operation":        func(r *OperationProviderResult) { r.OperationID = "operation_other" },
		"other action":           func(r *OperationProviderResult) { r.Action = OperationKubernetesRestart },
		"other resource":         func(r *OperationProviderResult) { r.ResourceID = fixtureResourceRef(t, "other").ID },
		"wrong prior version":    func(r *OperationProviderResult) { r.PreviousVersion = "version_other" },
		"missing result version": func(r *OperationProviderResult) { r.CurrentVersion = "" },
		"unsafe effect":          func(r *OperationProviderResult) { r.EffectCode = "plain" },
	} {
		t.Run(name, func(t *testing.T) { r := result; change(&r); requireInvalid(t, r.ValidateFor(execution)) })
	}
	execution.OperationID = ""
	requireInvalid(t, result.ValidateFor(execution))
	execution.Request.Source.SourceID = ""
	requireInvalid(t, execution.Validate())
}

func TestNotificationPagesNeverHideDuplicatesOrOrderingGaps(t *testing.T) {
	source := SourceID("source_demo")
	first := Notification{ID: "notice_a", SourceID: &source, Kind: "notification.test", Severity: SeverityInfo, SummaryCode: "notification.ready", CreatedAt: fixtureTime(), Synthetic: true}
	second := first
	second.ID = "notice_b"
	page := NotificationPage{Notifications: []Notification{first, second}, Page: fixturePage(2)}
	if err := page.ValidateFor(PageRequest{Limit: 2}); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*NotificationPage){
		"duplicate": func(p *NotificationPage) { p.Notifications[1] = p.Notifications[0] },
		"reverse": func(p *NotificationPage) {
			p.Notifications[0], p.Notifications[1] = p.Notifications[1], p.Notifications[0]
		},
		"invalid row": func(p *NotificationPage) { p.Notifications[0].Kind = "plain" },
		"over limit":  func(p *NotificationPage) { p.Notifications = append(p.Notifications, second) },
		"bad page":    func(p *NotificationPage) { p.Page.Limit = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			p := page
			p.Notifications = append([]Notification(nil), page.Notifications...)
			change(&p)
			requireInvalid(t, p.ValidateFor(PageRequest{Limit: 2}))
		})
	}
}
