package postgres

import (
 "context"
 "testing"
 "time"
 "github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
 "github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
)

func TestRunbookAssistanceNeedsSeparateReviewAndLiveSession(t *testing.T){
 store,principal,_:=sourceSettingsFixture(t,-2*time.Second)
 ctx:=context.Background()
 actor:=assistant.Actor{UserID:string(principal.UserID),AuthRevision:principal.AuthRevision}
 var digest []byte
 if err:=store.pool.QueryRow(ctx,`SELECT token_digest,token_digest_key_id FROM sessions WHERE id=$1`,string(principal.SessionID)).Scan(&digest,&actor.DigestKeyID);err!=nil || len(digest)!=32 {t.Fatal("read synthetic actor",err)}
 copy(actor.SessionDigest[:],digest)
 first:=runbookCommand(t,store,principal,runbookCandidate(1,runbook.AllUsers),0,0,"manual_scope")
 prepareRunbookTest(t,store,first)
 if _,err:=store.ChangeRunbook(ctx,first);err!=nil {t.Fatal(err)}
 ref:=runbook.Reference{CorpusID:first.Corpus.ID,Version:1,Digest:first.Digest,DocumentID:"restart",Locale:"en"}
 if _,err:=store.ReadRunbook(ctx,principal,ref);err!=nil {t.Fatal("manual read lost",err)}
 if err:=store.AuthorizeAssistantRunbook(ctx,actor,ref);err!=runbook.ErrDenied {t.Fatal("manual approval granted processing",err)}
 if _,err:=store.ReadAssistantRunbook(ctx,actor,ref);err!=runbook.ErrDenied {t.Fatal("reference-only body dispatched",err)}
 corpus:=runbookCandidate(2,runbook.AllUsers);corpus.Provenance.AssistanceReviewed=true
 next:=runbookCommand(t,store,principal,corpus,1,0,"assistance_scope")
 prepareRunbookTest(t,store,next)
 if _,err:=store.ChangeRunbook(ctx,next);err!=nil {t.Fatal(err)}
 ref.Version=2;ref.Digest=next.Digest
 current,err:=store.ReadAssistantRunbook(ctx,actor,ref)
 if err!=nil || !current.Provenance.AssistanceReviewed || current.Document.ID!="restart" {t.Fatal("explicit assistance scope lost",err)}
 if err:=store.AuthorizeAssistantRunbook(ctx,actor,ref);err!=nil {t.Fatal("current metadata denied",err)}
 if _,err:=store.pool.Exec(ctx,`UPDATE runbook_corpora SET provenance_assistance_reviewed=false WHERE id=$1`,ref.CorpusID);err!=nil {t.Fatal(err)}
 if err:=store.AuthorizeAssistantRunbook(ctx,actor,ref);err!=runbook.ErrDenied {t.Fatal("withdrawn processing permission reused",err)}
 if _,err:=store.pool.Exec(ctx,`UPDATE sessions SET revoked_at_us=(extract(epoch FROM clock_timestamp())*1000000)::bigint,revoke_reason_code='session.revoked' WHERE id=$1`,string(principal.SessionID));err!=nil {t.Fatal(err)}
 if _,err:=store.ReadAssistantRunbook(ctx,actor,ref);err!=runbook.ErrDenied {t.Fatal("old session read guide",err)}
}
