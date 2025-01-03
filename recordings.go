package pvwaAPI

// SessionRecordings represents a collection of PSM session recordings
// retrieved from the PVWA API.
type SessionRecordings struct {
	// Recordings contains the list of individual recording sessions
	Recordings []Recording `json:"Recordings"`
	// Total is the count of all available recordings matching the query
	Total      int         `json:"Total"`
}

// Recording contains metadata about a single PSM recording session.
// Each recording represents a single user session that was captured
// by the PSM server.
type Recording struct {
	SessionID             string          `json:"SessionID"`
	SessionGuid           string          `json:"SessionGuid"`
	SafeName              string          `json:"SafeName"`
	FileName              string          `json:"FileName"`
	Start                 int64           `json:"Start"`
	End                   int64           `json:"End"`
	Duration              int             `json:"Duration"`
	User                  string          `json:"User"`
	RemoteMachine         string          `json:"RemoteMachine"`
	AccountUsername       string          `json:"AccountUsername"`
	AccountPlatformID     string          `json:"AccountPlatformID"`
	AccountAddress        string          `json:"AccountAddress"`
	RecordedActivities    []interface{}   `json:"RecordedActivities"`
	ConnectionComponentID string          `json:"ConnectionComponentID"`
	FromIP                string          `json:"FromIP"`
	Client                string          `json:"Client"`
	RiskScore             float64         `json:"RiskScore"`
	Severity              string          `json:"Severity"`
	RecordingFiles        []RecordingFile `json:"RecordingFiles"`
	VideoSize             int             `json:"VideoSize"`
	TextSize              int             `json:"TextSize"`
	DetailsUrl            string          `json:"DetailsUrl"`
}

type RecordingFile struct {
	FileName           string `json:"FileName"`
	RecordingType      int    `json:"RecordingType"`
	LastReviewBy       string `json:"LastReviewBy"`
	LastReviewDate     int64  `json:"LastReviewDate"`
	FileSize           int64  `json:"FileSize"`
	CompressedFileSize int64  `json:"CompressedFileSize"`
	Format             string `json:"Format"`
}
