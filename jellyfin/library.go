package jellyfin

import "net/http"

// /Library/VirtualFolders
//
// libraryVirtualFoldersHandler returns the available collections as virtual folders
func (j *Jellyfin) libraryVirtualFoldersHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFMediaLibrary{}
	for _, c := range j.collections.GetCollections() {
		collectionItem, err := j.makeJFItemCollection(c.ID)
		if err != nil {
			apierror(w, err.Error(), http.StatusInternalServerError)
			return
		}
		l := JFMediaLibrary{
			Name:               collectionItem.Name,
			ItemId:             collectionItem.ID,
			PrimaryImageItemId: collectionItem.ID,
			CollectionType:     collectionItem.Type,
			Locations:          []string{"/"},
		}
		response = append(response, l)
	}
	serveJSON(response, w)
}
