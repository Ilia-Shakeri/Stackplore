package assistantevidence

import (
 "context"
 "errors"
 "github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
 "github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
 "github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
)

// Admission checks metadata only. It cannot replace the reserved read or its seal.
func (reader *Reader) AuthorizeDocumentAdmission(ctx context.Context,actor assistant.Actor,run assistant.Run,scope contextbuilder.Scope,reference runbook.Reference)error{
 if reader==nil || reader.documents==nil || ctx==nil || reference.Validate()!=nil || reference.Generation!=scope.RestoreGeneration {return ErrDenied}
 if _,err:=reader.validate(actor,run,scope);err!=nil{return err}
 work,cancel,err:=reader.metadataWork(ctx)
 if err!=nil{return err}
 defer cancel()
 if err:=reader.documents.AuthorizeAssistantRunbook(work,actor,reference);err!=nil{return documentError(err)}
 return nil
}

func documentError(err error)error{
 if errors.Is(err,runbook.ErrDenied) || errors.Is(err,runbook.ErrConflict){return ErrDenied}
 if errors.Is(err,runbook.ErrInvalid){return ErrInvalid}
 if errors.Is(err,context.Canceled) || errors.Is(err,context.DeadlineExceeded){return ErrCancelled}
 return ErrUnavailable
}
