package formatter

import (
	"fmt"
	"strings"

	"github.com/gdql/gdql/internal/data"
	"github.com/gdql/gdql/internal/executor"
)

func formatTable(result *executor.Result) (string, error) {
	switch result.Type {
	case executor.ResultShows:
		return tableShows(result.Shows), nil
	case executor.ResultSongs:
		return tableSongs(result.Songs), nil
	case executor.ResultPerformances:
		return tablePerformances(result.Performances), nil
	case executor.ResultSetlist:
		return tableSetlist(result.Setlist), nil
	case executor.ResultCount:
		return tableCount(result.Count), nil
	default:
		return "", nil
	}
}

func tableCount(cr *executor.CountResult) string {
	if cr == nil {
		return "0"
	}
	if cr.SongName != "" {
		return fmt.Sprintf("%s: %d\n", cr.SongName, cr.Count)
	}
	return fmt.Sprintf("%d\n", cr.Count)
}

func tableShows(shows []*data.Show) string {
	if len(shows) == 0 {
		return "No shows found."
	}
	var b strings.Builder
	b.WriteString("DATE       | VENUE                          | CITY                     | STATE\n")
	b.WriteString("-----------+--------------------------------+--------------------------+------\n")
	for _, s := range shows {
		date := s.Date.Format("2006-01-02")
		venue := truncate(s.Venue, 30)
		city := truncate(s.City, 24)
		state := truncate(s.State, 5)
		fmt.Fprintf(&b, "%-10s | %-30s | %-24s | %s\n", date, venue, city, state)
	}
	return b.String()
}

func tableSongs(songs []*data.Song) string {
	if len(songs) == 0 {
		return "No songs found."
	}
	var b strings.Builder
	b.WriteString("NAME                 | TIMES_PLAYED\n")
	b.WriteString("---------------------+-------------\n")
	for _, s := range songs {
		name := truncate(s.Name, 19)
		fmt.Fprintf(&b, "%-20s | %d\n", name, s.TimesPlayed)
	}
	return b.String()
}

func tablePerformances(perfs []*data.Performance) string {
	if len(perfs) == 0 {
		return "No performances found."
	}
	// Check if any performance has length data
	hasLength := false
	for _, p := range perfs {
		if p.LengthSeconds > 0 {
			hasLength = true
			break
		}
	}
	var b strings.Builder
	if hasLength {
		b.WriteString("SHOW_ID | SET | POS | SEGUE | LENGTH\n")
		b.WriteString("--------+-----+-----+-------+-------\n")
		for _, p := range perfs {
			seg := p.SegueType
			if seg == "" {
				seg = "-"
			}
			fmt.Fprintf(&b, "%7d | %3d | %3d | %-5s | %s\n", p.ShowID, p.SetNumber, p.Position, seg, formatLength(p.LengthSeconds))
		}
	} else {
		b.WriteString("SHOW_ID | SET | POS | SEGUE\n")
		b.WriteString("--------+-----+-----+------\n")
		for _, p := range perfs {
			seg := p.SegueType
			if seg == "" {
				seg = "-"
			}
			fmt.Fprintf(&b, "%7d | %3d | %3d | %s\n", p.ShowID, p.SetNumber, p.Position, seg)
		}
	}
	return b.String()
}

func formatLength(seconds int) string {
	if seconds <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func tableSetlist(sl *executor.SetlistResult) string {
	if sl == nil || len(sl.Performances) == 0 {
		return "No setlist."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Setlist for %s (show_id=%d)\n\n", sl.Date.Format("2006-01-02"), sl.ShowID)
	b.WriteString("SET | POS | SEGUE | SONG\n")
	b.WriteString("----+-----+-------+----------------------------\n")
	for _, p := range sl.Performances {
		seg := p.SegueType
		if seg == "" {
			seg = "-"
		}
		name := p.SongName
		if name == "" {
			name = "?"
		}
		name = truncate(name, 28)
		fmt.Fprintf(&b, "%3d | %3d | %-5s | %s\n", p.SetNumber, p.Position, seg, name)
	}
	return b.String()
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
