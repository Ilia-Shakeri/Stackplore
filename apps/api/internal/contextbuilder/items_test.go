package contextbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestItemsKeepEveryCanonicalRecordAndQualityEnvelope(t *testing.T) {
	request := requestFixture(t)
	request.MoreAvailable = true
	request.Status.Completeness = domain.CompletenessPartial
	request.Status.ReasonCode = "source.partial"
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := bundle.JSON()
	before := bundle.Citations()
	items, err := bundle.Items()
	if err != nil || len(items) != len(before) {
		t.Fatal("valid bundle did not export every citation")
	}
	for index, item := range items {
		if item.ID != before[index].ID || !canonicalItemID(item.ID) || len(item.Text) > MaximumItemBytes || !utf8.ValidString(item.Text) || !json.Valid([]byte(item.Text)) {
			t.Fatal("item lost its canonical ID or text bound")
		}
		var part itemDocument
		if json.Unmarshal([]byte(item.Text), &part) != nil || part.ItemIndex != index || part.ItemCount != len(items) || len(part.Records) != 1 || !reflect.DeepEqual(part.Records[0], bundle.document.Records[index]) {
			t.Fatal("item changed a cited record or its bundle position")
		}
		part.Records = bundle.document.Records
		if !reflect.DeepEqual(part.document, bundle.document) {
			t.Fatal("item lost source, version, time, completeness or remaining-data metadata")
		}
		if strings.Contains(item.Text, "private_") || strings.Contains(item.Text, "private.") || strings.Contains(item.Text, "href") {
			t.Fatal("remote projection exposed names, prose or an action link")
		}
	}
	if wire := explicitItemsJSON(t, items); len(wire) > MaximumItemsBytes {
		t.Fatal("explicit wire array exceeded its complete framing bound")
	}
	after, _ := bundle.JSON()
	if !bytes.Equal(original, after) || !reflect.DeepEqual(before, bundle.Citations()) {
		t.Fatal("export mutated the canonical bundle")
	}
	items[0].ID, items[0].Text = "changed", "changed"
	repeat, err := bundle.Items()
	if err != nil || repeat[0].ID != before[0].ID || repeat[0].Text == "changed" {
		t.Fatal("returned items could mutate sealed evidence")
	}
}

func TestItemsPreserveUnavailableWithoutInventingCitation(t *testing.T) {
	request := requestFixture(t)
	request.Resources, request.Health, request.Metrics, request.Events, request.Relationships = nil, nil, nil, nil, nil
	request.Status.State, request.Status.ReasonCode = domain.AvailabilityUnsupported, "source.unsupported"
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	items, err := bundle.Items()
	if err != nil || items == nil || len(items) != 0 || bundle.document.Quality.Availability != domain.AvailabilityUnsupported {
		t.Fatal("unavailable scope acquired a fabricated citation")
	}
	if string(explicitItemsJSON(t, items)) != "[]" {
		t.Fatal("empty evidence wire was not an empty array")
	}
}

func TestItemsRespectReferenceAndCompleteWireBounds(t *testing.T) {
	for _, count := range []int{MaximumItems, MaximumItems + 1} {
		t.Run(fmt.Sprintf("events-%d", count), func(t *testing.T) {
			request := repeatedItemsRequest(t, "event", count)
			bundle, err := Build(context.Background(), request)
			if err != nil {
				t.Fatal("projection fixture did not fit the original bundle cap")
			}
			items, err := bundle.Items()
			if count > MaximumItems {
				if !errors.Is(err, ErrBudget) || items != nil {
					t.Fatal("overflow returned an uncited prefix")
				}
				return
			}
			if err != nil || len(items) != MaximumItems || len(explicitItemsJSON(t, items)) > MaximumItemsBytes {
				t.Fatal("legal 64-reference scope failed its explicit wire bound")
			}
		})
	}
	request := repeatedItemsRequest(t, "metric", MaximumItems)
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := bundle.JSON()
	if len(encoded) >= MaximumItemsBytes {
		t.Fatal("aggregate fixture already exceeded the original bundle budget")
	}
	if items, err := bundle.Items(); !errors.Is(err, ErrBudget) || items != nil {
		t.Fatal("repeated envelopes bypassed the aggregate wire bound or silently dropped facts")
	}
}

func TestItemsBoundEscapedLogFramingWithoutChangingCitation(t *testing.T) {
	request := requestFixture(t)
	request.Destination, request.LogConsent = Local, true
	capture := captureFixture(t, request, func(query *domain.LogReadQuery, page *domain.LogPage) {
		query.ByteLimit = MaximumLogBytes
		page.Records[0].Message = strings.Repeat("<", MaximumLogLineBytes)
	})
	request.Logs = &capture
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := bundle.JSON()
	if items, err := bundle.Items(); !errors.Is(err, ErrBudget) || items != nil {
		t.Fatal("4 KiB raw line bypassed the 8 KiB escaped JSON cap")
	}
	after, _ := bundle.JSON()
	if !bytes.Equal(before, after) {
		t.Fatal("export truncated already-cited content to make it fit")
	}
}

func TestItemsKeepLocalLogConsentMaskingUTF8AndTruncation(t *testing.T) {
	request := requestFixture(t)
	request.Destination, request.LogConsent = Local, true
	capture := captureFixture(t, request, func(query *domain.LogReadQuery, page *domain.LogPage) {
		query.ByteLimit = MaximumLogBytes
		page.Page.HasMore, page.Page.NextCursor = true, "bmV4dA"
		page.Records[0].Message = "customer_id=synthetic_marker " + strings.Repeat("ژ", 3000)
	})
	request.Logs = &capture
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	items, err := bundle.Items()
	if err != nil || len(items) != 6 {
		t.Fatal("legal local log did not fit bounded export")
	}
	var last itemDocument
	if json.Unmarshal([]byte(items[len(items)-1].Text), &last) != nil || len(last.Records) != 1 || last.Records[0].Log == nil {
		t.Fatal("local log framing lost its record")
	}
	line := last.Records[0].Log
	if !utf8.ValidString(line.Message) || len(line.Message) > MaximumLogLineBytes || !line.Truncated || !line.Redacted || strings.Contains(line.Message, "synthetic_marker") || !strings.Contains(line.Message, "[redacted]") || !last.MoreAvailable {
		t.Fatal("export lost masking, UTF-8, truncation or page completeness")
	}
	for _, change := range []func(*Request){
		func(value *Request) { value.LogConsent = false },
		func(value *Request) { value.Destination = Remote },
	} {
		denied := request
		change(&denied)
		if rejected, err := Build(context.Background(), denied); !errors.Is(err, ErrDenied) {
			t.Fatal("export construction bypassed local log consent")
		} else if leaked, err := rejected.Items(); !errors.Is(err, ErrInvalid) || leaked != nil {
			t.Fatal("rejected log construction produced evidence items")
		}
	}
}

func TestItemsUseSafeFixedErrorsForInvalidSealedState(t *testing.T) {
	if items, err := (Bundle{}).Items(); !errors.Is(err, ErrInvalid) || items != nil {
		t.Fatal("zero bundle emitted items")
	}
	for name, edit := range map[string]func(*Bundle){
		"source":          func(bundle *Bundle) { bundle.document.SourceReference = "private_source" },
		"provider":        func(bundle *Bundle) { bundle.document.Provider = "private_provider" },
		"reference-count": func(bundle *Bundle) { bundle.references = bundle.references[:1] },
		"reference-ID":    func(bundle *Bundle) { bundle.references[0].ID = "private_reference" },
		"reference-scope": func(bundle *Bundle) { bundle.references[0].PolicyVersion++ },
		"reference-time":  func(bundle *Bundle) { bundle.references[0].ObservedAt = bundle.references[0].ObservedAt.Add(1) },
		"citation":        func(bundle *Bundle) { bundle.document.Records[0].CitationID = "ev_" + strings.Repeat("G", 64) },
		"duplicate": func(bundle *Bundle) {
			bundle.document.Records[1] = bundle.document.Records[0]
			bundle.references[1] = bundle.references[0]
		},
		"quality": func(bundle *Bundle) { bundle.document.Records[0].Quality.Completeness = "private_quality" },
		"quality-time": func(bundle *Bundle) {
			bundle.document.Records[0].Quality.ObservedAt = bundle.document.Records[0].Quality.ObservedAt.Add(1)
		},
		"invalid-UTF8": func(bundle *Bundle) { bundle.document.Records[0].Log = &logFact{Message: string([]byte{0xff})} },
	} {
		t.Run(name, func(t *testing.T) {
			bundle, err := Build(context.Background(), requestFixture(t))
			if err != nil {
				t.Fatal(err)
			}
			edit(&bundle)
			items, err := bundle.Items()
			if !errors.Is(err, ErrInvalid) || items != nil || strings.Contains(err.Error(), "private_") {
				t.Fatal("malformed evidence leaked a partial result or raw metadata")
			}
		})
	}
	bundle, err := Build(context.Background(), requestFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	bundle.document.Quality.Availability = domain.AvailabilityPermissionDenied
	if items, err := bundle.Items(); !errors.Is(err, ErrDenied) || items != nil {
		t.Fatal("denied envelope emitted evidence")
	}
}

func TestItemOrdinaryFormattingAndJSONDoNotLogText(t *testing.T) {
	item := Item{ID: "test_reference", Text: "private_evidence_text"}
	for _, value := range []any{item, &item, []Item{item}} {
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			if strings.Contains(fmt.Sprintf(format, value), "private_") {
				t.Fatal("ordinary item formatting exposed evidence text")
			}
		}
		data, err := json.Marshal(value)
		if err != nil || strings.Contains(string(data), "private_") || strings.Contains(string(data), "test_reference") {
			t.Fatal("ordinary item JSON exposed volatile text or reference")
		}
	}
	if data := explicitItemsJSON(t, []Item{item}); !bytes.Contains(data, []byte(item.Text)) || !bytes.Contains(data, []byte(item.ID)) {
		t.Fatal("explicit transport could not access projected fields")
	}
}

func repeatedItemsRequest(t *testing.T, kind string, count int) Request {
	t.Helper()
	request := requestFixture(t)
	event, metric := request.Events[0], request.Metrics[0]
	request.Resources, request.Health, request.Metrics, request.Events, request.Relationships = nil, nil, nil, nil, nil
	for index := 0; index < count; index++ {
		if kind == "event" {
			entry := event
			entry.ID = domain.EventID(fmt.Sprintf("event_%d", index))
			request.Events = append(request.Events, entry)
		} else {
			entry := metric
			entry.ID = domain.MetricSampleID(fmt.Sprintf("metric_%d", index))
			request.Metrics = append(request.Metrics, entry)
		}
	}
	return request
}

func explicitItemsJSON(t *testing.T, items []Item) []byte {
	t.Helper()
	wire := make([]struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}, len(items))
	for index, item := range items {
		wire[index].ID, wire[index].Text = item.ID, item.Text
	}
	data, err := json.Marshal(wire)
	if err != nil {
		t.Fatal("explicit item transport did not serialize")
	}
	return data
}
