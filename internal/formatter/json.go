package formatter

import (
	"encoding/json"
	"github.com/gdql/gdql/internal/executor"
)

func formatJSON(result *executor.Result) (string, error) {
	out := map[string]interface{}{
		"type":     resultTypeStr(result.Type),
		"duration": result.Duration.String(),
	}
	switch result.Type {
	case executor.ResultShows:
		out["shows"] = result.Shows
	case executor.ResultSongs:
		out["songs"] = result.Songs
	case executor.ResultPerformances:
		out["performances"] = result.Performances
	case executor.ResultSetlist:
		out["setlist"] = result.Setlist
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func resultTypeStr(t executor.ResultType) string {
	switch t {
	case executor.ResultShows:
		return "shows"
	case executor.ResultSongs:
		return "songs"
	case executor.ResultPerformances:
		return "performances"
	case executor.ResultSetlist:
		return "setlist"
	}
	return ""
}
