package assistantplan

import (
	"slices"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestPlansAreFiniteFreshAndRequireExplicitLogConsent(t *testing.T) {
	for _, consent := range []bool{false, true} {
		plan, err := Reads(assistant.InvestigateIncident, consent)
		if err != nil || len(plan) > MaximumReads || len(plan) < 7 || plan[0] != assistant.Inventory || !slices.Contains(plan, assistant.AlertHistory) || !slices.Contains(plan, assistant.WorkloadHistory) || slices.Contains(plan, assistant.Inference) || slices.Contains(plan, assistant.Logs) != consent {
			t.Fatal("investigation escaped its fixed read recipe")
		}
		seen := map[assistant.StepKind]bool{}
		for _, kind := range plan {
			if !kind.Valid() || seen[kind] {
				t.Fatal("unknown or repeated read")
			}
			seen[kind] = true
		}
		plan[0] = assistant.Logs
		again, _ := Reads(assistant.InvestigateIncident, consent)
		if again[0] != assistant.Inventory {
			t.Fatal("caller changed the next plan")
		}
	}
	plain, err := Reads(assistant.ExplainResource, false)
	if err != nil || !slices.Equal(plain, []assistant.StepKind{assistant.Inventory}) {
		t.Fatal("ordinary explanation acquired extra reads")
	}
	for _, goal := range []assistant.Goal{assistant.ExplainResource, assistant.ExplainAlert, "shell", "sql", "http://unapproved.invalid"} {
		if _, err := Reads(goal, true); err != assistant.ErrInvalid {
			t.Fatal("unapproved goal or log access accepted")
		}
	}
}

func TestSelectedDocumentUsesOneReservedReadWithoutRaisingCap(t *testing.T) {
 for _,goal:=range []assistant.Goal{assistant.ExplainResource,assistant.InvestigateIncident} {
  before,err:=Reads(goal,false);if err!=nil {t.Fatal(err)}
  plan,err:=WithDocument(goal,false,true)
  if err!=nil || len(plan)!=len(before)+1 || len(plan)>MaximumReads || plan[len(plan)-1]!=assistant.ReviewedDocument || !slices.Equal(before,plan[:len(plan)-1]) {t.Fatal("selected guide displaced or widened required reads")}
  plan[0]=assistant.Logs
  again,_:=WithDocument(goal,false,true)
  if again[0]!=assistant.Inventory {t.Fatal("guide plan shared caller storage")}
  if _,err:=WithDocument(goal,true,true);err!=assistant.ErrInvalid {t.Fatal("document plus logs accepted")}
 }
}
