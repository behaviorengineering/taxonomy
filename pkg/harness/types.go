package harness

import (
	"context"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
)

const (
	ChoiceSkip          = "skip"
	ChoicePrefixUse     = "use:"
	ChoiceAcceptDraft   = "accept_draft"
	ChoiceReject        = "reject"
	DraftKindNewLeaf    = "new_leaf"
	DraftKindAlias      = "alias"
	DraftKindBreadcrumb = "breadcrumb"
)

// Strategy selects Operate behavior.
type Strategy string

const (
	StrategyWalk   Strategy = "walk"
	StrategyAttach Strategy = "attach"
)

// PackedOption is one closed choice for Judge seats.
type PackedOption struct {
	Choice      string
	Label       string
	Description string
	ParentID    string
	LeafID      string
}

// DecideIn is input for the primary Judge pass.
type DecideIn struct {
	WorldContext string
	Text         string
	Options      []PackedOption
	Path         []string
}

// DecideOut is output from Judge.
type DecideOut struct {
	Choice string
	Score  float64
}

// DraftIn is input for Author when Judge skips or scores low.
type DraftIn struct {
	WorldContext string
	Text         string
	Parents      []PackedOption
	Reason       string
}

// DraftOut is a proposed vocabulary mutation.
type DraftOut struct {
	Kind        string
	ID          string
	Parent      string
	Label       string
	Description string
	LeafID      string
	Alias       string
}

// EssenceIn is input for Essencer (Attach strategy).
type EssenceIn struct {
	WorldContext string
	Text         string
}

// EssenceOut is structured essence from Essencer.
type EssenceOut struct {
	Why   [5]string
	About string
	Shape string
	Kind  string
}

// Op is one Operate invocation.
type Op struct {
	WorldContext string
	Text         string
	Catalog      *catalog.Catalog
}

// Result is the outcome of Operate.
type Result struct {
	Assigned      []catalog.Assignment
	Draft         *DraftOut
	DraftAccepted bool
	JudgeScore    float64
	GateScore     float64
	Path          []string
	Strategy      Strategy
	Kind          string
	About         string
	Shape         string
	Cosine        float64
	CanonicalID   string
	Reinforced    bool
	Essence       *EssenceOut
}

// Judge scores text against a closed option set.
type Judge interface {
	Decide(ctx context.Context, in DecideIn) (DecideOut, error)
}

// Author drafts a new leaf or alias when Judge does not assign.
type Author interface {
	Draft(ctx context.Context, in DraftIn) (DraftOut, error)
}

// Embedder returns embedding vectors for each input string.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// Essencer extracts essence fields from message text.
type Essencer interface {
	Essence(ctx context.Context, in EssenceIn) (EssenceOut, error)
}
