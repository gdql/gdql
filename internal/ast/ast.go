package ast

// Query is the top-level AST node for any GDQL query.
type Query interface {
	queryNode()
}

func (*ShowQuery) queryNode()        {}
func (*SongQuery) queryNode()        {}
func (*PerformanceQuery) queryNode() {}
func (*SetlistQuery) queryNode()    {}
func (*CountQuery) queryNode()      {}
func (*FirstLastQuery) queryNode()  {}
func (*RandomShowQuery) queryNode() {}

// ShowQuery represents: SHOWS [AT "venue"] [TOUR "name"] [FROM date_range] [WHERE conditions] [modifiers]
type ShowQuery struct {
	At        string // venue name filter
	Tour      string // tour name filter
	From      *DateRange
	Where     *WhereClause
	OrderBy   *OrderClause
	Limit     *int
	OutputFmt OutputFormat
}

// SongQuery represents: SONGS [WITH clause] [WRITTEN clause] [modifiers]
type SongQuery struct {
	With      *WithClause
	Written   *DateRange
	From      *DateRange // SONGS FROM 1977 / SONGS PLAYED IN 1977
	OrderBy   *OrderClause
	Limit     *int
	OutputFmt OutputFormat
}

// PerformanceQuery represents: PERFORMANCES OF song [FROM range] [WITH clause]
type PerformanceQuery struct {
	Song    *SongRef
	From    *DateRange
	With    *WithClause
	OrderBy *OrderClause
	Limit   *int
}

// SetlistQuery represents: SETLIST FOR date [AS format]
type SetlistQuery struct {
	Date      *Date
	OutputFmt OutputFormat
}

// FirstLastQuery represents: FIRST "Song" or LAST "Song"
type FirstLastQuery struct {
	Song  *SongRef
	IsLast bool // false = FIRST, true = LAST
}

// RandomShowQuery represents: RANDOM SHOW [FROM date_range]
type RandomShowQuery struct {
	From *DateRange
}

// CountQuery represents: COUNT "Song Name" [FROM date_range] or COUNT SHOWS [FROM date_range] [WHERE ...]
type CountQuery struct {
	Song       *SongRef     // nil for COUNT SHOWS
	CountShows bool         // true for COUNT SHOWS
	From       *DateRange
	Where      *WhereClause // optional WHERE conditions (COUNT SHOWS WHERE ...)
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
func (*SegueIntoCondition) conditionNode()    {}
func (*NegatedSegueCondition) conditionNode() {}

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
// When SegueChain is set, Song is nil and the condition uses a segue chain
// (e.g., OPENER ("Help on the Way" > "Slipknot!")).
type PositionCondition struct {
	Set        SetPosition
	Operator   PositionOp
	Song       *SongRef
	SegueChain *SegueCondition
	Negated    bool
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

// PlayedCondition represents: PLAYED "Song" or NOT PLAYED "Song"
type PlayedCondition struct {
	Song    *SongRef
	Negated bool
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

// NegatedSegueCondition represents: "Song A" NOT > "Song B"
// Matches shows where Song A was played and the next song was NOT Song B.
type NegatedSegueCondition struct {
	Song    *SongRef // the song that was played
	NotSong *SongRef // the song that did NOT follow
}

// SegueIntoCondition represents a standalone segue operator before a song:
// >"Song" (segued into), >>"Song" (then played), ~>"Song" (teased into).
type SegueIntoCondition struct {
	Song     *SongRef
	Operator SegueOp
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
	OutputCount
)
