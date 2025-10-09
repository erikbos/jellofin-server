package notflix

type Collection struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Items []Item `json:"items,omitempty"`
}

type Collections []Collection

// An 'item' can be a movie, a tv-show, a folder, etc.
type Item struct {
	// generic
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	BaseUrl    string   `json:"baseurl"`
	Type       string   `json:"type"`
	FirstVideo int64    `json:"firstvideo,omitempty"`
	LastVideo  int64    `json:"lastvideo,omitempty"`
	SortName   string   `json:"sortName,omitempty"`
	Nfo        ItemNfo  `json:"nfo"`
	Banner     string   `json:"banner,omitempty"`
	Fanart     string   `json:"fanart,omitempty"`
	Folder     string   `json:"folder,omitempty"`
	Poster     string   `json:"poster,omitempty"`
	Rating     float32  `json:"rating,omitempty"`
	Votes      int      `json:"votes,omitempty"`
	Genre      []string `json:"genre,omitempty"`
	Year       int      `json:"year,omitempty"`

	// movie
	Video   string `json:"video,omitempty"`
	Thumb   string `json:"thumb,omitempty"`
	SrtSubs []Subs `json:"srtsubs,omitempty"`
	VttSubs []Subs `json:"vttsubs,omitempty"`

	// show
	SeasonAllBanner string   `json:"seasonAllBanner,omitempty"`
	SeasonAllPoster string   `json:"seasonAllPoster,omitempty"`
	Seasons         []Season `json:"seasons,omitempty"`
}

type ItemNfo struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Plot      string   `json:"plot"`
	Genre     []string `json:"genre,omitempty"`
	Premiered string   `json:"premiered"`
	MPAA      string   `json:"mpaa"`
	Aired     string   `json:"aired"`
	Studio    string   `json:"studio"`
	Rating    float32  `json:"rating"`
}

type Season struct {
	SeasonNo int       `json:"seasonno"`
	Banner   string    `json:"banner,omitempty"`
	Fanart   string    `json:"fanart,omitempty"`
	Poster   string    `json:"poster,omitempty"`
	Episodes []Episode `json:"episodes,omitempty"`
}

type Episode struct {
	Name      string     `json:"name"`
	SeasonNo  int        `json:"seasonno"`
	EpisodeNo int        `json:"episodeno"`
	Double    bool       `json:"double,omitempty"`
	SortName  string     `json:"sortName,omitempty"`
	Nfo       EpisodeNfo `json:"nfo"`
	Video     string     `json:"video"`
	Thumb     string     `json:"thumb,omitempty"`
	SrtSubs   []Subs     `json:"srtsubs,omitempty"`
	VttSubs   []Subs     `json:"vttsubs,omitempty"`
}

type EpisodeNfo struct {
	Title   string `json:"title"`
	Plot    string `json:"plot"`
	Season  string `json:"season"`
	Episode string `json:"episode"`
	Aired   string `json:"aired"`
}

type Subs struct {
	Lang string `json:"lang"`
	Path string `json:"path"`
}
