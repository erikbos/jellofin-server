package jellyfin

import (
	"log"
	"net/http"
	"sort"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/collection"
)

// /Shows/rXlq4EHNxq4HIVQzw3o2/Episodes?UserId=2b1ec0a52b09456c9823a367d84ac9e5&ExcludeLocationTypes=Virtual&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&SeasonId=rXlq4EHNxq4HIVQzw3o2/1
//
// generate episode overview for one season of a show
func (j *Jellyfin) showsEpisodesHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	queryparams := r.URL.Query()

	_, i := j.collections.GetItemByID(vars["show"])
	if i == nil {
		apierror(w, "Show not found", http.StatusNotFound)
		return
	}
	show, ok := i.(*collection.Show)
	if !ok {
		apierror(w, "Item is not a show", http.StatusInternalServerError)
		return
	}

	// Create API response for all episodes of the show
	episodes := make([]JFItem, 0)
	for _, s := range show.Seasons {
		if episodesOfSeason, err := j.makeJFEpisodesOverview(r.Context(), accessToken.UserID, &s); err == nil {
			episodes = append(episodes, episodesOfSeason...)
		}
	}

	// Apply filtering, e.g. if a particular season is requested ("seasonId")
	episodes = j.applyItemsFilter(episodes, queryparams)

	episodes = j.applyItemSorting(episodes, queryparams)

	response := UserItemsResponse{
		Items:            episodes,
		TotalRecordCount: len(episodes),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// /Shows/4QBdg3S803G190AgFrBf/Seasons?UserId=2b1ec0a52b09456c9823a367d84ac9e5&ExcludeLocationTypes=Virtual&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount
//
// showsSeasonsHandler returns a list of seasons for a specific show
func (j *Jellyfin) showsSeasonsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	queryparams := r.URL.Query()

	showID := vars["show"]
	_, i := j.collections.GetItemByID(showID)
	if i == nil {
		apierror(w, "Show not found", http.StatusNotFound)
		return
	}
	show, ok := i.(*collection.Show)
	if !ok {
		apierror(w, "Item is not a show", http.StatusInternalServerError)
		return
	}

	seasons, err := j.makeJFSeasonsOverview(r.Context(), accessToken.UserID, show)
	if err != nil {
		apierror(w, "Could not generate seasons overview", http.StatusInternalServerError)
		return
	}

	seasons = j.applyItemsFilter(seasons, queryparams)

	// Always sort seasons by number, no user provided sortBy option.
	// This way season 99, Specials ends up last.
	sort.SliceStable(seasons, func(i, j int) bool {
		return seasons[i].IndexNumber < seasons[j].IndexNumber
	})

	response := UserItemsResponse{
		Items:            seasons,
		TotalRecordCount: len(seasons),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// /Shows/NextUp?
//
//	enableImageTypes=Primary&
//	enableImageTypes=Backdrop&
//	enableImageTypes=Thumb&
//	enableResumable=false&
//	fields=MediaSourceCount&limit=20&
//
// showsNextUpHandler returns a list of next up items for the user
func (j *Jellyfin) showsNextUpHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()

	recentlyWatchedIDs, err := j.repo.GetRecentlyWatched(r.Context(), accessToken.UserID, true)
	if err != nil {
		apierror(w, "Could not get recently watched items list", http.StatusInternalServerError)
		return
	}
	nextUpItemIDs, err := j.collections.NextUp(recentlyWatchedIDs)
	if err != nil {
		apierror(w, "Could not get next up items list", http.StatusInternalServerError)
		return
	}

	items := make([]JFItem, 0, len(nextUpItemIDs))
	for _, id := range nextUpItemIDs {
		if _, i, s, e := j.collections.GetEpisodeByID(id); i != nil {
			jfitem, err := j.makeJFItemEpisode(r.Context(), accessToken.UserID, e, s.ID())
			if err == nil && j.applyItemFilter(&jfitem, queryparams) {
				items = append(items, jfitem)
			}
			continue
		}
		log.Printf("usersItemsResumeHandler: item %s not found\n", id)
	}

	items = j.applyItemsFilter(items, queryparams)

	// Apply user provided filters & sorting
	items = j.applyItemSorting(items, queryparams)

	totalItemCount := len(items)
	resumeItems, startIndex := j.applyItemPaginating(items, queryparams)
	response := JFShowsNextUpResponse{
		Items:            resumeItems,
		StartIndex:       startIndex,
		TotalRecordCount: totalItemCount,
	}
	serveJSON(response, w)
}
