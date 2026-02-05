package formatter

import (
	"fmt"
	"strings"

	"github.com/gdql/gdql/internal/data"
	"github.com/gdql/gdql/internal/executor"
)

func formatSetlist(result *executor.Result) (string, error) {
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
