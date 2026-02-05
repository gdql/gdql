package formatter

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/gdql/gdql/internal/executor"
)

func formatCSV(result *executor.Result) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	switch result.Type {
	case executor.ResultShows:
		w.Write([]string{"id", "date", "venue_id", "venue", "city", "state", "notes", "rating"})
		for _, s := range result.Shows {
			w.Write([]string{
				fmt.Sprint(s.ID), s.Date.Format("2006-01-02"), fmt.Sprint(s.VenueID),
				s.Venue, s.City, s.State, s.Notes, fmt.Sprint(s.Rating),
			})
		}
	case executor.ResultSongs:
		w.Write([]string{"id", "name", "short_name", "writers", "times_played"})
		for _, s := range result.Songs {
			w.Write([]string{fmt.Sprint(s.ID), s.Name, s.ShortName, s.Writers, fmt.Sprint(s.TimesPlayed)})
		}
	case executor.ResultPerformances:
		w.Write([]string{"id", "show_id", "song_id", "set_number", "position", "segue_type", "length_seconds"})
		for _, p := range result.Performances {
			w.Write([]string{
				fmt.Sprint(p.ID), fmt.Sprint(p.ShowID), fmt.Sprint(p.SongID),
				fmt.Sprint(p.SetNumber), fmt.Sprint(p.Position), p.SegueType, fmt.Sprint(p.LengthSeconds),
			})
		}
	case executor.ResultSetlist:
		if result.Setlist != nil {
			w.Write([]string{"set_number", "position", "segue_type", "length_seconds"})
			for _, p := range result.Setlist.Performances {
				w.Write([]string{fmt.Sprint(p.SetNumber), fmt.Sprint(p.Position), p.SegueType, fmt.Sprint(p.LengthSeconds)})
			}
		}
	}
	w.Flush()
	return b.String(), w.Error()
}
