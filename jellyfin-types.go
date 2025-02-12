package main

import (
	"time"
)

// API definitions: https://swagger.emby.media/ & https://api.jellyfin.org/
// Docs: https://github.com/mediabrowser/emby/wiki

type JFSystemInfoResponse struct {
	LocalAddress           string `json:"LocalAddress"`
	ServerName             string `json:"ServerName"`
	Version                string `json:"Version"`
	ProductName            string `json:"ProductName"`
	OperatingSystem        string `json:"OperatingSystem"`
	Id                     string `json:"Id"`
	StartupWizardCompleted bool   `json:"StartupWizardCompleted"`
}

type JFUser struct {
	Name                      string              `json:"Name"`
	ServerId                  string              `json:"ServerId"`
	Id                        string              `json:"Id"`
	HasPassword               bool                `json:"HasPassword"`
	HasConfiguredPassword     bool                `json:"HasConfiguredPassword"`
	HasConfiguredEasyPassword bool                `json:"HasConfiguredEasyPassword"`
	EnableAutoLogin           bool                `json:"EnableAutoLogin"`
	LastLoginDate             time.Time           `json:"LastLoginDate"`
	LastActivityDate          time.Time           `json:"LastActivityDate"`
	Configuration             JFUserConfiguration `json:"Configuration"`
	Policy                    JFUserPolicy        `json:"Policy"`
}

type JFUserConfiguration struct {
	PlayDefaultAudioTrack      bool     `json:"PlayDefaultAudioTrack"`
	SubtitleLanguagePreference string   `json:"SubtitleLanguagePreference"`
	DisplayMissingEpisodes     bool     `json:"DisplayMissingEpisodes"`
	GroupedFolders             []string `json:"GroupedFolders"`
	// SubtitleMode               string   `json:"SubtitleMode"`
	DisplayCollectionsView     bool     `json:"DisplayCollectionsView"`
	EnableLocalPassword        bool     `json:"EnableLocalPassword"`
	OrderedViews               []string `json:"OrderedViews"`
	LatestItemsExcludes        []string `json:"LatestItemsExcludes"`
	MyMediaExcludes            []string `json:"MyMediaExcludes"`
	HidePlayedInLatest         bool     `json:"HidePlayedInLatest"`
	RememberAudioSelections    bool     `json:"RememberAudioSelections"`
	RememberSubtitleSelections bool     `json:"RememberSubtitleSelections"`
	EnableNextEpisodeAutoPlay  bool     `json:"EnableNextEpisodeAutoPlay"`
	CastReceiverId             string   `json:"CastReceiverId"`
}

type JFUserPolicy struct {
	IsAdministrator                  bool     `json:"IsAdministrator"`
	IsHidden                         bool     `json:"IsHidden"`
	EnableCollectionManagement       bool     `json:"EnableCollectionManagement"`
	EnableSubtitleManagement         bool     `json:"EnableSubtitleManagement"`
	EnableLyricManagement            bool     `json:"EnableLyricManagement"`
	IsDisabled                       bool     `json:"IsDisabled"`
	BlockedTags                      []string `json:"BlockedTags"`
	AllowedTags                      []string `json:"AllowedTags"`
	EnableUserPreferenceAccess       bool     `json:"EnableUserPreferenceAccess"`
	AccessSchedules                  []string `json:"AccessSchedules"`
	BlockUnratedItems                []string `json:"BlockUnratedItems"`
	EnableRemoteControlOfOtherUsers  bool     `json:"EnableRemoteControlOfOtherUsers"`
	EnableSharedDeviceControl        bool     `json:"EnableSharedDeviceControl"`
	EnableRemoteAccess               bool     `json:"EnableRemoteAccess"`
	EnableLiveTvManagement           bool     `json:"EnableLiveTvManagement"`
	EnableLiveTvAccess               bool     `json:"EnableLiveTvAccess"`
	EnableMediaPlayback              bool     `json:"EnableMediaPlayback"`
	EnableAudioPlaybackTranscoding   bool     `json:"EnableAudioPlaybackTranscoding"`
	EnableVideoPlaybackTranscoding   bool     `json:"EnableVideoPlaybackTranscoding"`
	EnablePlaybackRemuxing           bool     `json:"EnablePlaybackRemuxing"`
	ForceRemoteSourceTranscoding     bool     `json:"ForceRemoteSourceTranscoding"`
	EnableContentDeletion            bool     `json:"EnableContentDeletion"`
	EnableContentDeletionFromFolders []string `json:"EnableContentDeletionFromFolders"`
	EnableContentDownloading         bool     `json:"EnableContentDownloading"`
	EnableSyncTranscoding            bool     `json:"EnableSyncTranscoding"`
	EnableMediaConversion            bool     `json:"EnableMediaConversion"`
	EnabledDevices                   []string `json:"EnabledDevices"`
	EnableAllDevices                 bool     `json:"EnableAllDevices"`
	EnabledChannels                  []string `json:"EnabledChannels"`
	EnableAllChannels                bool     `json:"EnableAllChannels"`
	EnabledFolders                   []string `json:"EnabledFolders"`
	EnableAllFolders                 bool     `json:"EnableAllFolders"`
	InvalidLoginAttemptCount         int      `json:"InvalidLoginAttemptCount"`
	LoginAttemptsBeforeLockout       int      `json:"LoginAttemptsBeforeLockout"`
	MaxActiveSessions                int      `json:"MaxActiveSessions"`
	EnablePublicSharing              bool     `json:"EnablePublicSharing"`
	BlockedMediaFolders              []string `json:"BlockedMediaFolders"`
	BlockedChannels                  []string `json:"BlockedChannels"`
	RemoteClientBitrateLimit         int      `json:"RemoteClientBitrateLimit"`
	AuthenticationProviderID         string   `json:"AuthenticationProviderId"`
	PasswordResetProviderID          string   `json:"PasswordResetProviderId"`
	SyncPlayAccess                   string   `json:"SyncPlayAccess"`
}

type JFAuthenticateByNameResponse struct {
	User        JFUser        `json:"User"`
	SessionInfo JFSessionInfo `json:"SessionInfo"`
	AccessToken string        `json:"AccessToken"`
	ServerId    string        `json:"ServerId"`
}

type JFUsersItemsResumeResponse struct {
	Items            []string `json:"Items"`
	TotalRecordCount int      `json:"TotalRecordCount"`
	StartIndex       int      `json:"StartIndex"`
}

type JFSessionInfo struct {
	PlayState struct {
		CanSeek       bool   `json:"CanSeek"`
		IsPaused      bool   `json:"IsPaused"`
		IsMuted       bool   `json:"IsMuted"`
		RepeatMode    string `json:"RepeatMode"`
		PlaybackOrder string `json:"PlaybackOrder"`
	} `json:"PlayState"`
	RemoteEndPoint     string    `json:"RemoteEndPoint"`
	Id                 string    `json:"Id"`
	UserId             string    `json:"UserId"`
	UserName           string    `json:"UserName"`
	Client             string    `json:"Client"`
	LastActivityDate   time.Time `json:"LastActivityDate"`
	DeviceName         string    `json:"DeviceName"`
	DeviceId           string    `json:"DeviceId"`
	ApplicationVersion string    `json:"ApplicationVersion"`
	IsActive           bool      `json:"IsActive"`
}

type DisplayPreferencesCustomPrefs struct {
	ChromecastVersion          string `json:"chromecastVersion"`
	SkipForwardLength          string `json:"skipForwardLength"`
	SkipBackLength             string `json:"skipBackLength"`
	EnableNextVideoInfoOverlay string `json:"enableNextVideoInfoOverlay"`
	Tvhome                     string `json:"tvhome"`
	DashboardTheme             string `json:"dashboardTheme"`
}

type DisplayPreferencesResponse struct {
	ID                 string                        `json:"Id"`
	SortBy             string                        `json:"SortBy"`
	RememberIndexing   bool                          `json:"RememberIndexing"`
	PrimaryImageHeight int                           `json:"PrimaryImageHeight"`
	PrimaryImageWidth  int                           `json:"PrimaryImageWidth"`
	CustomPrefs        DisplayPreferencesCustomPrefs `json:"CustomPrefs"`
	ScrollDirection    string                        `json:"ScrollDirection"`
	ShowBackdrop       bool                          `json:"ShowBackdrop"`
	RememberSorting    bool                          `json:"RememberSorting"`
	SortOrder          string                        `json:"SortOrder"`
	ShowSidebar        bool                          `json:"ShowSidebar"`
	Client             string                        `json:"Client"`
}

type JFCollection struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

type JFUserViewsResponse struct {
	Items            []JFItem `json:"Items"`
	TotalRecordCount int      `json:"TotalRecordCount"`
	StartIndex       int      `json:"StartIndex"`
}

type ProviderIds struct {
}

type UserData struct {
	PlaybackPositionTicks int    `json:"PlaybackPositionTicks"`
	PlayCount             int    `json:"PlayCount"`
	IsFavorite            bool   `json:"IsFavorite"`
	Played                bool   `json:"Played"`
	Key                   string `json:"Key"`
}

type JFItem struct {
	Name                     string             `json:"Name"`
	OriginalTitle            string             `json:"OriginalTitle,omitempty"`
	ServerID                 string             `json:"ServerId"`
	ID                       string             `json:"Id"`
	Etag                     string             `json:"Etag"`
	DateCreated              time.Time          `json:"DateCreated,omitempty"`
	CanDelete                bool               `json:"CanDelete,omitempty"`
	CanDownload              bool               `json:"CanDownload,omitempty"`
	Container                string             `json:"Container,omitempty"`
	SortName                 string             `json:"SortName,omitempty"`
	ForcedSortName           string             `json:"ForcedSortName,omitempty"`
	PremiereDate             time.Time          `json:"PremiereDate,omitempty"`
	ExternalUrls             []JFExternalUrls   `json:"ExternalUrls,omitempty"`
	MediaSources             []JFMediaSources   `json:"MediaSources,omitempty"`
	CriticRating             int                `json:"CriticRating,omitempty"`
	ProductionLocations      []string           `json:"ProductionLocations,omitempty"`
	Path                     string             `json:"Path,omitempty"`
	EnableMediaSourceDisplay bool               `json:"EnableMediaSourceDisplay"`
	OfficialRating           string             `json:"OfficialRating,omitempty"`
	ChannelID                []string           `json:"ChannelId,omitempty"`
	ChildCount               int                `json:"ChildCount,omitempty"`
	CollectionType           string             `json:"CollectionType,omitempty"`
	Overview                 string             `json:"Overview,omitempty"`
	Taglines                 []string           `json:"Taglines,omitempty"`
	Genres                   []string           `json:"Genres,omitempty"`
	CommunityRating          float64            `json:"CommunityRating,omitempty"`
	RunTimeTicks             int64              `json:"RunTimeTicks,omitempty"`
	PlayAccess               string             `json:"PlayAccess,omitempty"`
	ProductionYear           int                `json:"ProductionYear,omitempty"`
	RemoteTrailers           []JFRemoteTrailers `json:"RemoteTrailers,omitempty"`
	// ProviderIds              JFProviderIds      `json:"ProviderIds,omitempty"`
	IsFolder                bool               `json:"IsFolder"`
	ParentID                string             `json:"ParentId,omitempty"`
	Type                    string             `json:"Type,omitempty"`
	People                  []JFPeople         `json:"People,omitempty"`
	Studios                 []JFStudios        `json:"Studios,omitempty"`
	GenreItems              []JFGenreItems     `json:"GenreItems,omitempty"`
	LocalTrailerCount       int                `json:"LocalTrailerCount,omitempty"`
	UserData                JFUserData         `json:"UserData,omitempty"`
	SpecialFeatureCount     int                `json:"SpecialFeatureCount,omitempty"`
	DisplayPreferencesID    string             `json:"DisplayPreferencesId,omitempty"`
	Tags                    []string           `json:"Tags,omitempty"`
	PrimaryImageAspectRatio float64            `json:"PrimaryImageAspectRatio,omitempty"`
	MediaStreams            []JFMediaStreams   `json:"MediaStreams,omitempty"`
	VideoType               string             `json:"VideoType,omitempty"`
	ImageTags               *JFImageTags       `json:"ImageTags,omitempty"`
	BackdropImageTags       []string           `json:"BackdropImageTags,omitempty"`
	ImageBlurHashes         *JFImageBlurHashes `json:"ImageBlurHashes,omitempty"`
	Chapters                []string           `json:"Chapters,omitempty"`
	LocationType            string             `json:"LocationType,omitempty"`
	MediaType               string             `json:"MediaType,omitempty"`
	LockedFields            []string           `json:"LockedFields,omitempty"`
	LockData                bool               `json:"LockData,omitempty"`
	Width                   int                `json:"Width,omitempty"`
	Height                  int                `json:"Height,omitempty"`
	SeriesID                string             `json:"SeriesId,omitempty"`
	SeriesName              string             `json:"SeriesName,omitempty"`
	SeasonID                string             `json:"SeasonId,omitempty"`
	SeasonName              string             `json:"SeasonName,omitempty"`
	IndexNumber             int                `json:"IndexNumber,omitempty"`
	ParentIndexNumber       int                `json:"ParentIndexNumber,omitempty"`
	RecursiveItemCount      int                `json:"RecursiveItemCount,omitempty"`
	HasSubtitles            bool               `json:"HasSubtitles,omitempty"`
}

type JFExternalUrls struct {
	Name string `json:"Name"`
	URL  string `json:"Url"`
}
type JFMediaStreams struct {
	Title                  string  `json:"Title"`
	Codec                  string  `json:"Codec"`
	CodecTag               string  `json:"CodecTag,omitempty"`
	Language               string  `json:"Language,omitempty"`
	TimeBase               string  `json:"TimeBase"`
	VideoRange             string  `json:"VideoRange"`
	VideoRangeType         string  `json:"VideoRangeType"`
	AudioSpatialFormat     string  `json:"AudioSpatialFormat"`
	DisplayTitle           string  `json:"DisplayTitle,omitempty"`
	NalLengthSize          string  `json:"NalLengthSize,omitempty"`
	IsInterlaced           bool    `json:"IsInterlaced"`
	IsAVC                  bool    `json:"IsAVC"`
	BitRate                int     `json:"BitRate,omitempty"`
	BitDepth               int     `json:"BitDepth,omitempty"`
	RefFrames              int     `json:"RefFrames,omitempty"`
	IsDefault              bool    `json:"IsDefault"`
	IsForced               bool    `json:"IsForced"`
	IsHearingImpaired      bool    `json:"IsHearingImpaired"`
	Height                 int     `json:"Height,omitempty"`
	Width                  int     `json:"Width,omitempty"`
	AverageFrameRate       float64 `json:"AverageFrameRate,omitempty"`
	RealFrameRate          float64 `json:"RealFrameRate,omitempty"`
	Profile                string  `json:"Profile,omitempty"`
	Type                   string  `json:"Type"`
	AspectRatio            string  `json:"AspectRatio,omitempty"`
	Index                  int     `json:"Index"`
	IsExternal             bool    `json:"IsExternal"`
	IsTextSubtitleStream   bool    `json:"IsTextSubtitleStream"`
	SupportsExternalStream bool    `json:"SupportsExternalStream"`
	PixelFormat            string  `json:"PixelFormat,omitempty"`
	Level                  int     `json:"Level"`
	IsAnamorphic           bool    `json:"IsAnamorphic,omitempty"`
	LocalizedDefault       string  `json:"LocalizedDefault,omitempty"`
	LocalizedExternal      string  `json:"LocalizedExternal,omitempty"`
	ChannelLayout          string  `json:"ChannelLayout,omitempty"`
	Channels               int     `json:"Channels,omitempty"`
	SampleRate             int     `json:"SampleRate,omitempty"`
	ColorSpace             string  `json:"ColorSpace,omitempty"`
}

type JFMediaAttachments struct {
	Codec    string `json:"Codec"`
	CodecTag string `json:"CodecTag"`
	Index    int    `json:"Index"`
}

type JFRequiredHTTPHeaders struct {
}

type JFMediaSources struct {
	Protocol                string                `json:"Protocol"`
	ID                      string                `json:"Id"`
	Path                    string                `json:"Path"`
	Type                    string                `json:"Type"`
	Container               string                `json:"Container"`
	Size                    int64                 `json:"Size"`
	Name                    string                `json:"Name"`
	IsRemote                bool                  `json:"IsRemote"`
	ETag                    string                `json:"ETag"`
	RunTimeTicks            int64                 `json:"RunTimeTicks"`
	ReadAtNativeFramerate   bool                  `json:"ReadAtNativeFramerate"`
	IgnoreDts               bool                  `json:"IgnoreDts"`
	IgnoreIndex             bool                  `json:"IgnoreIndex"`
	GenPtsInput             bool                  `json:"GenPtsInput"`
	SupportsTranscoding     bool                  `json:"SupportsTranscoding"`
	SupportsDirectStream    bool                  `json:"SupportsDirectStream"`
	SupportsDirectPlay      bool                  `json:"SupportsDirectPlay"`
	IsInfiniteStream        bool                  `json:"IsInfiniteStream"`
	RequiresOpening         bool                  `json:"RequiresOpening"`
	RequiresClosing         bool                  `json:"RequiresClosing"`
	RequiresLooping         bool                  `json:"RequiresLooping"`
	SupportsProbing         bool                  `json:"SupportsProbing"`
	VideoType               string                `json:"VideoType"`
	MediaStreams            []JFMediaStreams      `json:"MediaStreams"`
	MediaAttachments        []JFMediaAttachments  `json:"MediaAttachments"`
	Formats                 []string              `json:"Formats"`
	Bitrate                 int                   `json:"Bitrate"`
	RequiredHTTPHeaders     JFRequiredHTTPHeaders `json:"RequiredHttpHeaders"`
	TranscodingSubProtocol  string                `json:"TranscodingSubProtocol"`
	DefaultAudioStreamIndex int                   `json:"DefaultAudioStreamIndex"`
}
type JFRemoteTrailers struct {
	URL  string `json:"Url"`
	Name string `json:"Name,omitempty"`
}

type JFProviderIds struct {
	Tmdb           string `json:"Tmdb"`
	Imdb           string `json:"Imdb"`
	TmdbCollection string `json:"TmdbCollection"`
}

// ImageBlurHashes Gets or sets the primary image blurhash.
type JFImageBlurHashes struct {
	Art        map[string]string `json:"Art,omitempty"`
	Backdrop   map[string]string `json:"Backdrop,omitempty"`
	Banner     map[string]string `json:"Banner,omitempty"`
	Box        map[string]string `json:"Box,omitempty"`
	BoxRear    map[string]string `json:"BoxRear,omitempty"`
	Chapter    map[string]string `json:"Chapter,omitempty"`
	Disc       map[string]string `json:"Disc,omitempty"`
	Logo       map[string]string `json:"Logo,omitempty"`
	Menu       map[string]string `json:"Menu,omitempty"`
	Primary    map[string]string `json:"Primary,omitempty"`
	Profile    map[string]string `json:"Profile,omitempty"`
	Screenshot map[string]string `json:"Screenshot,omitempty"`
	Thumb      map[string]string `json:"Thumb,omitempty"`
}

type JFPeople struct {
	Name            string             `json:"Name"`
	ID              string             `json:"Id"`
	Role            string             `json:"Role,omitempty"`
	Type            string             `json:"Type"`
	PrimaryImageTag string             `json:"PrimaryImageTag,omitempty"`
	ImageBlurHashes *JFImageBlurHashes `json:"ImageBlurHashes,omitempty"`
}

type JFStudios struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

type JFGenreItems struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}
type JFUserData struct {
	PlaybackPositionTicks int       `json:"PlaybackPositionTicks"`
	PlayCount             int       `json:"PlayCount"`
	IsFavorite            bool      `json:"IsFavorite"`
	LastPlayedDate        time.Time `json:"LastPlayedDate,omitempty"`
	Played                bool      `json:"Played"`
	Key                   string    `json:"Key"`
}

type JFImageTags struct {
	Primary  string `json:"Primary,omitempty"`
	Backdrop string `json:"Backdrop,omitempty"`
	Logo     string `json:"Logo,omitempty"`
	Thumb    string `json:"Thumb,omitempty"`
}

type UserItemsResponse struct {
	Items            []JFItem `json:"Items"`
	StartIndex       int      `json:"StartIndex"`
	TotalRecordCount int      `json:"TotalRecordCount"`
}

type JFShowsNextUpResponse struct {
	Items            []JFItem `json:"Items"`
	TotalRecordCount int      `json:"TotalRecordCount"`
	StartIndex       int      `json:"StartIndex"`
}

type JFUsersPlaybackInfoResponse struct {
	MediaSources  []JFMediaSources `json:"MediaSources"`
	PlaySessionID string           `json:"PlaySessionId"`
}

type JFPathInfo struct {
	Path string `json:"Path,omitempty"`
}

type JFTypeOption struct {
	Type                 string   `json:"Type,omitempty"`
	MetadataFetchers     []string `json:"MetadataFetchers,omitempty"`
	MetadataFetcherOrder []string `json:"MetadataFetcherOrder,omitempty"`
	ImageFetchers        []string `json:"ImageFetchers,omitempty"`
	ImageFetcherOrder    []string `json:"ImageFetcherOrder,omitempty"`
	ImageOptions         []string `json:"ImageOptions,omitempty"`
}

type JFLibraryOptions struct {
	Enabled                                 bool           `json:"Enabled"`
	EnablePhotos                            bool           `json:"EnablePhotos,omitempty"`
	EnableRealtimeMonitor                   bool           `json:"EnableRealtimeMonitor,omitempty"`
	EnableLUFSScan                          bool           `json:"EnableLUFSScan,omitempty"`
	EnableChapterImageExtraction            bool           `json:"EnableChapterImageExtraction,omitempty"`
	ExtractChapterImagesDuringLibraryScan   bool           `json:"ExtractChapterImagesDuringLibraryScan,omitempty"`
	EnableTrickplayImageExtraction          bool           `json:"EnableTrickplayImageExtraction,omitempty"`
	ExtractTrickplayImagesDuringLibraryScan bool           `json:"ExtractTrickplayImagesDuringLibraryScan,omitempty"`
	PathInfos                               []JFPathInfo   `json:"PathInfos,omitempty"`
	SaveLocalMetadata                       bool           `json:"SaveLocalMetadata,omitempty"`
	EnableInternetProviders                 bool           `json:"EnableInternetProviders,omitempty"`
	EnableAutomaticSeriesGrouping           bool           `json:"EnableAutomaticSeriesGrouping,omitempty"`
	EnableEmbeddedTitles                    bool           `json:"EnableEmbeddedTitles,omitempty"`
	EnableEmbeddedExtrasTitles              bool           `json:"EnableEmbeddedExtrasTitles,omitempty"`
	EnableEmbeddedEpisodeInfos              bool           `json:"EnableEmbeddedEpisodeInfos,omitempty"`
	AutomaticRefreshIntervalDays            int            `json:"AutomaticRefreshIntervalDays,omitempty"`
	PreferredMetadataLanguage               string         `json:"PreferredMetadataLanguage,omitempty"`
	MetadataCountryCode                     string         `json:"MetadataCountryCode,omitempty"`
	SeasonZeroDisplayName                   string         `json:"SeasonZeroDisplayName,omitempty"`
	MetadataSavers                          []string       `json:"MetadataSavers,omitempty"`
	DisabledLocalMetadataReaders            []string       `json:"DisabledLocalMetadataReaders,omitempty"`
	LocalMetadataReaderOrder                []string       `json:"LocalMetadataReaderOrder,omitempty"`
	DisabledSubtitleFetchers                []string       `json:"DisabledSubtitleFetchers,omitempty"`
	SubtitleFetcherOrder                    []string       `json:"SubtitleFetcherOrder,omitempty"`
	SkipSubtitlesIfEmbeddedSubtitlesPresent bool           `json:"SkipSubtitlesIfEmbeddedSubtitlesPresent,omitempty"`
	SkipSubtitlesIfAudioTrackMatches        bool           `json:"SkipSubtitlesIfAudioTrackMatches,omitempty"`
	SubtitleDownloadLanguages               []string       `json:"SubtitleDownloadLanguages,omitempty"`
	RequirePerfectSubtitleMatch             bool           `json:"RequirePerfectSubtitleMatch,omitempty"`
	SaveSubtitlesWithMedia                  bool           `json:"SaveSubtitlesWithMedia,omitempty"`
	SaveLyricsWithMedia                     bool           `json:"SaveLyricsWithMedia,omitempty"`
	AutomaticallyAddToCollection            bool           `json:"AutomaticallyAddToCollection,omitempty"`
	AllowEmbeddedSubtitles                  string         `json:"AllowEmbeddedSubtitles,omitempty"`
	TypeOptions                             []JFTypeOption `json:"TypeOptions,omitempty"`
}

type JFMediaLibrary struct {
	Name               string           `json:"Name"`
	Locations          []string         `json:"Locations,omitempty"`
	CollectionType     string           `json:"CollectionType,omitempty"`
	LibraryOptions     JFLibraryOptions `json:"LibraryOptions,omitempty"`
	ItemId             string           `json:"ItemId,omitempty"`
	PrimaryImageItemId string           `json:"PrimaryImageItemId,omitempty"`
	RefreshStatus      string           `json:"RefreshStatus,omitempty"`
}

type JFPlayState struct {
	CanSeek         bool   `json:"CanSeek"`
	RepeatMode      string `json:"RepeatMode"`
	PositionTicks   int    `json:"PositionTicks"`
	PlaySessionID   string `json:"PlaySessionId"`
	MediaSourceID   string `json:"MediaSourceId"`
	ItemID          string `json:"ItemId"`
	PlayMethod      string `json:"PlayMethod"`
	IsMuted         bool   `json:"IsMuted"`
	EventName       string `json:"EventName"`
	NowPlayingQueue []struct {
		PlaylistItemID string `json:"PlaylistItemId"`
		ID             string `json:"Id"`
	} `json:"NowPlayingQueue"`
	PlaylistLength int  `json:"PlaylistLength"`
	PlaylistIndex  int  `json:"PlaylistIndex"`
	IsPaused       bool `json:"IsPaused"`
}
