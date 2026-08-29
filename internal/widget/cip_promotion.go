package widget

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

// CIPPromotion is one version of a repository on its way through the
// stages. The daemon leaves its Go fields untagged, so the JSON keys are
// the capitalized Go names.
type CIPPromotion struct {
	ID     int    `json:"ID"`
	Repo   string `json:"Repo"`
	SHA    string `json:"SHA"`
	Branch string `json:"Branch"`
	// State is active, passed, failed, or superseded.
	State   string `json:"State"`
	Trigger string `json:"Trigger"`
	// SpecJSON is the spec snapshot. The list endpoint leaves it empty; only
	// the detail endpoint fills it in.
	SpecJSON  string    `json:"SpecJSON"`
	CreatedAt time.Time `json:"CreatedAt"`
}

// ShortSHA is the first 7 characters of the commit.
func (p CIPPromotion) ShortSHA() string {
	if len(p.SHA) > 7 {
		return p.SHA[:7]
	}
	return p.SHA
}

// CIPStage is one stage of one promotion.
type CIPStage struct {
	PromotionID int    `json:"PromotionID"`
	Stage       string `json:"Stage"`
	// State is pending, gated, running, passed, failed, or superseded.
	State      string    `json:"State"`
	EligibleAt time.Time `json:"EligibleAt"`
	// GateIdx is the gate the stage waits at, as an index into the gates of
	// this stage in the spec.
	GateIdx int `json:"GateIdx"`
	// RunID links the stage to the run that carried out its jobs. It is 0
	// while no run started.
	RunID         int       `json:"RunID"`
	ApprovedBy    string    `json:"ApprovedBy"`
	ApprovedAt    time.Time `json:"ApprovedAt"`
	ApproveReason string    `json:"ApproveReason"`
	// GateStartedAt is when the clock of a bake or window gate started.
	GateStartedAt time.Time `json:"GateStartedAt"`
}

// HasRun reports whether a run carried out this stage.
func (s CIPStage) HasRun() bool { return s.RunID != 0 }

// Approved reports whether a person approved this stage.
func (s CIPStage) Approved() bool { return !s.ApprovedAt.IsZero() }

// WaitingFor is how long the stage has waited at its gate. It is zero
// before the gate clock starts, because a stage that never became eligible
// has waited no time at all.
func (s CIPStage) WaitingFor(now time.Time) time.Duration {
	if s.GateStartedAt.IsZero() {
		return 0
	}
	d := now.Sub(s.GateStartedAt)
	if d < 0 {
		return 0
	}
	return d
}

// CIPGate is one condition that holds a stage back. The spec is written by
// the CDK, so its keys are lower camel case, unlike the daemon's own rows.
type CIPGate struct {
	// Type is manual, bake, check, window, or alarm.
	Type       string `json:"type"`
	Minutes    int    `json:"minutes"`
	Run        string `json:"run"`
	TimeoutSec int    `json:"timeoutSec"`
}

// Describe says why a stage waits, in words a reader understands. An
// unknown gate type still names itself, because "waiting for something" is
// more honest than silence.
func (g CIPGate) Describe() string {
	switch g.Type {
	case "manual":
		return "awaiting approval"
	case "bake":
		return fmt.Sprintf("bake %dm", g.Minutes)
	case "check":
		if g.Run != "" {
			return "check: " + g.Run
		}
		return "check"
	case "window":
		return "waiting for the time window"
	case "alarm":
		return fmt.Sprintf("alarm %dm", g.Minutes)
	case "":
		return "waiting"
	default:
		return "gate " + g.Type
	}
}

// CIPSpecStage is one stage as the spec declares it. Needs gives the edges
// of the stage flow, and Gates gives the conditions to enter the stage.
type CIPSpecStage struct {
	Name  string    `json:"name"`
	Needs []string  `json:"needs"`
	Gates []CIPGate `json:"gates"`
}

// CIPSpec is the part of the pipeline spec this widget reads. It decodes
// only the fields it draws, so a spec that grows new fields still parses.
type CIPSpec struct {
	Name   string         `json:"name"`
	Stages []CIPSpecStage `json:"stages"`
}

// GateFor is the gate one stage waits at. It returns false when the stage
// or the index is unknown, so a caller shows no reason rather than a wrong
// one.
func (s CIPSpec) GateFor(stage string, index int) (CIPGate, bool) {
	for _, declared := range s.Stages {
		if declared.Name != stage {
			continue
		}
		if index < 0 || index >= len(declared.Gates) {
			return CIPGate{}, false
		}
		return declared.Gates[index], true
	}
	return CIPGate{}, false
}

// CIPPromotionEntry is one row of GET /promotions.
type CIPPromotionEntry struct {
	Promotion CIPPromotion `json:"promotion"`
	Stages    []CIPStage   `json:"stages"`
}

// CIPPromotionList is every promotion the daemon knows, newest first.
type CIPPromotionList struct {
	SchemaVersion int                 `json:"schema_version"`
	Name          string              `json:"name"`
	At            time.Time           `json:"at"`
	Promotions    []CIPPromotionEntry `json:"promotions"`
	Error         string              `json:"error,omitempty"`
}

// CIPPromotionDetail is one promotion with its stages and its parsed spec.
type CIPPromotionDetail struct {
	SchemaVersion int          `json:"schema_version"`
	Name          string       `json:"name"`
	At            time.Time    `json:"at"`
	Promotion     CIPPromotion `json:"promotion"`
	Stages        []CIPStage   `json:"stages"`
	Spec          CIPSpec      `json:"spec"`
	Error         string       `json:"error,omitempty"`
}

// FetchCIPPromotions reads only the authenticated GET /promotions endpoint.
// It never approves a gate and never starts a stage. It fails closed.
func FetchCIPPromotions(ctx context.Context, provider config.Widget, token string) CIPPromotionList {
	result := CIPPromotionList{SchemaVersion: 1, Name: provider.Name, At: time.Now()}
	if strings.TrimSpace(token) == "" {
		result.Error = "no token configured: set token_env or token_file"
		return result
	}
	base := strings.TrimRight(provider.Endpoint, "/")
	var entries []CIPPromotionEntry
	if err := cipGet(ctx, base+"/promotions", token, &entries); err != nil {
		result.Error = err.Error()
		return result
	}
	result.At, result.Promotions = time.Now(), entries
	return result
}

// FetchCIPPromotion reads one promotion, which is the only place the spec
// comes from. The spec is a snapshot taken when the promotion started, so a
// caller can cache the answer for as long as the promotion exists.
//
// A spec that does not parse is not a failed read: the stage rows still
// draw the flow, only the edges and the gate reasons are missing.
func FetchCIPPromotion(ctx context.Context, provider config.Widget, token string, id int) CIPPromotionDetail {
	result := CIPPromotionDetail{SchemaVersion: 1, Name: provider.Name, At: time.Now(),
		Promotion: CIPPromotion{ID: id}}
	if strings.TrimSpace(token) == "" {
		result.Error = "no token configured: set token_env or token_file"
		return result
	}
	base := strings.TrimRight(provider.Endpoint, "/")
	var entry CIPPromotionEntry
	if err := cipGet(ctx, fmt.Sprintf("%s/promotions/%d", base, id), token, &entry); err != nil {
		result.Error = err.Error()
		return result
	}
	result.At = time.Now()
	result.Promotion, result.Stages = entry.Promotion, entry.Stages
	if result.Promotion.ID == 0 {
		result.Promotion.ID = id
	}
	if spec := strings.TrimSpace(entry.Promotion.SpecJSON); spec != "" {
		// Ignore a parse failure on purpose. The flow degrades to a list of
		// stages without edges, which is better than losing the promotion.
		_ = json.Unmarshal([]byte(spec), &result.Spec)
	}
	return result
}

// CIPStageColumns groups the stages into dependency columns, using the
// needs the spec declares. Without a spec every stage lands in the first
// column, so the flow still shows every stage while the spec loads.
func CIPStageColumns(stages []CIPStage, spec CIPSpec) [][]CIPStage {
	if len(stages) == 0 {
		return nil
	}
	needs := make([][]string, len(stages))
	names := make([]string, len(stages))
	for i, stage := range stages {
		names[i] = stage.Stage
		for _, declared := range spec.Stages {
			if declared.Name == stage.Stage {
				needs[i] = declared.Needs
				break
			}
		}
	}
	columns := make([][]CIPStage, 0)
	for _, group := range dependencyColumns(names, needs) {
		column := make([]CIPStage, 0, len(group))
		for _, i := range group {
			column = append(column, stages[i])
		}
		columns = append(columns, column)
	}
	return columns
}
