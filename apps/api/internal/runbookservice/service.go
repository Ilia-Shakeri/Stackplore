// Package runbookservice coordinates reviewed prose without inference or provider writes.
package runbookservice

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
)

type keyState struct { mu sync.RWMutex; value [32]byte; closed bool }
func safeError(err error)error{for _,owned:=range []error{runbook.ErrInvalid,runbook.ErrDenied,runbook.ErrConflict}{if errors.Is(err,owned){return owned}};return runbook.ErrUnavailable}
type Service struct { repository runbook.Repository; key *keyState; now func()time.Time; lifetime context.Context; stop context.CancelFunc }
func (Service) Format(state fmt.State,_ rune){_,_=io.WriteString(state,"[protected runbook service]")}
func (Service) MarshalJSON()([]byte,error){return []byte("{}"),nil}
func New(repository runbook.Repository,key []byte,now func()time.Time)(*Service,error){
	if repository==nil || len(key)!=32 || now==nil{return nil,runbook.ErrInvalid};var secret [32]byte;copy(secret[:],key);if secret==[32]byte{}{return nil,runbook.ErrInvalid};life,stop:=context.WithCancel(context.Background());return &Service{repository:repository,key:&keyState{value:secret},now:now,lifetime:life,stop:stop},nil
}
func(s *Service)Close(){if s==nil || s.key==nil{return};s.key.mu.Lock();s.key.closed=true;clear(s.key.value[:]);s.key.mu.Unlock();s.stop()}
func(s *Service)open()bool{if s==nil || s.key==nil{return false};s.key.mu.RLock();defer s.key.mu.RUnlock();return !s.key.closed}
func(s *Service)work(ctx context.Context)(context.Context,func(),error){if ctx==nil || ctx.Err()!=nil || !s.open(){return nil,nil,runbook.ErrUnavailable};work,cancel:=context.WithTimeout(ctx,5*time.Second);stop:=context.AfterFunc(s.lifetime,cancel);return work,func(){stop();cancel()},nil}
func(s *Service)mac(parts ...[]byte)[]byte{s.key.mu.RLock();key,closed:=s.key.value,s.key.closed;s.key.mu.RUnlock();defer clear(key[:]);if closed{return nil};mac:=hmac.New(sha256.New,key[:]);for _,part:=range parts{_,_=mac.Write(part)};return mac.Sum(nil)}
func(s *Service)confirmation(token string)([32]byte,error){var digest [32]byte;if _,err:=auth.ParseOpaqueToken(token);err!=nil{return digest,runbook.ErrInvalid};proof:=s.mac([]byte("stackplore-runbook-confirm-v1\x00"),[]byte(token));if len(proof)!=32{return digest,runbook.ErrUnavailable};copy(digest[:],proof);return digest,nil}

type Material struct { Corpus runbook.Corpus; ExpectedVersion int64; Generation int64; Digest [32]byte; Revoke bool }
func(Material)Format(state fmt.State,_ rune){_,_=io.WriteString(state,"[protected runbook material]")}
func(Material)MarshalJSON()([]byte,error){return []byte("{}"),nil}
func(m Material)validate(actor auth.Principal)error{
	if (runbook.Preview{Actor:actor,CorpusID:m.Corpus.ID,ExpectedVersion:m.ExpectedVersion,ExpectedGeneration:m.Generation,Digest:m.Digest,Revoke:m.Revoke,ConfirmationDigest:[32]byte{1}}).Validate()!=nil{return runbook.ErrInvalid}
	if m.Revoke{if m.Corpus.Version!=m.ExpectedVersion || m.Corpus.Audience!="" || len(m.Corpus.Documents)!=0{return runbook.ErrInvalid};return nil}
	digest,err:=m.Corpus.Digest();if err!=nil || m.Corpus.Version!=m.ExpectedVersion+1 || digest!=m.Digest{return runbook.ErrInvalid};return nil
}
type Draft struct { Token string; ExpiresAt time.Time }
func(Draft)Format(state fmt.State,_ rune){_,_=io.WriteString(state,"[protected runbook confirmation]")}
func(Draft)MarshalJSON()([]byte,error){return []byte("{}"),nil}
func(s *Service)Preview(ctx context.Context,actor auth.Principal,m Material)(Draft,error){
	if m.validate(actor)!=nil{return Draft{},runbook.ErrInvalid};work,done,err:=s.work(ctx);if err!=nil{return Draft{},safeError(err)};defer done()
	proof,err:=auth.GenerateOpaqueToken();if err!=nil{return Draft{},runbook.ErrUnavailable};token,err:=proof.Encode();if err!=nil{return Draft{},runbook.ErrUnavailable};digest,err:=s.confirmation(token);if err!=nil{return Draft{},safeError(err)}
	result,err:=s.repository.PreviewRunbook(work,runbook.Preview{Actor:actor,CorpusID:m.Corpus.ID,ExpectedVersion:m.ExpectedVersion,ExpectedGeneration:m.Generation,Digest:m.Digest,Revoke:m.Revoke,ConfirmationDigest:digest})
	at:=s.now().UTC().Truncate(time.Microsecond)
	if err!=nil{return Draft{},safeError(err)};if work.Err()!=nil || !s.open() || !runbook.Timestamp(result.ExpiresAt) || !result.ExpiresAt.After(at) || result.ExpiresAt.Sub(at)>2*time.Minute || result.Current.ID!=m.Corpus.ID || result.Current.Version!=m.ExpectedVersion{return Draft{},runbook.ErrUnavailable};return Draft{Token:token,ExpiresAt:result.ExpiresAt},nil
}
func(s *Service)Commit(ctx context.Context,actor auth.Principal,m Material,token string,confirmed bool,grant auth.RecentGrantUse,requestID domain.RequestID)(runbook.Revision,error){
	if m.validate(actor)!=nil || !confirmed || requestID.Validate()!=nil{return runbook.Revision{},runbook.ErrInvalid};work,done,err:=s.work(ctx);if err!=nil{return runbook.Revision{},safeError(err)};defer done();digest,err:=s.confirmation(token);if err!=nil{return runbook.Revision{},safeError(err)}
	var id [16]byte;if _,err:=rand.Read(id[:]);err!=nil{return runbook.Revision{},runbook.ErrUnavailable}
	action,reason:=domain.AuditAction("runbook.approve"),domain.ReasonCode("runbook.approved");if m.Revoke{action="runbook.revoke";reason="runbook.revoked"}
	c:=runbook.Change{Actor:actor,Grant:grant,Corpus:m.Corpus,ExpectedVersion:m.ExpectedVersion,ExpectedGeneration:m.Generation,Digest:m.Digest,Revoke:m.Revoke,Confirmed:true,ConfirmationDigest:digest,Audit:domain.AuditEntry{ID:domain.AuditEntryID("runbook_"+hex.EncodeToString(id[:])),OccurredAt:grant.ConsumedAt,Actor:domain.AuditActor{Kind:domain.AuditActorUser,ID:string(actor.UserID)},Action:action,Target:domain.AuditTarget{Kind:"runbook.corpus",ID:m.Corpus.ID},Outcome:domain.AuditSuccess,RequestID:requestID,ReasonCode:reason}}
	if c.Validate()!=nil{return runbook.Revision{},runbook.ErrInvalid};result,err:=s.repository.ChangeRunbook(work,c);if err!=nil{return runbook.Revision{},safeError(err)};if work.Err()!=nil || !s.open() || result.Validate()!=nil || result.ID!=m.Corpus.ID || result.Version!=m.Corpus.Version || result.Digest!=m.Digest || result.Generation!=m.Generation || result.Approved==m.Revoke{return runbook.Revision{},runbook.ErrUnavailable};return result,nil
}
func(s *Service)Catalog(ctx context.Context,actor auth.Principal)(runbook.Catalog,error){
	if actor.Validate()!=nil || actor.Role!=auth.RoleAdministrator{return runbook.Catalog{},runbook.ErrDenied};work,done,err:=s.work(ctx);if err!=nil{return runbook.Catalog{},safeError(err)};defer done();result,err:=s.repository.InspectRunbooks(work,actor);if err!=nil{return runbook.Catalog{},safeError(err)}
	if work.Err()!=nil || !s.open() || result.Generation<0 || result.Version<1 || result.Version>runbook.MaximumVersion || len(result.Revisions)>runbook.MaximumCorpora{return runbook.Catalog{},runbook.ErrUnavailable}
	for i,item:=range result.Revisions{if item.Validate()!=nil || i>0 && result.Revisions[i-1].ID>=item.ID{return runbook.Catalog{},runbook.ErrUnavailable}};result.Revisions=append([]runbook.Revision{},result.Revisions...);return result,nil
}

const MaximumCursorBytes=768
type cursor struct{ Anchor int64 `json:"a"`; Generation int64 `json:"g"`; Catalog int64 `json:"v"`; Corpus string `json:"c"`; Document string `json:"d"` }
type Results struct{ Hits []runbook.Hit; Cursor string }
func(Results)Format(state fmt.State,_ rune){_,_=io.WriteString(state,"[protected runbook results]")}
func(Results)MarshalJSON()([]byte,error){return []byte("{}"),nil}
func(s *Service)signature(q runbook.Search,body []byte)[]byte{query:=sha256.Sum256([]byte(q.Query));return s.mac([]byte("stackplore-runbook-page-v1\x00"+string(q.Actor.UserID)+"\x00"+string(q.Actor.SessionID)+"\x00"+string(q.Actor.Role)+"\x00"+strconv.FormatInt(q.Actor.AuthRevision,10)+"\x00"+q.Locale+"\x00"+strconv.Itoa(q.Limit)+"\x00"),query[:],body)}
func(s *Service)Search(ctx context.Context,actor auth.Principal,query,locale string,limit int,encoded string)(Results,error){
	q:=runbook.Search{Actor:actor,Query:query,Locale:locale,Limit:limit,Generation:-1,CatalogVersion:-1};if q.Validate()!=nil || len(encoded)>MaximumCursorBytes{return Results{},runbook.ErrInvalid};work,done,err:=s.work(ctx);if err!=nil{return Results{},safeError(err)};defer done();at:=s.now().UTC().Truncate(time.Microsecond);if !runbook.Timestamp(at){return Results{},runbook.ErrUnavailable};anchor:=at.UnixMicro()
	if encoded!=""{parts:=strings.Split(encoded,".");if len(parts)!=2{return Results{},runbook.ErrInvalid};body,err:=base64.RawURLEncoding.Strict().DecodeString(parts[0]);if err!=nil{return Results{},runbook.ErrInvalid};defer clear(body);proof,err:=base64.RawURLEncoding.Strict().DecodeString(parts[1]);if err!=nil || len(proof)!=32 || !hmac.Equal(proof,s.signature(q,body)){return Results{},runbook.ErrInvalid};var value cursor;if json.Unmarshal(body,&value)!=nil{return Results{},runbook.ErrInvalid};canonical,err:=json.Marshal(value);defer clear(canonical);if err!=nil || !bytes.Equal(body,canonical) || value.Anchor<0 || value.Anchor>anchor || anchor-value.Anchor>=int64((10*time.Minute)/time.Microsecond) || value.Generation<0 || value.Catalog<1 || !runbook.ValidID(value.Corpus) || !runbook.ValidID(value.Document){return Results{},runbook.ErrInvalid};anchor=value.Anchor;q.Generation=value.Generation;q.CatalogVersion=value.Catalog;q.BeforeCorpus=value.Corpus;q.BeforeDocument=value.Document}
	page,err:=s.repository.SearchRunbooks(work,q);if err!=nil{return Results{},safeError(err)};if work.Err()!=nil || !s.open() || page.Generation<0 || page.CatalogVersion<1 || len(page.Hits)>limit || page.HasMore && len(page.Hits)!=limit || q.Generation>=0 && (q.Generation!=page.Generation || q.CatalogVersion!=page.CatalogVersion){return Results{},runbook.ErrUnavailable}
	for i,hit:=range page.Hits{ref:=hit.Reference;if hit.Validate()!=nil || ref.Locale!=locale || ref.Generation!=page.Generation || q.BeforeCorpus!="" && (ref.CorpusID<q.BeforeCorpus || ref.CorpusID==q.BeforeCorpus && ref.DocumentID<=q.BeforeDocument) {return Results{},runbook.ErrUnavailable};if i>0{prior:=page.Hits[i-1].Reference;if prior.CorpusID>ref.CorpusID || prior.CorpusID==ref.CorpusID && prior.DocumentID>=ref.DocumentID{return Results{},runbook.ErrUnavailable}}}
	result:=Results{Hits:append([]runbook.Hit{},page.Hits...)};if page.HasMore{last:=result.Hits[len(result.Hits)-1].Reference;body,err:=json.Marshal(cursor{Anchor:anchor,Generation:page.Generation,Catalog:page.CatalogVersion,Corpus:last.CorpusID,Document:last.DocumentID});if err!=nil{return Results{},runbook.ErrUnavailable};defer clear(body);proof:=s.signature(q,body);if len(proof)!=32{return Results{},runbook.ErrUnavailable};result.Cursor=base64.RawURLEncoding.EncodeToString(body)+"."+base64.RawURLEncoding.EncodeToString(proof)};if !s.open() || work.Err()!=nil{return Results{},runbook.ErrUnavailable};return result,nil
}
func(s *Service)Read(ctx context.Context,actor auth.Principal,reference runbook.Reference)(runbook.Document,error){if actor.Validate()!=nil || reference.Validate()!=nil{return runbook.Document{},runbook.ErrInvalid};work,done,err:=s.work(ctx);if err!=nil{return runbook.Document{},safeError(err)};defer done();document,err:=s.repository.ReadRunbook(work,actor,reference);if err!=nil{return runbook.Document{},safeError(err)};if work.Err()!=nil || !s.open() || document.Validate()!=nil || document.ID!=reference.DocumentID || document.Locale!=reference.Locale{return runbook.Document{},runbook.ErrUnavailable};return document,nil}
