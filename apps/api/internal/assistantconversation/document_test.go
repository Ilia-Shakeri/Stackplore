package assistantconversation

import (
 "encoding/json"
 "strings"
 "testing"
 "github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestSelectedDocumentIsClosedExplicitAndSameLanguage(t *testing.T) {
 base:=`{"source_id":"source_demo","resource_id":"r1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","window_minutes":15,"locale":"fa","question":"بررسی منبع"`
 selected:=`,"reviewed_document":{"consent":true,"reference":{"corpus_id":"operations","version":2,"digest":"`+strings.Repeat("a",64)+`","document_id":"guide","locale":"fa","generation":0}}}`
 var valid Request
 if json.Unmarshal([]byte(base+selected),&valid)!=nil || valid.Document==nil || !valid.Valid() {t.Fatal("selected exact guide rejected")}
 for _,raw:=range []string{
  strings.Replace(base+selected,`"consent":true`,`"consent":false`,1),
  strings.Replace(base+selected,`"consent":true`,`"consent":null`,1),
  strings.Replace(base+selected,`"consent":true`,`"CONSENT":true`,1),
  strings.Replace(base+selected,`"corpus_id"`,`"CORPUS_ID"`,1),
  strings.Replace(base+selected,`"document_id":"guide"`,`"document_id":"guide","body":"private"`,1),
  strings.Replace(base+selected,`"document_id":"guide"`,`"document_id":"guide","document_id":"other"`,1),
  strings.Replace(base+selected,`"generation":0`,`"generation":null`,1),
  base+strings.Replace(selected,`"locale":"fa"`,`"locale":"en"`,1),
  base+`,"log_consent":true`+selected,
 } {
  var rejected Request
  if err:=json.Unmarshal([]byte(raw),&rejected);err!=assistant.ErrInvalid || rejected.Document!=nil {t.Fatal("widened or implicit document selection accepted",err)}
 }
}
