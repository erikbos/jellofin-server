// middleware for normalizing request paths and query parameters to improve
// compatibility with clients that deviate from Jellyfin OpenAPI spec.
//
// E.g. /emby/Items?ParentId=123 will be normalized to /Items?parentId=123
package muxnormalizer

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
)

type Normalizer struct {
	bySegmentCount map[int][]routeTemplate
}

type routeTemplate struct {
	staticPos map[int]string
}

// New builds a full request normalizer
func New(r *mux.Router) (*Normalizer, error) {
	n := &Normalizer{
		bySegmentCount: make(map[int][]routeTemplate),
	}

	// Build route casing index from all registred routes
	err := r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		template, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}

		parts := strings.Split(template, "/")
		staticPos := make(map[int]string)

		segIndex := 0
		for _, part := range parts {
			if part == "" {
				continue
			}
			// Skip path parameters
			if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
				segIndex++
				continue
			}
			staticPos[segIndex] = part
			segIndex++
		}

		rt := routeTemplate{
			staticPos: staticPos,
		}
		n.bySegmentCount[segIndex] = append(n.bySegmentCount[segIndex], rt)

		return nil
	})

	return n, err
}

// Middleware returns an HTTP middleware that normalizes request paths and query parameters
func (n *Normalizer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Strip /emby prefix
		if strings.HasPrefix(strings.ToLower(path), "/emby/") {
			path = path[len("/emby"):]
		}

		// Remove duplicate slashes
		for strings.Contains(path, "//") {
			path = strings.ReplaceAll(path, "//", "/")
		}

		// Remove trailing slash (except for root path)
		if path != "/" && strings.HasSuffix(path, "/") {
			path = path[:len(path)-1]
		}

		// Canonicalize casing using route index
		r.URL.Path = n.normalizePath(path)

		// Tidy up query parameters
		if len(r.URL.RawQuery) > 0 {
			r.URL.RawQuery = n.normalizeQueryParameters(r.URL.RawQuery)
		}
		next.ServeHTTP(w, r)
	})
}

// normalizePath normalizes the given path using the route templates
func (n *Normalizer) normalizePath(path string) string {
	parts := strings.Split(path, "/")
	segments := make([]string, 0, len(parts))

	for _, p := range parts {
		if p != "" {
			segments = append(segments, p)
		}
	}

	if candidates := n.bySegmentCount[len(segments)]; len(candidates) > 0 {
		for _, tpl := range candidates {
			match := true
			modified := false
			newSegments := make([]string, len(segments))
			copy(newSegments, segments)

			for i, seg := range segments {
				if canonical, ok := tpl.staticPos[i]; ok {
					if strings.EqualFold(seg, canonical) {
						if seg != canonical {
							newSegments[i] = canonical
							modified = true
						}
					} else {
						match = false
						break
					}
				}
			}
			if !match {
				continue
			}
			if modified {
				path = "/" + strings.Join(newSegments, "/")
			}
			break
		}
	}
	return path
}

// normalizeQueryParameters normalizes query parameters by renaming all
// parameters to have the right casing, and removing unwanted parameters.
func (n *Normalizer) normalizeQueryParameters(rawQuery string) string {
	queryparameters, _ := url.ParseQuery(rawQuery)
	newValues := url.Values{}

	for queryParamName, values := range queryparameters {
		k := strings.ToLower(queryParamName)
		// Remove unwanted params
		if _, remove := removeParams[k]; remove {
			continue
		}
		// Rename if needed
		if newName, ok := queryParameters[k]; ok {
			queryParamName = newName
		}
		for _, v := range values {
			newValues.Add(queryParamName, v)
		}
	}
	return newValues.Encode()
}

// These are the query parameters we rename
var queryParameters = map[string]string{
	"api_key":                 "api_key",
	"apikey":                  "apiKey",
	"appearsinitemid":         "appearsInItemId",
	"code":                    "code",
	"excludeitemids":          "excludeItemIds",
	"filters":                 "filters",
	"genreids":                "genreIds",
	"genres":                  "genres",
	"id":                      "id",
	"ids":                     "ids",
	"includehidden":           "includeHidden",
	"indexnumber":             "indexNumber",
	"is4k":                    "is4K",
	"isfavorite":              "isFavorite",
	"ishd":                    "isHd",
	"isplayed":                "isPlayed",
	"limit":                   "limit",
	"maxpremieredate":         "maxPremiereDate",
	"mediatypes":              "mediaTypes",
	"mincommunityrating":      "minCommunityRating",
	"mincriticrating":         "minCriticRating",
	"minpremieredate":         "minPremiereDate",
	"name":                    "name",
	"namelessthan":            "nameLessThan",
	"namestartswith":          "nameStartsWith",
	"namestartswithorgreater": "nameStartsWithOrGreater",
	"officialratings":         "officialRatings",
	"parentid":                "parentId",
	"parentindexnumber":       "parentIndexNumber",
	"personids":               "personIds",
	"recursive":               "recursive",
	"searchterm":              "searchTerm",
	"seasonid":                "seasonId",
	"seriesid":                "seriesId",
	"sortby":                  "sortBy",
	"sortorder":               "sortOrder",
	"startindex":              "startIndex",
	"studioids":               "studioIds",
	"studios":                 "studios",
	"tag":                     "tag",
	"userid":                  "userId",
	"years":                   "years",
}

// These are the query parameters we remove
var removeParams = map[string]struct{}{
	// field parameter is ignored as we always return full API response object
	"fields": {},
}
