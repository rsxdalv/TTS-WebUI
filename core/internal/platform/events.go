package platform

import "context"

// The steps of an install, for a page that shows them. An install is a text
// stream (Emit); a Reporter in the context is the second, structured stream:
// the plan up front, then a change to one step at a time. With no Reporter in
// the context every call here does nothing, so callers that only want the text
// (tests, the CLI) are not affected.

// Step states.
const (
	StepPending = "pending"
	StepRunning = "running"
	StepDone    = "done"
	StepSkipped = "skipped" // already there
	StepFailed  = "failed"
)

// PlanItem is one step of the plan.
type PlanItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// StepEvent is a change to one step. Zero fields mean "unchanged", except that
// Done and Total are only carried while they mean something.
type StepEvent struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
	State string `json:"state,omitempty"`
	// Done and Total are bytes, or Unit's own count (packages) when Unit is set.
	Done  int64  `json:"done,omitempty"`
	Total int64  `json:"total,omitempty"`
	Unit  string `json:"unit,omitempty"`
	// Estimated says Total is a guess (uv prints no progress for a download;
	// the cache folder's growth is measured against the size of the unpacked
	// wheels).
	Estimated bool   `json:"estimated,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// Reporter receives the plan and the step changes of an install. Its methods
// may be called from several goroutines.
type Reporter interface {
	Plan(items []PlanItem)
	Step(ev StepEvent)
}

type reporterKey struct{}

// WithReporter returns a context whose installs report to r.
func WithReporter(ctx context.Context, r Reporter) context.Context {
	return context.WithValue(ctx, reporterKey{}, r)
}

// ReporterFrom is the context's Reporter, or one that drops everything.
func ReporterFrom(ctx context.Context) Reporter {
	if r, ok := ctx.Value(reporterKey{}).(Reporter); ok && r != nil {
		return r
	}
	return nopReporter{}
}

// Plan announces the steps of the install, in order.
func Plan(ctx context.Context, items ...PlanItem) { ReporterFrom(ctx).Plan(items) }

// Step reports one step's change.
func Step(ctx context.Context, ev StepEvent) { ReporterFrom(ctx).Step(ev) }

type nopReporter struct{}

func (nopReporter) Plan([]PlanItem) {}
func (nopReporter) Step(StepEvent)  {}
