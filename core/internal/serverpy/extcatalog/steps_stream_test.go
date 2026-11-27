package extcatalog

import (
	"encoding/json"
	"strings"
	"testing"
)

// The install answer carries the plan and the step changes next to the lines,
// for a page that shows them (the launcher's Extensions page).
func TestInstallStreamsThePlanAndTheSteps(t *testing.T) {
	s, _, _ := nativeStore(t)
	_, got, raw := stream(t, s, `install {"package_name":"ext.native","gpu":"Custom (Manual torch install)"}`)
	if !got.ok {
		t.Fatalf("install: %+v", got.lines)
	}
	type item struct{ ID, Label string }
	var plan []item
	state := map[string]string{}
	planAt, firstStepAt := -1, -1
	for i, l := range strings.Split(strings.TrimSpace(raw), "\n") {
		var m struct {
			Plan []item `json:"plan"`
			Step *struct {
				ID    string `json:"id"`
				State string `json:"state"`
			} `json:"step"`
		}
		if json.Unmarshal([]byte(l), &m) != nil {
			t.Fatalf("line %d is not JSON: %s", i, l)
		}
		if m.Plan != nil {
			plan, planAt = m.Plan, i
		}
		if m.Step != nil {
			if firstStepAt < 0 {
				firstStepAt = i
			}
			if m.Step.State != "" {
				state[m.Step.ID] = m.Step.State
			}
		}
	}
	if planAt < 0 || (firstStepAt >= 0 && firstStepAt < planAt) {
		t.Fatalf("the plan comes first: plan at %d, first step at %d", planAt, firstStepAt)
	}
	var ids []string
	for _, p := range plan {
		if p.Label == "" {
			t.Errorf("%s has no label", p.ID)
		}
		ids = append(ids, p.ID)
	}
	joined := strings.Join(ids, ",")
	for _, id := range []string{"uv", "python", "ext.native:venv", "ext.native:torch", "ext.native:requirements"} {
		if !strings.Contains(joined, id) {
			t.Errorf("the plan lacks %s: %s", id, joined)
		}
		if state[id] != "done" && state[id] != "skipped" {
			t.Errorf("%s ended %q, want done: %v", id, state[id], state)
		}
	}
	// Every planned step was reported.
	for _, id := range ids {
		if _, ok := state[id]; !ok {
			t.Errorf("planned step %s never reported", id)
		}
	}
}
