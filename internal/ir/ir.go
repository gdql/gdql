package ir

import "time"

// QueryType identifies the kind of query.
type QueryType int

const (
	QueryTypeShows QueryType = iota
	QueryTypeSongs
	QueryTypePerformances
	QueryTypeSetlist
)

// QueryIR is the resolved, expanded representation ready for SQL generation.
type QueryIR struct {
	Type       QueryType
	DateRange  *ResolvedDateRange
	SingleDate *time.Time // for SETLIST FOR date
	SongID     *int       // for PERFORMANCES OF song
	SegueChain *SegueChainIR
	Conditions []ConditionIR
	OrderBy    *OrderByIR
	Limit      *int
	OutputFmt  OutputFormat
}

// ResolvedDateRange has concrete dates (no eras).
type ResolvedDateRange struct {
	Start time.Time
	End   time.Time
}

// SegueChainIR has resolved song IDs and segue operators.
type SegueChainIR struct {
	SongIDs   []int
	Operators []SegueOp
}

// SegueOp is the type of transition between songs.
type SegueOp int

const (
	SegueOpSegue SegueOp = iota // >
	SegueOpBreak                 // >>
	SegueOpTease                 // ~>
)

// ConditionIR is a resolved condition (tagging interface).
type ConditionIR interface {
	conditionIRNode()
}

func (*PositionConditionIR) conditionIRNode() {}
func (*LyricsConditionIR) conditionIRNode() {}
func (*LengthConditionIR) conditionIRNode() {}
func (*PlayedConditionIR) conditionIRNode() {}
func (*GuestConditionIR) conditionIRNode()  {}

// PositionConditionIR: SET1 OPENED "Song", ENCORE = "Song"
type PositionConditionIR struct {
	Set      SetPosition
	Operator PositionOp
	SongID   int
}

// LyricsConditionIR: LYRICS("word1", "word2")
type LyricsConditionIR struct {
	Words    []string
	Operator LogicOp
}

// LengthConditionIR: LENGTH > 20min
type LengthConditionIR struct {
	SongID   *int // nil for PERFORMANCES OF "X" WITH LENGTH > 20 (applies to that song)
	Operator CompOp
	Seconds  int
}

// PlayedConditionIR: PLAYED "Song"
type PlayedConditionIR struct {
	SongID int
}

// GuestConditionIR: GUEST "Name"
type GuestConditionIR struct {
	Name string
}

// SetPosition is SET1, SET2, SET3, or ENCORE.
type SetPosition int

const (
	SetAny SetPosition = iota
	Set1
	Set2
	Set3
	Encore
)

// PositionOp is OPENED, CLOSED, or =.
type PositionOp int

const (
	PosOpened PositionOp = iota
	PosClosed
	PosEquals
)

// CompOp is a comparison operator.
type CompOp int

const (
	CompGT CompOp = iota
	CompLT
	CompEQ
	CompGTE
	CompLTE
	CompNEQ
)

// LogicOp is AND or OR.
type LogicOp int

const (
	OpAnd LogicOp = iota
	OpOr
)

// OrderByIR: ORDER BY field DESC
type OrderByIR struct {
	Field string
	Desc  bool
}

// OutputFormat for result formatting.
type OutputFormat int

const (
	OutputDefault OutputFormat = iota
	OutputJSON
	OutputCSV
	OutputSetlist
	OutputCalendar
	OutputTable
)
