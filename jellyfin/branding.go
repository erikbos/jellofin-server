package jellyfin

import (
	"net/http"
)

// /Branding/Configuration
func (j *Jellyfin) brandingConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	response := JFBrandingConfigurationResponse{
		SplashscreenEnabled: false,
	}
	serveJSON(response, w)
}

// /Branding/Css
// /Branding/Css.css
func (j *Jellyfin) brandingCssHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

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
	j.cache1h(w)
	serveJSON(response, w)
}
