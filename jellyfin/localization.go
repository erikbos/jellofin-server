package jellyfin

import (
	"net/http"
)

// /Localization/Countries
func (j *Jellyfin) localizationCountriesHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFCountry{
		{
			DisplayName:              "United States",
			Name:                     "US",
			ThreeLetterISORegionName: "USA",
			TwoLetterISORegionName:   "US",
		},
	}
	serveJSON(response, w)
}

// /Localization/Cultures
func (j *Jellyfin) localizationCulturesHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFLanguage{
		{
			DisplayName:                 "English",
			Name:                        "English",
			ThreeLetterISOLanguageName:  "eng",
			ThreeLetterISOLanguageNames: []string{"eng"},
			TwoLetterISOLanguageName:    "en",
		},
	}
	serveJSON(response, w)
}

// Localization/Options
func (j *Jellyfin) localizationOptionsHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFLocalizationOptions{
		{
			Name:  "English",
			Value: "en-US",
		},
	}
	serveJSON(response, w)
}

// Localization/ParentalRatings
func (j *Jellyfin) localizationParentalRatingsHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFLocalizationParentalRatings{
		{
			Name:  "Unrated",
			Value: 0,
		},
	}
	serveJSON(response, w)
}
