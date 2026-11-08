package runbookservice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
)

type repository struct {
	preview func(context.Context,runbook.Preview)(runbook.PreviewResult,error)
	change func(context.Context,runbook.Change)(runbook.Revision,error)
	search func(context.Context,runbook.Search)(runbook.Page,error)
	read func(context.Context,auth.Principal,runbook.Reference)(runbook.Document,error)
	catalog func(context.Context,auth.Principal)(runbook.Catalog,error)
}
func(r *repository)PreviewRunbook(ctx context.Context,p runbook.Preview)(runbook.PreviewResult,error){return r.preview(ctx,p)}
func(r *repository)ChangeRunbook(ctx context.Context,c runbook.Change)(runbook.Revision,error){return r.change(ctx,c)}
func(r *repository)SearchRunbooks(ctx context.Context,q runbook.Search)(runbook.Page,error){return r.search(ctx,q)}
func(r *repository)ReadRunbook(ctx context.Context,a auth.Principal,ref runbook.Reference)(runbook.Document,error){return r.read(ctx,a,ref)}
func(r *repository)InspectRunbooks(ctx context.Context,a auth.Principal)(runbook.Catalog,error){return r.catalog(ctx,a)}

func fixture(t *testing.T)(*Service,*repository,auth.Principal,Material,*time.Time){
	t.Helper();at:=time.Date(2026,11,8,10,0,0,0,time.UTC);actor:=auth.Principal{UserID:"user_document",SessionID:"session_document",Role:auth.RoleAdministrator,AuthRevision:1}
	corpus:=runbook.Corpus{ID:"operations",Version:1,Audience:runbook.AllUsers,Documents:[]runbook.Document{{ID:"a",Locale:"en",Title:"Service recovery",Body:"Read service state."}}};digest,err:=corpus.Digest();if err!=nil{t.Fatal(err)}
	repo:=&repository{};service,err:=New(repo,append([]byte{1},make([]byte,31)...),func()time.Time{return at});if err!=nil{t.Fatal(err)};t.Cleanup(service.Close);return service,repo,actor,Material{Corpus:corpus,Digest:digest},&at
}

func TestPreviewBindsProtectedMaterialAndCommitRequiresGrant(t *testing.T){
	s,repo,actor,m,at:=fixture(t);var reviewed runbook.Preview
	repo.preview=func(ctx context.Context,p runbook.Preview)(runbook.PreviewResult,error){if _,ok:=ctx.Deadline();!ok{t.Fatal("unbounded preview")};reviewed=p;return runbook.PreviewResult{Current:runbook.Revision{ID:m.Corpus.ID},ExpiresAt:at.Add(time.Minute)},nil}
	draft,err:=s.Preview(context.Background(),actor,m);if err!=nil || draft.Token=="" || reviewed.Digest!=m.Digest || reviewed.ConfirmationDigest==[32]byte{}{t.Fatal("preview lost material",err)}
	digest,err:=auth.ParseRecentAuthDigest(append([]byte{1},make([]byte,31)...));if err!=nil{t.Fatal(err)}
	grant:=auth.RecentGrantUse{Actor:actor,Purpose:auth.RecentAuthPurposeRunbookManage,Digest:digest,DigestKeyID:"key",ConsumedAt:*at}
	calls:=0;repo.change=func(_ context.Context,c runbook.Change)(runbook.Revision,error){calls++;if c.Validate()!=nil || c.ConfirmationDigest!=reviewed.ConfirmationDigest || c.Digest!=reviewed.Digest{t.Fatal("commit detached review identity")};return runbook.Revision{ID:m.Corpus.ID,Version:1,Digest:m.Digest,Audience:runbook.AllUsers,Approved:true,Documents:1,Bytes:100},nil}
	if _,err:=s.Commit(context.Background(),actor,m,draft.Token,false,grant,"request_doc");err!=runbook.ErrInvalid{t.Fatal("confirmation bypass")}
	bad:=grant;bad.Purpose=auth.RecentAuthPurposeSettingsUpdate
	if _,err:=s.Commit(context.Background(),actor,m,draft.Token,true,bad,"request_doc");err!=runbook.ErrInvalid{t.Fatal("foreign recent purpose")}
	if calls!=0{t.Fatal("invalid commit reached repository")}
	if _,err:=s.Commit(context.Background(),actor,m,draft.Token,true,grant,"request_doc");err!=nil || calls!=1{t.Fatal("valid commit",err)}
	for _,value:=range []any{*s,m,draft,Results{}}{encoded,err:=json.Marshal(value);if err!=nil || string(encoded)!="{}" || strings.Contains(fmt.Sprintf("%+v",value),draft.Token){t.Fatal("ordinary formatting exposed material")}}
}

func TestCursorBindsActorQueryLocalePageAndLifetimeBeforeIO(t *testing.T){
	s,repo,actor,m,at:=fixture(t);calls:=0
	repo.search=func(ctx context.Context,q runbook.Search)(runbook.Page,error){calls++;if _,ok:=ctx.Deadline();!ok{t.Fatal("unbounded search")};id:="a";more:=true;if q.BeforeDocument!=""{if q.BeforeDocument!="a" || q.Generation!=2 || q.CatalogVersion!=3{t.Fatal("lost keyset scope")};id="b";more=false};return runbook.Page{Generation:2,CatalogVersion:3,HasMore:more,Hits:[]runbook.Hit{{Reference:runbook.Reference{CorpusID:"operations",DocumentID:id,Locale:"en",Version:1,Digest:m.Digest,Generation:2},Title:"Service",Snippet:"Service state"}}},nil}
	first,err:=s.Search(context.Background(),actor,"private search words","en",1,"");if err!=nil || first.Cursor==""{t.Fatal("first page",err)}
	body,err:=base64.RawURLEncoding.DecodeString(strings.Split(first.Cursor,".")[0]);if err!=nil || strings.Contains(string(body),"private") || strings.Contains(string(body),string(actor.UserID)){t.Fatal("cursor exposed query or owner")}
	if _,err:=s.Search(context.Background(),actor,"private search words","en",1,first.Cursor);err!=nil{t.Fatal("next page",err)}
	for _,change:=range []func(*auth.Principal){func(a *auth.Principal){a.UserID="other"},func(a *auth.Principal){a.SessionID="other"},func(a *auth.Principal){a.AuthRevision++},func(a *auth.Principal){a.Role=auth.RoleViewer}}{changed:=actor;change(&changed);if _,err:=s.Search(context.Background(),changed,"private search words","en",1,first.Cursor);err!=runbook.ErrInvalid{t.Fatal("changed identity accepted",err)}}
	for _,test:=range []struct{query,locale,cursor string;limit int}{{"changed","en",first.Cursor,1},{"private search words","fa",first.Cursor,1},{"private search words","en",first.Cursor,2},{"private search words","en","bad",1},{"private search words","en",strings.Repeat("x",769),1}}{if _,err:=s.Search(context.Background(),actor,test.query,test.locale,test.limit,test.cursor);err!=runbook.ErrInvalid{t.Fatal("changed cursor scope accepted",err)}}
	*at=at.Add(10*time.Minute);if _,err:=s.Search(context.Background(),actor,"private search words","en",1,first.Cursor);err!=runbook.ErrInvalid{t.Fatal("expired cursor accepted")};if calls!=2{t.Fatal("invalid cursor performed I/O")}
}

func TestReadShutdownMalformedResultsAndSafeErrors(t *testing.T){
	s,repo,actor,m,_:=fixture(t);reference:=runbook.Reference{CorpusID:"operations",Version:1,Digest:m.Digest,DocumentID:"a",Locale:"en"}
	repo.read=func(context.Context,auth.Principal,runbook.Reference)(runbook.Document,error){return m.Corpus.Documents[0],nil}
	if _,err:=s.Read(context.Background(),actor,reference);err!=nil{t.Fatal("current document read",err)}
	repo.read=func(context.Context,auth.Principal,runbook.Reference)(runbook.Document,error){d:=m.Corpus.Documents[0];d.ID="other";return d,nil}
	if _,err:=s.Read(context.Background(),actor,reference);err!=runbook.ErrUnavailable{t.Fatal("foreign document projected")}
	repo.catalog=func(context.Context,auth.Principal)(runbook.Catalog,error){return runbook.Catalog{},errors.New("private diagnostic")}
	if _,err:=s.Catalog(context.Background(),actor);err!=runbook.ErrUnavailable{t.Fatal("raw repository error escaped")}
	repo.read=func(ctx context.Context,_ auth.Principal,_ runbook.Reference)(runbook.Document,error){s.Close();select{case <-ctx.Done():case <-time.After(time.Second):t.Fatal("shutdown did not cancel read")};return m.Corpus.Documents[0],nil}
	if _,err:=s.Read(context.Background(),actor,reference);err!=runbook.ErrUnavailable{t.Fatal("late document escaped shutdown")};if s.key.value!=[32]byte{}{t.Fatal("cursor key retained")}
	if _,err:=s.Read(context.Background(),actor,reference);err!=runbook.ErrUnavailable{t.Fatal("closed service read")}
	if _,err:=New(nil,make([]byte,32),time.Now);err!=runbook.ErrInvalid{t.Fatal("missing repository")};if _,err:=New(repo,make([]byte,32),time.Now);err!=runbook.ErrInvalid{t.Fatal("zero key")}
}
