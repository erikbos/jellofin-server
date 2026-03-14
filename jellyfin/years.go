package jellyfin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"
)

// /Years
//
// yearsHandler returns item summaries for all years.
func (j *Jellyfin) yearsHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	yearStats, err := j.getYearStats(r.Context(), reqCtx.User.ID, r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	years := []JFItem{}
	for year, stats := range yearStats {
		if item, err := j.makeJFItemYear(year, stats); err == nil {
			years = append(years, item)
		}
	}

	totalItemCount := len(years)
	queryparams := r.URL.Query()
	years, _ = j.applyItemPaginating(j.applyItemSorting(years, queryparams), queryparams)

	response := UserItemsResponse{
		Items:            years,
		TotalRecordCount: totalItemCount,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// /Years/{year}
//
// yearHandler returns item summary item for a specific year.
func (j *Jellyfin) yearHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	yearStats, err := j.getYearStats(r.Context(), reqCtx.User.ID, r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	year, _ := strconv.Atoi(vars["year"])
	stats, ok := yearStats[year]
	if !ok {
		http.Error(w, "Year not found", http.StatusNotFound)
		return
	}

	response, err := j.makeJFItemYear(year, stats)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	serveJSON(response, w)
}

type yearStats struct {
	movies int
	series int
}

func (j *Jellyfin) getYearStats(ctx context.Context, userID string, params url.Values) (map[int]yearStats, error) {
	parentID := params.Get("parentId")

	var items []JFItem
	var err error
	if parentID != "" {
		items, err = j.getJFItemsByParentID(ctx, userID, parentID)
		if err != nil {
			return nil, err
		}
	} else {
		// All items recursively
		items, err = j.getJFItemsAll(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	items = j.applyItemsFilter(items, params)

	yearMap := make(map[int]yearStats)
	for _, item := range items {
		y := item.PremiereDate.Year()
		if y > 0 {
			switch item.Type {
			case itemTypeMovie:
				yr := yearMap[y]
				yr.movies++
				yearMap[y] = yr
			case itemTypeShow:
				yr := yearMap[y]
				yr.series++
				yearMap[y] = yr
			}
		}
	}
	return yearMap, nil
}

func (j *Jellyfin) makeJFItemYear(year int, stats yearStats) (JFItem, error) {
	ID := makeJFYearID(year)
	response := JFItem{
		ID:                ID,
		Etag:              ID,
		ParentID:          j.serverID,
		Name:              strconv.Itoa(year),
		SortName:          fmt.Sprintf("%010d", year),
		Type:              "Year",
		MovieCount:        stats.movies,
		SeriesCount:       stats.series,
		BackdropImageTags: []string{},
		ExternalUrls:      []JFExternalUrls{},
		Genres:            []string{},
		GenreItems:        []JFGenreItem{},
		LocationType:      "FileSystem",
		LockData:          false,
		LockedFields:      []string{},
		Path:              fmt.Sprintf("/years/%d", year),
		People:            []JFPeople{},
		PlayAccess:        "Full",
		RemoteTrailers:    []JFRemoteTrailers{},
		Studios:           []JFStudios{},
		Tags:              []string{},
		UserData: &JFUserData{
			ItemID: ID,
			Key:    fmt.Sprintf("Year-%d", year),
		},
		Taglines:  []string{},
		ImageTags: &JFImageTags{},
	}
	return response, nil
}

// makeJFYearID returns an external id for a year.
func makeJFYearID(year int) string {
	return encodeExternalName(itemprefix_year, strconv.Itoa(year))
}
