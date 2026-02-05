package ast

// Query is the top-level AST node for any GDQL query.
type Query interface {
	queryNode()
}

func (*ShowQuery) queryNode()        {}
func (*SongQuery) queryNode()        {}
func (*PerformanceQuery) queryNode() {}
func (*SetlistQuery) queryNode()    {}

// ShowQuery represents: SHOWS [FROM date_range] [WHERE conditions] [modifiers]
type ShowQuery struct {
	From      *DateRange
	Where     *WhereClause
	OrderBy   *OrderClause
	Limit     *int
	OutputFmt OutputFormat
}

// SongQuery represents: SONGS [WITH clause] [WRITTEN clause] [modifiers]
type SongQuery struct {
	With    *WithClause
	Written *DateRange
	OrderBy *OrderClause
	Limit   *int
}

// PerformanceQuery represents: PERFORMANCES OF song [FROM range] [WITH clause]
type PerformanceQuery struct {
	Song    *SongRef
	From    *DateRange
	With    *WithClause
	OrderBy *OrderClause
	Limit   *int
}

// SetlistQuery represents: SETLIST FOR date
type SetlistQuery struct {
	Date *Date
}

// DateRange represents date ranges: 1977, 1977-1980, 5/8/77, spring-77
type DateRange struct {
	Start *Date
	End   *Date
	Era   *EraAlias
}

// Date represents a date (year, optional month/day, optional season).
type Date struct {
	Year   int
	Month  int
	Day    int
	Season string
}

// EraAlias is a named era (e.g. PRIMAL, EUROPE72).
type EraAlias int

const (
	EraPrimal EraAlias = iota
	EraEurope72
	EraWallOfSound
	EraHiatus
	EraBrent
	EraVince
)

// WhereClause represents WHERE conditions.
type WhereClause struct {
	Conditions []Condition
	Operators  []LogicOp
}

// LogicOp is AND or OR between conditions.
type LogicOp int

const (
	OpAnd LogicOp = iota
	OpOr
)

// Condition is implemented by all condition node types.
type Condition interface {
	conditionNode()
}

func (*SegueCondition) conditionNode()     {}
func (*PositionCondition) conditionNode()  {}
func (*PlayedCondition) conditionNode()   {}
func (*LengthCondition) conditionNode()   {}
func (*GuestCondition) conditionNode()     {}

// SegueCondition represents: "Song A" > "Song B" > "Song C"
type SegueCondition struct {
	Songs     []*SongRef
	Operators []SegueOp
}

// SegueOp is the type of transition between songs.
type SegueOp int

const (
	SegueOpSegue SegueOp = iota // >
	SegueOpBreak                 // >>
	SegueOpTease                 // ~>
)

// PositionCondition represents: SET1 OPENED "Song", ENCORE = "Song"
type PositionCondition struct {
	Set      SetPosition
	Operator PositionOp
	Song     *SongRef
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

// PlayedCondition represents: PLAYED "Song"
type PlayedCondition struct {
	Song *SongRef
}

// LengthCondition represents: LENGTH("Song") > 20min or LENGTH > 20min
type LengthCondition struct {
	Song     *SongRef // optional, for PERFORMANCES OF "X" WITH LENGTH > 20
	Operator CompOp
	Duration string // e.g. "20min"
}

// GuestCondition represents: GUEST "Name"
type GuestCondition struct {
	Name string
}

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

// SongRef is a reference to a song by name.
type SongRef struct {
	Name    string
	Negated bool
}

// WithClause represents WITH conditions.
type WithClause struct {
	Conditions []WithCondition
}

// WithCondition is implemented by LYRICS, LENGTH, GUEST conditions.
type WithCondition interface {
	withConditionNode()
}

func (*LyricsCondition) withConditionNode() {}
func (*LengthWithCondition) withConditionNode() {}
func (*GuestWithCondition) withConditionNode() {}

// LyricsCondition represents: LYRICS("word1", "word2")
type LyricsCondition struct {
	Words    []string
	Operator LogicOp
}

// LengthWithCondition represents: LENGTH > 20min
type LengthWithCondition struct {
	Operator CompOp
	Duration string
}

// GuestWithCondition represents: GUEST "Name"
type GuestWithCondition struct {
	Name string
}

// OrderClause represents ORDER BY field [ASC|DESC]
type OrderClause struct {
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
