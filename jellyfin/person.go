package jellyfin

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/database/model"
)

// /Persons
//
// personsHandler returns a list of persons
func (j *Jellyfin) personsHandler(w http.ResponseWriter, r *http.Request) {
	response := UserItemsResponse{
		Items:            []JFItem{},
		TotalRecordCount: 0,
		StartIndex:       0,
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

	dbperson, err := j.repo.GetPersonByName(r.Context(), name, accessToken.UserID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}

	response := j.makeJFItemPerson(r.Context(), dbperson)
	serveJSON(response, w)
}

func (j *Jellyfin) makeJFItemPerson(_ context.Context, p *model.Person) JFItem {
	id := makeJFPersonID(p.Name)
	response := JFItem{
		ID:                  id,
		ServerID:            j.serverID,
		Type:                itemTypePerson,
		Name:                p.Name,
		SortName:            p.Name,
		Etag:                id,
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
			ItemID: id,
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
	return response
}
