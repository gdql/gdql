package formatter

import (
	"fmt"
	"strings"

	"github.com/gdql/gdql/internal/data"
	"github.com/gdql/gdql/internal/executor"
)

func formatSetlist(result *executor.Result) (string, error) {
	// Multi-show setlist (AS SETLIST on SHOWS query)
	if len(result.Setlists) > 0 {
		return formatMultiSetlist(result.Setlists)
	}
	if result.Type != executor.ResultSetlist || result.Setlist == nil {
		return formatTable(result)
	}
	sl := result.Setlist
	var b strings.Builder
	fmt.Fprintf(&b, "Setlist — %s\n\n", sl.Date.Format("Monday, January 2, 2006"))
	set := -1
	for _, p := range sl.Performances {
		if p.SetNumber != set {
			set = p.SetNumber
			b.WriteString(fmtSetName(set))
			b.WriteString("\n")
		}
		seg := ""
		if p.SegueType != "" {
			seg = " " + p.SegueType + " "
		}
		name := p.SongName
		if name == "" {
			name = "?"
		}
		fmt.Fprintf(&b, "  %d.%s%s%s\n", p.Position, seg, name, fmtPerformance(p))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func formatMultiSetlist(setlists []*executor.SetlistResult) (string, error) {
	var b strings.Builder
	for i, sl := range setlists {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		fmt.Fprintf(&b, "Setlist — %s (show_id=%d)\n\n", sl.Date.Format("Monday, January 2, 2006"), sl.ShowID)
		set := -1
		for _, p := range sl.Performances {
			if p.SetNumber != set {
				set = p.SetNumber
				b.WriteString(fmtSetName(set))
				b.WriteString("\n")
			}
			name := p.SongName
			if name == "" {
				name = "?"
			}
			seg := ""
			if p.SegueType != "" {
				seg = " " + p.SegueType + " "
			}
			fmt.Fprintf(&b, "  %d.%s%s%s\n", p.Position, seg, name, fmtPerformance(p))
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func fmtSetName(setNum int) string {
	switch setNum {
	case 1:
		return "Set 1"
	case 2:
		return "Set 2"
	case 3:
		return "Set 3 / Encore"
	case 0:
		return "Soundcheck"
	}
	return fmt.Sprintf("Set %d", setNum)
}

func fmtPerformance(p *data.Performance) string {
	if p.LengthSeconds > 0 {
		return fmt.Sprintf(" (%dm)", p.LengthSeconds/60)
	}
	return ""
}
