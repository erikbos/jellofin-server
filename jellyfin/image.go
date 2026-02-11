package jellyfin

import (
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/gorilla/mux"
)

// /Items/rVFG3EzPthk2wowNkqUl/Images/Backdrop?tag=7cec54f0c8f362c75588e83d76fefa75
// /Items/rVFG3EzPthk2wowNkqUl/Images/Logo?tag=e28fbe648d2dbb76b65c14f14e6b1d72
// /Items/q2e2UzCOd9zkmJenIOph/Images/Primary?tag=70931a7d8c147c9e2c0aafbad99e03e5
// /Items/rVFG3EzPthk2wowNkqUl/Images/Primary?tag=268b80952354f01d5a184ed64b36dd52
// /Items/2vx0ZYKeHxbh5iWhloIB/Images/Primary?tag=redirect_https://image.tmdb.org/t/p/original/3E4x5doNuuu6i9Mef6HPrlZjNb1.jpg
//
// itemsImagesHandler serves item images like posters, backdrops and logos
func (j *Jellyfin) itemsImagesHandler(w http.ResponseWriter, r *http.Request) {
	// handle tag-based redirects for item imagery that is external (e.g. external images of actors)
	// for these we do not care about the provided item id
	queryparams := r.URL.Query()
	tag := queryparams.Get("tag")
	if strings.HasPrefix(tag, tagprefix_redirect) {
		w.Header().Set("cache-control", "max-age=2592000")
		http.Redirect(w, r, strings.TrimPrefix(tag, tagprefix_redirect), http.StatusFound)
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]
	imageType := vars["type"]

	switch {
	case isJFCollectionID(itemID):
		fallthrough
	case isJFCollectionFavoritesID(itemID):
		fallthrough
	case isJFCollectionPlaylistID(itemID):
		log.Printf("Image request for collection %s!", itemID)
		apierror(w, "Image request for collection not yet supported", http.StatusNotFound)
		return
	case isJFPersonID(itemID):
		name, err := decodeJFPersonID(itemID)
		if err != nil {
			apierror(w, "Invalid person ID", http.StatusBadRequest)
			return
		}
		dbperson, err := j.repo.GetPersonByName(r.Context(), name, "")
		if err == nil && dbperson.PosterURL != "" {
			http.Redirect(w, r, dbperson.PosterURL, http.StatusFound)
			return
		}
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}

	c, i := j.collections.GetItemByID(trimPrefix(itemID))
	if i == nil {
		apierror(w, "Item not found", http.StatusNotFound)
		return
	}

	switch strings.ToLower(imageType) {
	case "primary":
		if i.Poster() != "" {
			j.serveImage(w, r, c.Directory+"/"+i.Path()+"/"+i.Poster(), j.imageQualityPoster)
			return
		}
		// todo implement fallback options:
		// 1. Serve item season all poster
		// 2. Serve show poster as fallback
		apierror(w, "Poster not found", http.StatusNotFound)
		return
	case "backdrop":
		if i.Fanart() != "" {
			j.serveFile(w, r, c.Directory+"/"+i.Path()+"/"+i.Fanart())
			return
		}
		apierror(w, "Backdrop not found", http.StatusNotFound)
		return
	case "logo":
		if i.Logo() != "" {
			j.serveImage(w, r, c.Directory+"/"+i.Path()+"/"+i.Logo(), j.imageQualityPoster)
			return
		}
		apierror(w, "Logo not found", http.StatusNotFound)
		return
	}
	log.Printf("Unknown image type requested: %s\n", vars["type"])
	apierror(w, "Item image not found", http.StatusNotFound)
}

func (j *Jellyfin) serveImage(w http.ResponseWriter, r *http.Request, filename string, imageQuality int) {
	file, err := j.imageresizer.OpenFile(w, r, filename, imageQuality)
	if err != nil {
		apierror(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	fileStat, err := file.Stat()
	if err != nil {
		apierror(w, "Could not retrieve file info", http.StatusInternalServerError)
		return
	}
	w.Header().Set("cache-control", "max-age=2592000")
	w.Header().Set("content-type", mimeTypeByExtension(filename))
	http.ServeContent(w, r, fileStat.Name(), fileStat.ModTime(), file)
}

// mimeTypeByExtension returns the mime type based on the file extension
func mimeTypeByExtension(filename string) string {
	switch strings.ToLower(path.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"

	case ".mp4":
		return "video/mp4"
	case ".m4v":
		return "video/x-m4v"
	case ".mov":
		return "video/quicktime"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"

	case ".mp3":
		return "audio/mpeg"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".wav":
		return "audio/wav"

	default:
		return "application/octet-stream"
	}
}
