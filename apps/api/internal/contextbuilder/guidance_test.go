package contextbuilder

import (
 "context"
 "strings"
 "testing"
 "unicode/utf8"
 "github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
)

func TestReviewedGuidanceIsLocalBoundedAndNeverAnObservation(t *testing.T){
 base:=requestFixture(t)
 base.Scope.Resources=base.Scope.Resources[:1]
 ref:=runbook.Reference{CorpusID:"operations",Version:2,Digest:[32]byte{1},DocumentID:"guide",Locale:"fa",Generation:base.Scope.RestoreGeneration}
 content:=runbook.Content{Document:runbook.Document{ID:"guide",Locale:"fa",Title:"راهنمای محلی",Body:strings.Repeat("ظ",200)},Provenance:runbook.Provenance{Kind:"operator_authored",Source:"Synthetic fixture",License:"Fixture permission",UseReviewed:true,AssistanceReviewed:true}}
 request:=Request{Scope:base.Scope,Destination:Local,Status:base.Status,Guidance:&ReviewedGuidance{Reference:ref,Content:content},RecordLimit:1,ByteLimit:8192}
 bundle,err:=Build(context.Background(),request)
 if err!=nil {t.Fatal(err)}
 items,err:=bundle.Items()
 if err!=nil || len(items)!=1 || !utf8.ValidString(items[0].Text) || !bundle.document.Records[0].Guidance.Truncated || len(bundle.document.Records[0].Guidance.Excerpt)!=256 || bundle.Citations()[0].Document!=ref {t.Fatal("guide excerpt or exact reference lost",err)}
 if _,err:=bundle.Observations();err!=ErrInvalid {t.Fatal("guide became live observation",err)}
 request.Destination=Remote
 if _,err:=Build(context.Background(),request);err!=ErrDenied {t.Fatal("guide crossed structured-only boundary",err)}
 request.Destination=Local;request.Guidance.Content.Provenance.AssistanceReviewed=false
 if _,err:=Build(context.Background(),request);err!=ErrDenied {t.Fatal("reference-only rights became processing permission",err)}
}
