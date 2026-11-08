package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
	"github.com/jackc/pgx/v5"
)

var _ runbook.Repository = (*Store)(nil)

func (store *Store) PreviewRunbook(ctx context.Context,p runbook.Preview) (runbook.Revision,error) {
	if ctx==nil || p.Validate()!=nil{return runbook.Revision{},runbook.ErrInvalid}
	ctx,cancel:=context.WithTimeout(ctx,3*time.Second);defer cancel();current:=runbook.Revision{ID:p.CorpusID}
	err:=store.withinAuthTransaction(ctx,func(ctx context.Context,transaction *transaction) error {
		tx:=transaction.transaction;if err:=runbookLive(ctx,tx,p.Actor,true);err!=nil{return err}
		generation,_,err:=runbookScope(ctx,tx,true);if err!=nil{return err};if generation!=p.ExpectedGeneration{return runbook.ErrConflict}
		var digest []byte
		err=tx.QueryRow(ctx,`SELECT version,digest,audience,approved,restore_generation,document_count,content_bytes FROM runbook_corpora WHERE id=$1`,p.CorpusID).Scan(&current.Version,&digest,&current.Audience,&current.Approved,&current.Generation,&current.Documents,&current.Bytes)
		if err!=nil && !errors.Is(err,pgx.ErrNoRows){return err};if err==nil && len(digest)!=32{return runbook.ErrUnavailable};copy(current.Digest[:],digest)
		if current.Version!=p.ExpectedVersion || p.Revoke && (!current.Approved || current.Digest!=p.Digest){return runbook.ErrConflict}
		if current.Version==0{var count int;if err:=tx.QueryRow(ctx,`SELECT count(*) FROM runbook_corpora`).Scan(&count);err!=nil{return err};if count>=runbook.MaximumCorpora{return runbook.ErrConflict}}
		if _,err:=tx.Exec(ctx,`DELETE FROM runbook_confirmations WHERE session_id IN (SELECT session_id FROM runbook_confirmations WHERE expires_at_us<=(extract(epoch FROM clock_timestamp())*1000000)::bigint ORDER BY expires_at_us,session_id LIMIT 100)`);err!=nil{return err}
		_,err=tx.Exec(ctx,`INSERT INTO runbook_confirmations(session_id,user_id,corpus_id,expected_version,restore_generation,candidate_digest,revoke,token_digest,created_at_us,expires_at_us)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,(extract(epoch FROM statement_timestamp())*1000000)::bigint,(extract(epoch FROM statement_timestamp())*1000000)::bigint+120000000)
ON CONFLICT(session_id) DO UPDATE SET user_id=EXCLUDED.user_id,corpus_id=EXCLUDED.corpus_id,expected_version=EXCLUDED.expected_version,restore_generation=EXCLUDED.restore_generation,candidate_digest=EXCLUDED.candidate_digest,revoke=EXCLUDED.revoke,token_digest=EXCLUDED.token_digest,created_at_us=EXCLUDED.created_at_us,expires_at_us=EXCLUDED.expires_at_us`,string(p.Actor.SessionID),string(p.Actor.UserID),p.CorpusID,p.ExpectedVersion,generation,p.Digest[:],p.Revoke,p.ConfirmationDigest[:])
		if err!=nil{return err};return runbookLive(ctx,tx,p.Actor,true)
	});if err!=nil{return runbook.Revision{},runbookError(err)};return current,nil
}

func runbookError(err error) error {
	for _, owned := range []error{runbook.ErrInvalid,runbook.ErrConflict,runbook.ErrDenied} { if errors.Is(err,owned) { return owned } }
	if errors.Is(err,auth.ErrAuthRepositoryRejected) || errors.Is(err,auth.ErrAuthorizationDenied) || errors.Is(err,auth.ErrRecentAuthRejected) { return runbook.ErrDenied }
	if err!=nil { return runbook.ErrUnavailable }; return nil
}

func runbookLive(ctx context.Context,tx pgx.Tx,actor auth.Principal,administrator bool) error {
	if actor.Validate()!=nil || administrator && actor.Role!=auth.RoleAdministrator { return runbook.ErrDenied }
	var marker int
	err:=tx.QueryRow(ctx,`SELECT 1 FROM users u JOIN sessions s ON s.user_id=u.id
WHERE u.id=$1 AND u.role=$2 AND u.auth_revision=$3 AND u.enabled=1 AND u.deleted_at_us IS NULL
AND s.id=$4 AND s.user_auth_revision=u.auth_revision AND s.revoked_at_us IS NULL
AND s.created_at_us<=(extract(epoch FROM clock_timestamp())*1000000)::bigint
AND s.last_seen_at_us<=(extract(epoch FROM clock_timestamp())*1000000)::bigint
AND s.idle_expires_at_us>(extract(epoch FROM clock_timestamp())*1000000)::bigint
AND s.absolute_expires_at_us>(extract(epoch FROM clock_timestamp())*1000000)::bigint
FOR SHARE OF u,s`,string(actor.UserID),string(actor.Role),actor.AuthRevision,string(actor.SessionID)).Scan(&marker)
	if errors.Is(err,pgx.ErrNoRows) { return runbook.ErrDenied }; return err
}

func runbookScope(ctx context.Context,tx pgx.Tx,write bool) (int64,int64,error) {
	var generation,version int64
	if err:=tx.QueryRow(ctx,`SELECT restore_generation FROM installation_metadata WHERE singleton_id=1 FOR SHARE`).Scan(&generation); err!=nil { return 0,0,err }
	query:=`SELECT row_version FROM runbook_catalog WHERE singleton=1 FOR SHARE`
	if write { query=`SELECT row_version FROM runbook_catalog WHERE singleton=1 FOR UPDATE` }
	err:=tx.QueryRow(ctx,query).Scan(&version)
	if err==nil && (generation<0 || version<1) { err=runbook.ErrUnavailable };return generation,version,err
}

func (store *Store) ChangeRunbook(ctx context.Context,c runbook.Change) (runbook.Revision,error) {
	c.Corpus.Documents=append([]runbook.Document(nil),c.Corpus.Documents...)
	defer clear(c.Corpus.Documents)
	if ctx==nil || c.Validate()!=nil || !validRecentGrantUse(c.Grant,auth.RecentAuthPurposeRunbookManage,c.Grant.ConsumedAt) { return runbook.Revision{},runbook.ErrInvalid }
	ctx,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel()
	var result runbook.Revision
	err:=store.withinAuthTransaction(ctx,func(ctx context.Context,transaction *transaction) error {
		tx:=transaction.transaction
		if err:=runbookLive(ctx,tx,c.Actor,true);err!=nil{return err}
		generation,catalog,err:=runbookScope(ctx,tx,true);if err!=nil{return err}
		if generation!=c.ExpectedGeneration || catalog>=runbook.MaximumVersion {return runbook.ErrConflict}
		current:=runbook.Revision{ID:c.Corpus.ID}
		var digest []byte
		err=tx.QueryRow(ctx,`SELECT version,digest,audience,approved,restore_generation,document_count,content_bytes FROM runbook_corpora WHERE id=$1 FOR UPDATE`,c.Corpus.ID).Scan(&current.Version,&digest,&current.Audience,&current.Approved,&current.Generation,&current.Documents,&current.Bytes)
		if err!=nil && !errors.Is(err,pgx.ErrNoRows){return err}
		if err==nil && len(digest)!=32{return runbook.ErrUnavailable};copy(current.Digest[:],digest)
		if current.Version!=c.ExpectedVersion{return runbook.ErrConflict}
		if c.Revoke && (!current.Approved || current.Digest!=c.Digest) {return runbook.ErrConflict}
		if current.Version==0 {
			var count int;if err:=tx.QueryRow(ctx,`SELECT count(*) FROM runbook_corpora`).Scan(&count);err!=nil{return err};if count>=runbook.MaximumCorpora{return runbook.ErrConflict}
		}
		var at time.Time;if err:=tx.QueryRow(ctx,`SELECT clock_timestamp()`).Scan(&at);err!=nil{return err};at=at.UTC().Truncate(time.Microsecond)
		c.Grant.ConsumedAt=at;c.Audit.OccurredAt=at
		confirmation,err:=tx.Exec(ctx,`DELETE FROM runbook_confirmations WHERE session_id=$1 AND user_id=$2 AND corpus_id=$3 AND expected_version=$4 AND restore_generation=$5 AND candidate_digest=$6 AND revoke=$7 AND token_digest=$8 AND created_at_us<=$9 AND expires_at_us>$9`,string(c.Actor.SessionID),string(c.Actor.UserID),c.Corpus.ID,c.ExpectedVersion,generation,c.Digest[:],c.Revoke,c.ConfirmationDigest[:],at.UnixMicro())
		if err!=nil{return err};if confirmation.RowsAffected()!=1{return runbook.ErrDenied}
		if err:=consumeRecentGrantTx(ctx,tx,c.Grant,auth.RecentAuthPurposeRunbookManage);err!=nil{return err}
		if _,err:=tx.Exec(ctx,`DELETE FROM runbook_documents WHERE corpus_id=$1`,c.Corpus.ID);err!=nil{return err}
		if c.Revoke {
			if _,err:=tx.Exec(ctx,`UPDATE runbook_corpora SET approved=false,document_count=0,content_bytes=0,updated_at_us=$2,restore_generation=$3 WHERE id=$1`,c.Corpus.ID,at.UnixMicro(),generation);err!=nil{return err}
			current.Approved=false;current.Documents=0;current.Bytes=0;current.Generation=generation;result=current
		} else {
			size:=len(c.Corpus.ID)+len(c.Corpus.Audience);for _,d:=range c.Corpus.Documents{size+=len(d.ID)+len(d.Locale)+len(d.Title)+len(d.Body)}
			if _,err:=tx.Exec(ctx,`INSERT INTO runbook_corpora(id,version,digest,audience,approved,restore_generation,document_count,content_bytes,updated_at_us)
VALUES($1,$2,$3,$4,true,$5,$6,$7,$8) ON CONFLICT(id) DO UPDATE SET version=EXCLUDED.version,digest=EXCLUDED.digest,audience=EXCLUDED.audience,approved=true,restore_generation=EXCLUDED.restore_generation,document_count=EXCLUDED.document_count,content_bytes=EXCLUDED.content_bytes,updated_at_us=EXCLUDED.updated_at_us`,c.Corpus.ID,c.Corpus.Version,c.Digest[:],string(c.Corpus.Audience),generation,len(c.Corpus.Documents),size,at.UnixMicro());err!=nil{return err}
			for _,d:=range c.Corpus.Documents{if _,err:=tx.Exec(ctx,`INSERT INTO runbook_documents(corpus_id,id,locale,title,body) VALUES($1,$2,$3,$4,$5)`,c.Corpus.ID,d.ID,d.Locale,d.Title,d.Body);err!=nil{return err}}
			result=runbook.Revision{ID:c.Corpus.ID,Version:c.Corpus.Version,Digest:c.Digest,Audience:c.Corpus.Audience,Approved:true,Generation:generation,Documents:len(c.Corpus.Documents),Bytes:size}
		}
		if _,err:=tx.Exec(ctx,`UPDATE runbook_catalog SET row_version=row_version+1 WHERE singleton=1`);err!=nil{return err}
		if err:=runbookLive(ctx,tx,c.Actor,true);err!=nil{return err}
		return insertAuthAuditTx(ctx,tx,c.Audit)
	})
	if err!=nil{return runbook.Revision{},runbookError(err)};return result,nil
}

func (store *Store) InspectRunbooks(ctx context.Context,actor auth.Principal) (runbook.Catalog,error) {
	if ctx==nil || actor.Validate()!=nil {return runbook.Catalog{},runbook.ErrInvalid}
	ctx,cancel:=context.WithTimeout(ctx,3*time.Second);defer cancel();result:=runbook.Catalog{Revisions:[]runbook.Revision{}}
	err:=store.withinTransaction(ctx,func(ctx context.Context,transaction *transaction) error {
		tx:=transaction.transaction;if err:=runbookLive(ctx,tx,actor,true);err!=nil{return err};var err error
		result.Generation,result.Version,err=runbookScope(ctx,tx,false);if err!=nil{return err}
		rows,err:=tx.Query(ctx,`SELECT id,version,digest,audience,approved,restore_generation,document_count,content_bytes FROM runbook_corpora ORDER BY id LIMIT 17`);if err!=nil{return err};defer rows.Close()
		for rows.Next(){var revision runbook.Revision;var digest []byte;if err:=rows.Scan(&revision.ID,&revision.Version,&digest,&revision.Audience,&revision.Approved,&revision.Generation,&revision.Documents,&revision.Bytes);err!=nil{return err};if len(digest)!=32 || len(result.Revisions)>=runbook.MaximumCorpora{return runbook.ErrUnavailable};copy(revision.Digest[:],digest);result.Revisions=append(result.Revisions,revision)}
		if err:=rows.Err();err!=nil{return err};rows.Close();return runbookLive(ctx,tx,actor,true)
	});if err!=nil{clear(result.Revisions);return runbook.Catalog{},runbookError(err)};return result,nil
}

// The fixed query parser treats search prose as terms, never as query syntax.
const runbookSearchSQL=`SELECT c.id,c.version,c.digest,c.restore_generation,d.id,d.title,d.body
FROM runbook_corpora c JOIN runbook_documents d ON d.corpus_id=c.id
WHERE c.approved AND c.restore_generation=$1 AND (c.audience='all_users' OR $2::boolean)
AND d.locale=$3 AND d.search_terms @@ plainto_tsquery('pg_catalog.simple'::regconfig,$4)
AND ($5::text='' OR (c.id,d.id)>($5,$6)) ORDER BY c.id,d.id LIMIT $7`

func (store *Store) SearchRunbooks(ctx context.Context,q runbook.Search) (runbook.Page,error) {
	if ctx==nil || q.Validate()!=nil{return runbook.Page{},runbook.ErrInvalid}
	ctx,cancel:=context.WithTimeout(ctx,3*time.Second);defer cancel()
	page:=runbook.Page{Hits:make([]runbook.Hit,0,q.Limit)}
	err:=store.withinTransaction(ctx,func(ctx context.Context,transaction *transaction) error {
		tx:=transaction.transaction;if err:=runbookLive(ctx,tx,q.Actor,false);err!=nil{return err}
		generation,catalog,err:=runbookScope(ctx,tx,false);if err!=nil{return err}
		if q.Generation>=0 && (q.Generation!=generation || q.CatalogVersion!=catalog){return runbook.ErrDenied}
		page.Generation=generation;page.CatalogVersion=catalog
		rows,err:=tx.Query(ctx,runbookSearchSQL,generation,q.Actor.Role==auth.RoleAdministrator,q.Locale,q.Query,q.BeforeCorpus,q.BeforeDocument,q.Limit+1);if err!=nil{return err}
		defer rows.Close()
		for rows.Next(){var hit runbook.Hit;var digest []byte;var prefix string
			if err:=rows.Scan(&hit.Reference.CorpusID,&hit.Reference.Version,&digest,&hit.Reference.Generation,&hit.Reference.DocumentID,&hit.Title,&prefix);err!=nil{return err}
			hit.Reference.Locale=q.Locale;if len(digest)!=32{return runbook.ErrUnavailable};copy(hit.Reference.Digest[:],digest)
			if hit.Reference.Validate()!=nil{return runbook.ErrUnavailable}
			hit.Snippet,err=runbook.Snippet(prefix);if err!=nil{return runbook.ErrUnavailable}
			if len(page.Hits)==q.Limit{page.HasMore=true;continue};page.Hits=append(page.Hits,hit)
		};if err:=rows.Err();err!=nil{return err};rows.Close();return runbookLive(ctx,tx,q.Actor,false)
	})
	if err!=nil{clear(page.Hits);return runbook.Page{},runbookError(err)};return page,nil
}

func (store *Store) ReadRunbook(ctx context.Context,actor auth.Principal,reference runbook.Reference) (runbook.Document,error) {
	if ctx==nil || actor.Validate()!=nil || reference.Validate()!=nil{return runbook.Document{},runbook.ErrInvalid}
	ctx,cancel:=context.WithTimeout(ctx,3*time.Second);defer cancel();var document runbook.Document
	err:=store.withinTransaction(ctx,func(ctx context.Context,transaction *transaction) error {
		tx:=transaction.transaction;if err:=runbookLive(ctx,tx,actor,false);err!=nil{return err}
		generation,_,err:=runbookScope(ctx,tx,false);if err!=nil{return err};if generation!=reference.Generation{return runbook.ErrDenied}
		err=tx.QueryRow(ctx,`SELECT d.id,d.locale,d.title,d.body FROM runbook_corpora c JOIN runbook_documents d ON d.corpus_id=c.id
WHERE c.id=$1 AND c.version=$2 AND c.digest=$3 AND c.restore_generation=$4 AND c.approved
AND (c.audience='all_users' OR $5::boolean) AND d.id=$6 AND d.locale=$7`,reference.CorpusID,reference.Version,reference.Digest[:],generation,actor.Role==auth.RoleAdministrator,reference.DocumentID,reference.Locale).Scan(&document.ID,&document.Locale,&document.Title,&document.Body)
		if errors.Is(err,pgx.ErrNoRows){return runbook.ErrDenied};if err!=nil{return err};if document.Validate()!=nil{return runbook.ErrUnavailable};return runbookLive(ctx,tx,actor,false)
	})
	if err!=nil{return runbook.Document{},runbookError(err)};return document,nil
}
