package formatter

import (
	"fmt"
	"strings"

	"github.com/gdql/gdql/internal/executor"
)

// formatTSV writes tab-separated values. Same column layout as formatCSV but
// uses tabs and skips quoting — TSV is the right choice for piping into Excel,
// Google Sheets, or any tool that prefers tabs over comma escaping.
func formatTSV(result *executor.Result) (string, error) {
	var b strings.Builder
	switch result.Type {
	case executor.ResultShows:
		writeTSVRow(&b, "id", "date", "venue", "city", "state", "tour")
		for _, s := range result.Shows {
			writeTSVRow(&b,
				fmt.Sprint(s.ID), s.Date.Format("2006-01-02"),
				s.Venue, s.City, s.State, s.Tour,
			)
		}
	case executor.ResultSongs:
		writeTSVRow(&b, "id", "name", "short_name", "writers", "times_played")
		for _, s := range result.Songs {
			writeTSVRow(&b, fmt.Sprint(s.ID), s.Name, s.ShortName, s.Writers, fmt.Sprint(s.TimesPlayed))
		}
	case executor.ResultPerformances:
		writeTSVRow(&b, "id", "show_id", "song_id", "set_number", "position", "segue_type", "length_seconds")
		for _, p := range result.Performances {
			writeTSVRow(&b,
				fmt.Sprint(p.ID), fmt.Sprint(p.ShowID), fmt.Sprint(p.SongID),
				fmt.Sprint(p.SetNumber), fmt.Sprint(p.Position), p.SegueType, fmt.Sprint(p.LengthSeconds),
			)
		}
	case executor.ResultSetlist:
		if result.Setlist != nil {
			writeTSVRow(&b, "set_number", "position", "segue_type", "length_seconds")
			for _, p := range result.Setlist.Performances {
				writeTSVRow(&b, fmt.Sprint(p.SetNumber), fmt.Sprint(p.Position), p.SegueType, fmt.Sprint(p.LengthSeconds))
			}
		}
	case executor.ResultCount:
		if result.Count != nil {
			writeTSVRow(&b, "song", "count")
			writeTSVRow(&b, result.Count.SongName, fmt.Sprint(result.Count.Count))
		}
	}
	return b.String(), nil
}

// writeTSVRow strips tabs and newlines from each cell so the row stays on one
// line and one delimiter. Keeps the format brain-dead-simple to parse.
func writeTSVRow(b *strings.Builder, cells ...string) {
	for i, c := range cells {
		if i > 0 {
			b.WriteByte('\t')
		}
		c = strings.ReplaceAll(c, "\t", " ")
		c = strings.ReplaceAll(c, "\n", " ")
		b.WriteString(c)
	}
	b.WriteByte('\n')
}
