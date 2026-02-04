package jellyfin

import (
	"context"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

// /Persons
//
// personsHandler returns a list of persons
func (j *Jellyfin) personsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()
	// parentID := queryparams.Get("appearsInItemId")
	searchTerm := queryparams.Get("searchTerm")

	var persons []JFItem

	if searchTerm != "" {
		searchTerm, err := url.QueryUnescape(searchTerm)
		if err != nil {
			apierror(w, "Invalid search term", http.StatusBadRequest)
			return
		}
		personNames, err := j.collections.SearchPerson(r.Context(), searchTerm)
		if personNames == nil || err != nil {
			apierror(w, "Search index not available", http.StatusInternalServerError)
			return
		}
		log.Printf("personsHandler: search found %d matching items\n", len(personNames))

		// Populate persons list based on found person names
		for _, name := range personNames {
			person, err := j.makeJFItemPerson(r.Context(), accessToken.UserID, makeJFPersonID(name))
			if err != nil {
				continue
			}
			persons = append(persons, person)
		}
	}

	persons = j.applyItemsFilter(persons, queryparams)

	totalItemCount := len(persons)
	responseItems, startIndex := j.applyItemPaginating(j.applyItemSorting(persons, queryparams), queryparams)
	response := UserItemsResponse{
		Items:            responseItems,
		StartIndex:       startIndex,
		TotalRecordCount: totalItemCount,
	}
	serveJSON(response, w)
}

// /Person/{name}
//
// // personHandler returns details of a specific person
func (j *Jellyfin) personHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	name := vars["name"]
	if name == "" {
		apierror(w, "Missing person name", http.StatusBadRequest)
		return
	}
	name, err := url.QueryUnescape(name)
	if err != nil {
		apierror(w, "Invalid person name", http.StatusBadRequest)
		return
	}
	response, err := j.makeJFItemPerson(r.Context(), accessToken.UserID, makeJFPersonID(name))
	if err != nil {
		apierror(w, "could not create person item", http.StatusInternalServerError)
		return
	}
	serveJSON(response, w)
}

// makeJFItemPerson creates a JFItem representing a person
func (j *Jellyfin) makeJFItemPerson(ctx context.Context, userID string, personID string) (JFItem, error) {
	name, err := decodeJFPersonID(personID)
	if err != nil {
		return JFItem{}, err
	}
	p, err := j.repo.GetPersonByName(ctx, name, userID)
	if err != nil {
		return JFItem{}, err
	}
	response := JFItem{
		ID:                  personID,
		ServerID:            j.serverID,
		Type:                itemTypePerson,
		Name:                p.Name,
		SortName:            p.Name,
		Etag:                personID,
		DateCreated:         p.Created,
		PremiereDate:        p.DateOfBirth,
		LocationType:        "FileSystem",
		MediaType:           "Unknown",
		Overview:            p.Bio,
		PlayAccess:          "Full",
		ProductionLocations: []string{},
		ImageBlurHashes:     &JFImageBlurHashes{},
		BackdropImageTags:   []string{},
		People:              []JFPeople{},
		Studios:             []JFStudios{},
		Genres:              []string{},
		GenreItems:          []JFGenreItem{},
		LockedFields:        []string{},
		Taglines:            []string{},
		Tags:                []string{},
		ProviderIds: JFProviderIds{
			Tmdb: p.ID,
		},
		UserData: &JFUserData{
			Key:    "Person-" + p.Name,
			ItemID: personID,
		},
		ChildCount: 1,
	}
	if p.PlaceOfBirth != "" {
		response.ProductionLocations = []string{
			p.PlaceOfBirth,
		}
	}
	if p.PosterURL != "" {
		response.ImageTags = &JFImageTags{
			Primary: tagprefix_redirect + p.PosterURL,
		}
	}
	return response, nil
}
