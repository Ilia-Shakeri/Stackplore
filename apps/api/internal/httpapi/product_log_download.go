package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const (
	logDownloadMaximumBytes   = 1 << 20
	logDownloadMaximumLines   = 1000
	logDownloadMaximumScanned = 10000
	logDownloadMaximumPages   = 100
	logDownloadLifetime       = 10 * time.Second
	logDownloadFooterReserve  = 128
)

func (handler productHTTP) downloadLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), logDownloadLifetime)
	defer cancel()
	r = r.WithContext(ctx)
	if !requireNoBody(w, r) {
		return
	}
	var bucket string
	if handler.auth.guestDemo != nil {
		var ok bool
		bucket, ok = guestStreamBucket(r)
		if !ok {
			writeProblem(w, r, invalidRequestProblem)
			return
		}
	} else {
		identity, ok := handler.authenticateIdentity(w, r)
		if !ok {
			return
		}
		bucket = "user:" + string(identity.User.ID)
	}
	release, ok := handler.downloadGate.acquireBucket(bucket)
	if !ok {
		writeProblem(w, r, rateLimitProblem)
		return
	}
	defer release()
	resourceID, term, severity, err := parseLogDownloadQuery(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	reader, ok := provider.(domain.LogReadProvider)
	if !ok {
		writeProblem(w, r, unavailableProblem)
		return
	}
	reader = configuredLogReader{reader: reader, settings: handler.applicationSettings, policy: handler.logRedactionPolicy}
	refs, selectionStatus, err := handler.resourceSelection(r, provider, resourceID, domain.MaxLogTargets)
	if err != nil {
		writeResourceProblem(w, r, err)
		return
	}
	if !selectionStatus.AllowsData() {
		writeLogDownloadAvailability(w, r, selectionStatus)
		return
	}
	container, previous, err := parseLogSelection(r, resourceID)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	end := observationTime(provider).Add(time.Nanosecond)
	rangeValue, err := parseObservationRange(r, domain.TimeRange{Start: end.Add(-24 * time.Hour), End: end}, domain.MaxLogLookback)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	targets := make([]domain.LogTarget, len(refs))
	for index, ref := range refs {
		targets[index] = domain.LogTarget{Resource: ref, Container: container, Previous: previous}
	}
	query := domain.LogReadQuery{Source: providerRef(provider), Targets: targets, Range: rangeValue, LineLimit: domain.MaxLogLines, ByteLimit: logDownloadMaximumBytes - logDownloadFooterReserve}
	var page domain.LogPage
	if len(targets) > 0 {
		page, err = reader.ReadLogs(ctx, query)
		if err != nil {
			writeProviderProblem(w, r, err)
			return
		}
		if !page.Status.AllowsData() {
			writeLogDownloadAvailability(w, r, page.Status)
			return
		}
		if !validLogDownloadPage(page, query) {
			writeProblem(w, r, unavailableProblem)
			return
		}
	} else {
		status, err := emptyReadStatus(r, provider, selectionStatus, domain.CapabilityLogRead)
		if err != nil {
			writeProviderProblem(w, r, err)
			return
		}
		if !status.AllowsData() {
			writeLogDownloadAvailability(w, r, status)
			return
		}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="stackplore-logs.txt"`)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	written := 0
	write := func(text string) bool {
		if ctx.Err() != nil || written+len(text) > logDownloadMaximumBytes {
			return false
		}
		deadline := time.Now().Add(streamWriteTimeout)
		if lifetime, ok := ctx.Deadline(); ok && lifetime.Before(deadline) {
			deadline = lifetime
		}
		_ = controller.SetWriteDeadline(deadline)
		count, err := fmt.Fprint(w, text)
		written += count
		return err == nil && count == len(text)
	}
	flush := func() bool { err := controller.Flush(); return err == nil || errors.Is(err, http.ErrNotSupported) }
	if !write("# Stackplore bounded log download v1\n") || !flush() {
		return
	}
	result := "complete"
	lines, scanned := 0, 0
	cursors := make(map[domain.PageCursor]struct{}, logDownloadMaximumPages)
	seen := make(map[domain.LogEntryID]struct{}, logDownloadMaximumScanned)
export:
	for pageIndex := 0; pageIndex < logDownloadMaximumPages; pageIndex++ {
		for recordIndex, record := range page.Records {
			if ctx.Err() != nil {
				return
			}
			if _, duplicate := seen[record.ID]; duplicate {
				result = "incomplete"
				break export
			}
			seen[record.ID] = struct{}{}
			scanned++
			if (severity == "" || record.Severity == severity) && (term == "" || strings.Contains(strings.ToLower(record.Message), term)) {
				target := string(record.ResourceID)
				if record.Container != "" {
					target += " [" + record.Container + "]"
				}
				line := authTimestamp(record.Timestamp) + " " + string(record.Severity) + " " + target + " " + record.Message + "\n"
				if lines >= logDownloadMaximumLines || written+len(line) > logDownloadMaximumBytes-logDownloadFooterReserve {
					result = "limited"
					break export
				}
				if !write(line) {
					return
				}
				lines++
			}
			if scanned >= logDownloadMaximumScanned {
				if recordIndex+1 < len(page.Records) || page.Page.HasMore {
					result = "limited"
				}
				break export
			}
		}
		if !flush() {
			return
		}
		if !page.Page.HasMore {
			break
		}
		if pageIndex+1 >= logDownloadMaximumPages {
			result = "limited"
			break
		}
		if _, repeated := cursors[page.Page.NextCursor]; repeated {
			result = "incomplete"
			break
		}
		cursors[page.Page.NextCursor] = struct{}{}
		query.Cursor = page.Page.NextCursor
		// Provider cursors bind the read budgets as well as source and range.
		// Keep these fixed; the writer independently enforces the total export cap.
		page, err = reader.ReadLogs(ctx, query)
		if err != nil || !page.Status.AllowsData() || !validLogDownloadPage(page, query) {
			result = "incomplete"
			break
		}
	}
	if write("# stackplore-export: " + result + "\n") {
		_ = flush()
	}
}

func validLogDownloadPage(page domain.LogPage, query domain.LogReadQuery) bool {
	if page.Page.Limit < 1 || page.Page.Limit > query.LineLimit {
		return false
	}
	query.LineLimit = page.Page.Limit
	return page.ValidateFor(query) == nil
}

func writeLogDownloadAvailability(w http.ResponseWriter, r *http.Request, status domain.ObservationStatus) {
	if status.State == domain.AvailabilityPermissionDenied {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	writeProblem(w, r, unavailableProblem)
}

func parseLogDownloadQuery(r *http.Request) (domain.ResourceID, string, domain.LogSeverity, error) {
	values := r.URL.Query()
	for name, items := range values {
		if len(items) != 1 || items[0] == "" {
			return "", "", "", errors.New("invalid download query")
		}
		switch name {
		case "source_id", "resource_id", "from", "to", "container", "previous", "q", "severity":
		default:
			return "", "", "", errors.New("unknown download filter")
		}
	}
	resource := domain.ResourceID(values.Get("resource_id"))
	if resource != "" && resource.Validate() != nil {
		return "", "", "", errors.New("invalid download resource")
	}
	term := values.Get("q")
	if (domain.ResourceFilter{Search: term}).Validate() != nil {
		return "", "", "", errors.New("invalid download search")
	}
	severity := domain.LogSeverity(values.Get("severity"))
	if severity != "" && severity.Validate() != nil {
		return "", "", "", errors.New("invalid download severity")
	}
	return resource, strings.ToLower(term), severity, nil
}
