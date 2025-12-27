package jellyfin

import "net/http"

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
	j.cache1h(w)
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
	j.cache1h(w)
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
	j.cache1h(w)
	serveJSON(response, w)
}

func (j *Jellyfin) cache1h(w http.ResponseWriter) {
	w.Header().Set("cache-control", "max-age=3600")
}
