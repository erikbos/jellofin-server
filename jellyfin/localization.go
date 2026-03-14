package jellyfin

import (
	"net/http"
)

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

// /Localization/ParentalRatings
//
// localizationParentalRatingsHandler serves the parental ratings for the media items.
func (j *Jellyfin) localizationParentalRatingsHandler(w http.ResponseWriter, r *http.Request) {
	ratings := GetAllParentalRatings()
	response := make([]ParentalRatingsItem, 0, len(ratings))
	for _, v := range ratings {
		entry := ParentalRatingsItem{
			Name: v.Name,
			RatingScore: &ParentalRatingsItemScore{
				Score:    v.Score,
				SubScore: v.SubScore,
			},
			Value: v.Score,
		}
		response = append(response, entry)
	}
	serveJSON(response, w)
}
