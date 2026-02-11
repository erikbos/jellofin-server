package jellyfin

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/collection/metadata"
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
			if person, err := j.makeJFItemPerson(r.Context(), accessToken.UserID, makeJFPersonID(name)); err == nil {
				persons = append(persons, person)
			}
		}
	}

	// Return all persons if no search term
	if searchTerm == "" {
		personNames, err := j.GetAllPersonNames(r.Context())
		if err != nil {
			apierror(w, "Failed to get person names", http.StatusInternalServerError)
			return
		}
		log.Printf("personsHandler: found %d persons\n", len(personNames))
		persons = make([]JFItem, 0, len(personNames))
		for _, name := range personNames {
			if person, err := j.makeJFItemPerson(r.Context(), accessToken.UserID, makeJFPersonID(name)); err == nil {
				persons = append(persons, person)
			}
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
// personHandler returns details of a specific person
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
	response := JFItem{
		ID:                  personID,
		ServerID:            j.serverID,
		Type:                itemTypePerson,
		Name:                name,
		SortName:            makeSortName(name),
		Etag:                personID,
		LocationType:        "FileSystem",
		MediaType:           "Unknown",
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
		UserData: &JFUserData{
			Key:    "Person-" + name,
			ItemID: personID,
		},
		ChildCount: 1,
	}

	person, err := j.repo.GetPersonByName(ctx, name, userID)
	if err != nil {
		// If we do not have details on this person, we just return a basic response with just the name.
		// The person may still exist and just not be in our database.
		return response, nil
	}

	// Populate response with details from database
	response.Name = person.Name
	response.Overview = person.Bio
	response.DateCreated = person.Created
	response.PremiereDate = person.DateOfBirth
	if person.PlaceOfBirth != "" {
		response.ProductionLocations = []string{
			person.PlaceOfBirth,
		}
	}
	if person.PosterURL != "" {
		response.ImageTags = &JFImageTags{
			Primary: tagprefix_redirect + person.PosterURL,
		}
	}
	return response, nil
}

// makeSortName returns a name suitable for sorting.
func makeSortName(name string) string {
	// Start with lowercasing and trimming whitespace.
	sortName := strings.ToLower(strings.TrimSpace(name))

	// Remove whitespace and punctuation.
	sortName = strings.TrimLeftFunc(sortName, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
	return sortName
}

// makeJFPeople creates a list of people (actors, directors, writers) for the item
func (j *Jellyfin) makeJFPeople(_ context.Context, m metadata.Metadata, userID string) []JFPeople {
	if userID != "XAOVn7iqiBujnIQY8sd0" {
		return []JFPeople{}
	}

	actors := m.Actors()
	directors := m.Directors()
	writers := m.Writers()
	people := make([]JFPeople, 0, len(actors)+len(directors)+len(writers))
	for name, role := range actors {
		id := makeJFPersonID(name)
		people = append(people, JFPeople{ID: id, Name: name, Role: role, Type: "Actor", PrimaryImageTag: id})
	}
	for _, name := range directors {
		id := makeJFPersonID(name)
		people = append(people, JFPeople{ID: id, Name: name, Role: "Director", Type: "Director", PrimaryImageTag: id})
	}
	for _, name := range writers {
		id := makeJFPersonID(name)
		people = append(people, JFPeople{ID: id, Name: name, Role: "Screenplay", Type: "Writer", PrimaryImageTag: id})
	}
	return people
}

// makeJFPersonID returns an external id for a person.
func makeJFPersonID(name string) string {
	// base64 encoded to handle special characters, as some clients have issues with % characters in IDs.
	return itemprefix_person + base64.RawURLEncoding.EncodeToString([]byte(name))
}

// isJFPersonID checks if the provided ID is a person ID.
func isJFPersonID(id string) bool {
	return strings.HasPrefix(id, itemprefix_person)
}

// decodeJFPersonID decodes a person ID to get the original name.
func decodeJFPersonID(personID string) (string, error) {
	if !strings.HasPrefix(personID, itemprefix_person) {
		return "", errors.New("invalid person ID")
	}
	nameBytes, err := base64.RawURLEncoding.DecodeString(trimPrefix(personID))
	if err != nil {
		return "", errors.New("cannot decode person ID")
	}
	return string(nameBytes), nil
}
