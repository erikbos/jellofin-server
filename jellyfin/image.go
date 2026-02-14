package jellyfin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/database/model"
)

const (
	// maximum allowed size for uploaded images (40MB)
	maxUploadSize = 40 * 1024 * 1024
)

// /Items/rVFG3EzPthk2wowNkqUl/Images/Backdrop?tag=7cec54f0c8f362c75588e83d76fefa75
// /Items/rVFG3EzPthk2wowNkqUl/Images/Logo?tag=e28fbe648d2dbb76b65c14f14e6b1d72
// /Items/q2e2UzCOd9zkmJenIOph/Images/Primary?tag=70931a7d8c147c9e2c0aafbad99e03e5
// /Items/rVFG3EzPthk2wowNkqUl/Images/Primary?tag=268b80952354f01d5a184ed64b36dd52
// /Items/2vx0ZYKeHxbh5iWhloIB/Images/Primary?tag=redirect_https://image.tmdb.org/t/p/original/3E4x5doNuuu6i9Mef6HPrlZjNb1.jpg
//
// itemsImagesGetHandler serves item images like posters, backdrops and logos
func (j *Jellyfin) itemsImagesGetHandler(w http.ResponseWriter, r *http.Request) {
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
		j.serveItemImage(w, r, itemID, imageType)
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
			j.serveImageFile(w, r, c.Directory+"/"+i.Path()+"/"+i.Poster(), j.imageQualityPoster)
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
			j.serveImageFile(w, r, c.Directory+"/"+i.Path()+"/"+i.Logo(), j.imageQualityPoster)
			return
		}
		apierror(w, "Logo not found", http.StatusNotFound)
		return
	}
	log.Printf("Unknown image type requested: %s\n", vars["type"])
	apierror(w, "Item image not found", http.StatusNotFound)
}

// /Items/{item}/Images/{type}/{index}?tag=7cec54f0c8f362c75588e83d76fefa75
//
// itemsImagesPostHandler stores item images like posters, backdrops and logos
func (j *Jellyfin) itemsImagesPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["item"]
	imageType := vars["type"]

	// Validate item ID is provided
	if itemID == "" {
		apierror(w, "itemId parameter is required", http.StatusBadRequest)
		return
	}
	// We do not check item type, so collections, shows, seasons and episodes can all have images uploaded.
	if strings.ToLower(imageType) != "primary" {
		apierror(w, "Only primary images can be uploaded", http.StatusBadRequest)
		return
	}
	j.receiveItemImage(w, r, itemID, imageType)
}

const (
	ImageTypeProfile = "Profile"
	ImageTypePrimary = "Primary"
)

// GET /Users/{user}/Images/{type}
//
// usersImagesProfileHandler handles requests for user profile images
func (j *Jellyfin) usersImagesProfileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["userid"]
	if userID == "" {
		apierror(w, "userId parameter is required", http.StatusBadRequest)
		return
	}
	// We ignore the provided image type and always use "Profile"
	j.serveItemImage(w, r, userID, ImageTypeProfile)
}

// GET /UserImage?userId=fa2b28f1af954a71a58353b7e2da9de6&
//
// userImageGetHandler retrieves a user's profile image
func (j *Jellyfin) userImageGetHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	userID := queryParams.Get("userId")
	if userID == "" {
		apierror(w, "userId parameter is required", http.StatusBadRequest)
		return
	}
	j.serveItemImage(w, r, userID, ImageTypeProfile)
}

// POST /UserImage?userId=fa2b28f1af954a71a58353b7e2da9de6&
//
// userImagePostHandler uploads a user's profile image
func (j *Jellyfin) userImagePostHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	queryParams := r.URL.Query()
	userID := queryParams.Get("userId")
	if userID == "" {
		apierror(w, "userId parameter is required", http.StatusBadRequest)
		return
	}
	if userID != accessToken.UserID {
		apierror(w, "Cannot upload image for another user", http.StatusForbidden)
		return
	}
	j.receiveItemImage(w, r, userID, ImageTypeProfile)
}

// DELETE /UserImage?userId=fa2b28f1af954a71a58353b7e2da9de6&
//
// userImageDeleteHandler deletes a user's profile image
func (j *Jellyfin) userImageDeleteHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	queryParams := r.URL.Query()
	userID := queryParams.Get("userId")
	if userID == "" {
		apierror(w, "userId parameter is required", http.StatusBadRequest)
		return
	}
	if userID != accessToken.UserID {
		apierror(w, "Cannot delete image for another user", http.StatusForbidden)
		return
	}
	if err := j.repo.DeleteImage(r.Context(), userID, ImageTypeProfile); err != nil {
		apierror(w, "Failed to delete image", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// /Items/{item}/Images
//
// itemsImagesHandler returns a list of images for an item
func (j *Jellyfin) itemsImagesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["item"]
	_, i := j.collections.GetItemByID(trimPrefix(itemID))
	if i == nil {
		apierror(w, "Item not found", http.StatusNotFound)
		return
	}
	var images []JFResponseItemImages
	index := 0
	if i.Poster() != "" {
		images = append(images, JFResponseItemImages{ImageIndex: index, ImageType: "Primary", ImageTag: i.ID()})
		index++
	}
	if i.Banner() != "" {
		images = append(images, JFResponseItemImages{ImageIndex: index, ImageType: "Backdrop", ImageTag: i.ID()})
		index++
	}
	if i.Logo() != "" {
		images = append(images, JFResponseItemImages{ImageIndex: index, ImageType: "Logo", ImageTag: i.ID()})
		index++
	}
	serveJSON(images, w)
}

// /Items/{item}/RemoteImages
//
// itemsRemoteImagesHandler returns a list of remote images for an item
func (j *Jellyfin) itemsRemoteImagesHandler(w http.ResponseWriter, r *http.Request) {
	response := JFResponseItemRemoteImages{
		Images:           []JFResponseItemRemoteImagesImage{},
		TotalRecordCount: 0,
	}
	serveJSON(response, w)
}

// /Items/episode_c2y4g6NdjoX23XPWnjJv/RemoteImages/Providers
//
// itemsRemoteImagesProvidersHandler returns a list of remote image providers for an item
func (j *Jellyfin) itemsRemoteImagesProvidersHandler(w http.ResponseWriter, r *http.Request) {
	response := JFResponseItemRemoteImagesProviders{
		{
			Name:            "Local Repository",
			SupportedImages: []string{"Primary"},
		},
	}
	serveJSON(response, w)
}

// receiveItemImage reads image data from the request and stores it in the repository
func (j *Jellyfin) receiveItemImage(w http.ResponseWriter, r *http.Request, userID, imageType string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	imageData, err := io.ReadAll(r.Body)
	if err != nil {
		apierror(w, "Failed to read image data", http.StatusBadRequest)
		return
	}
	mimeType := http.DetectContentType(imageData)
	// If we cannot detect an image mime type, we'll try to decode as Base64 and check again
	// Some clients like Swiftfin send Base64-encoded data without setting Content-Type..
	if !strings.HasPrefix(mimeType, "image/") {
		imageData, err := base64.StdEncoding.DecodeString(string(imageData))
		mimeType = http.DetectContentType(imageData)
		// Validate it's now a valid image
		if err != nil || !strings.HasPrefix(mimeType, "image/") {
			apierror(w, "Uploaded file is not a valid image", http.StatusBadRequest)
			return
		}
	}
	metadata := model.ImageMetadata{
		MimeType: mimeType,
		FileSize: len(imageData),
		Etag:     makeImageEtag(imageData),
		Updated:  time.Now().UTC(),
	}
	if err = j.repo.StoreImage(r.Context(), userID, imageType, metadata, imageData); err != nil {
		apierror(w, "Failed to store image", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// makeJFImageTags checks if an item has an image and returns the appropriate JFImageTags
func (j *Jellyfin) makeJFImageTags(ctx context.Context, itemID, imageType string) *JFImageTags {
	if _, err := j.repo.HasImage(ctx, itemID, imageType); err != nil {
		return nil
	}
	return &JFImageTags{Primary: itemID}
}

// serveItemImage serves an item image from the repository
func (j *Jellyfin) serveItemImage(w http.ResponseWriter, r *http.Request, itemID, imageType string) {
	metadata, imageData, err := j.repo.GetImage(r.Context(), itemID, imageType)
	if err == model.ErrNotFound {
		apierror(w, "Image not found", http.StatusNotFound)
		return
	}
	if err != nil {
		apierror(w, "Failed to retrieve image", http.StatusInternalServerError)
		return
	}
	w.Header().Set("etag", metadata.Etag)
	w.Header().Set("content-type", metadata.MimeType)
	w.Header().Set("content-length", fmt.Sprintf("%d", metadata.FileSize))
	w.Header().Set("last-modified", metadata.Updated.Format(http.TimeFormat))
	http.ServeContent(w, r, "", metadata.Updated, bytes.NewReader(imageData))
}

// serveImageFile serves an image file from the filesystem
func (j *Jellyfin) serveImageFile(w http.ResponseWriter, r *http.Request, filename string, imageQuality int) {
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
	w.Header().Set("etag", fileStat.ModTime().Format("20060102150405"))
	w.Header().Set("content-type", mimeTypeByExtension(filename))
	w.Header().Set("content-length", fmt.Sprintf("%d", fileStat.Size()))
	w.Header().Set("last-modified", fileStat.ModTime().Format(http.TimeFormat))
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

func makeImageEtag(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))[:16]
}
