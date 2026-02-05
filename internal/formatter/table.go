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
	default:
		return "", nil
	}
}

func tableShows(shows []*data.Show) string {
	if len(shows) == 0 {
		return "No shows found."
	}
	var b strings.Builder
	b.WriteString("DATE       | VENUE            | CITY         | STATE\n")
	b.WriteString("-----------+------------------+--------------+-----\n")
	for _, s := range shows {
		date := s.Date.Format("2006-01-02")
		venue := truncate(s.Venue, 16)
		city := truncate(s.City, 12)
		state := truncate(s.State, 5)
		fmt.Fprintf(&b, "%-10s | %-16s | %-12s | %s\n", date, venue, city, state)
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
	var b strings.Builder
	b.WriteString("SHOW_ID | SET | POS | SEGUE | LENGTH\n")
	b.WriteString("--------+-----+-----+-------+-------\n")
	for _, p := range perfs {
		seg := p.SegueType
		if seg == "" {
			seg = "-"
		}
		fmt.Fprintf(&b, "%7d | %3d | %3d | %-5s | %d\n", p.ShowID, p.SetNumber, p.Position, seg, p.LengthSeconds)
	}
	return b.String()
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
	if len(s) <= max {
		return s
	}
	return s[:max]
}
