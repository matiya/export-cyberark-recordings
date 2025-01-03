package pvwaAPI

import (
	// "bufio"
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"golang.org/x/term"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// pvwaClient is a type that holds the relevant information for the program
// see the field documentation
// pvwaClient handles all communication with the PVWA API.
// It maintains authentication state and provides methods
// for retrieving and downloading PSM session recordings.
type pvwaClient struct {
	// BaseURL is the root endpoint for the PVWA API service.
	BaseURL string
	// Username is the username of any user that can sww the recordings
	Username string
	// Authtoken is set automatically when calling NewPVWAConfig()
	AuthToken string
	// the resty client will be reused between calls
	Client *resty.Client
}

// DownloadRecordings retrieves the video files for all recordings in the provided
// SessionRecordings and saves them to the specified output directory.
// Each recording is saved as an .avi file named with its SessionID.
// The function handles large files by streaming the download in chunks.
func (p *pvwaClient) DownloadRecordings(outputPath string, sessions *SessionRecordings) error {
	slog.Info("starting download of recordings",
		"count", len(sessions.Recordings),
		"path", outputPath)

	// Create the output directory
	err := os.MkdirAll(outputPath, 0755)
	if err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	for _, recording := range sessions.Recordings {
		// Create the output file
		filePath := filepath.Join(outputPath, recording.SessionID+".avi")
		out, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("error creating output file: %w", err)
		}
		defer out.Close()

		// Make a streaming GET request
		resp, err := p.Client.R().
			SetDoNotParseResponse(true). // Important: don't parse response
			SetHeader("Accept", "*/*").
			SetHeader("authorization", p.AuthToken).
			Post(p.BaseURL + "/recordings/" + recording.SessionID + "/Play/")

		if err != nil {
			return fmt.Errorf("error making request: %w", err)
		}

		// Check response status
		if resp.StatusCode() != 200 {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
		}

		// Close the response body when done
		rawBody := resp.RawBody()
		if rawBody == nil {
			return fmt.Errorf("no response body received")
		}
		defer rawBody.Close()

		buffer := make([]byte, 32*1024) // 32KB chunks
		totalBytes := 0

		// Read and write in chunks
		for {
			n, err := rawBody.Read(buffer)
			if n > 0 {
				// Write the chunk to file
				_, writeErr := out.Write(buffer[:n])
				if writeErr != nil {
					return fmt.Errorf("error writing to file: %v", writeErr)
				}
				totalBytes += n

				fmt.Printf("\r\tDownloading %s: %d bytes", recording.SessionID, totalBytes)
			}

			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("error reading response: %v", err)
			}
		}

		slog.Info("download complete",
			"sessionID", recording.SessionID,
			"bytes", totalBytes,
			"file", filePath)
	}

	return nil
}

// GetRecordings will set the Recordings type in pvwaClient with information about
// recordings up to limit
// Check the SessionRecording type to see what information is available
// GetRecordings retrieves a list of recordings from the PVWA API based on the provided
// query parameters. Common parameters include:
//   - offset: Starting position for pagination
//   - sort: Field to sort by
//   - order: Sort order (asc/desc)
//   - fromtime: Start time as Unix timestamp
//   - totime: End time as Unix timestamp
//
// The function automatically handles pagination for results over 1000 records.
func (p *pvwaClient) GetRecordings(queryParams map[string]string) (*SessionRecordings, error) {
	slog.Info("retrieving recordings", "params", queryParams)
	const maxResultsPerPage = 1000
	allRecordings := &SessionRecordings{
		Recordings: make([]Recording, 0),
	}

	// Start with offset 0
	offset := 0
	for {
		// Update offset in query parameters
		currentParams := make(map[string]string)
		for k, v := range queryParams {
			currentParams[k] = v
		}
		currentParams["offset"] = fmt.Sprintf("%d", offset)

		var pageRecordings SessionRecordings
		_, err := p.Client.R().
			SetResult(&pageRecordings).
			SetQueryParams(currentParams).
			SetHeader("authorization", p.AuthToken).
			Get(p.BaseURL + "/recordings")

		if err != nil {
			return nil, fmt.Errorf("could not retrieve recordings at offset %d: %w", offset, err)
		}

		slog.Info("retrieved page of recordings",
			"offset", offset,
			"count", len(pageRecordings.Recordings),
			"total", pageRecordings.Total)

		// Add this page's recordings to our result
		allRecordings.Recordings = append(allRecordings.Recordings, pageRecordings.Recordings...)
		allRecordings.Total = pageRecordings.Total

		// If we got fewer results than the max, we're done
		if len(pageRecordings.Recordings) < maxResultsPerPage {
			break
		}

		// Move to next page
		offset += maxResultsPerPage

		// Safety check: if we've retrieved more than the total, something's wrong
		if offset >= pageRecordings.Total {
			break
		}
	}

	return allRecordings, nil
}

// GetAllRecordings retrieves recordings without filter.
// Note that this will max out at 1000, if there are more
// then use GetRecordingsByMonth
// GetAllRecordings retrieves all available recordings without any filtering.
// Note that this is limited to the first 1000 results due to API limitations.
// For more results, use GetRecordingsByMonth to paginate by time periods.
func (p *pvwaClient) GetAllRecordings() (*SessionRecordings, error) {
	queryParams := map[string]string{
		"offset": "0",
		"sort":   "name",
		"order":  "asc",
	}

	r, err := p.GetRecordings(queryParams)
	if err != nil {
		return nil, fmt.Errorf("Could not get all recordings: %w", err)
	}

	return r, nil
}

// GetRecordingsByMonth retrieves recordings for a specific month in 2024.
// The month parameter should be 1-12 representing the calendar month.
// This method helps work around the 1000 record limit by breaking queries
// into monthly chunks.
func (p *pvwaClient) GetRecordingsByMonth(month int) (*SessionRecordings, error) {

	from := time.Date(2024, time.Month(month), 0, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0).Add(-time.Second) // Last second of the month

	queryParams := map[string]string{
		"offset":   "0",
		"sort":     "name",
		"order":    "asc",
		"fromtime": fmt.Sprintf("%d", from.Unix()),
		"totime":   fmt.Sprintf("%d", to.Unix()),
	}

	r, err := p.GetRecordings(queryParams)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// GetAuthToken logins to the PVWA and returns an authorization token
// GetAuthToken authenticates with the PVWA API using the client's username
// and the provided password. On successful authentication, it stores the
// returned auth token in the client for subsequent requests.
func (p *pvwaClient) GetAuthToken(password string) error {

	authToken, err := p.Client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(`{"username":"` + p.Username + `", "password":"` + password + `"}`).
		Post(p.BaseURL + "/auth/CyberArk/Logon")

	if err != nil {
		return fmt.Errorf("error obtaining authorization token: %w", err)
	}
	authTokenTrimmed := strings.Trim(string(authToken.Body()), "\"")
	p.AuthToken = authTokenTrimmed
	return nil

}

// SaveToJSON saves the SessionRecordings structure to a JSON file
// SaveToJSON writes each Recording in the SessionRecordings to a separate
// JSON file in the specified directory. Each file is named using the
// recording's SessionID with a .json extension. The directory will be
// created if it doesn't exist.
func (s *SessionRecordings) SaveToJSON(dirname string) error {
	slog.Info("saving recordings to JSON",
		"directory", dirname,
		"count", len(s.Recordings))
	// create directory if it doesn't exist
	if err := os.MkdirAll(dirname, 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}
	// Convert the structure to JSON with proper indentation
	for _, session := range s.Recordings {
		jsonData, err := json.MarshalIndent(session, "", "    ")
		if err != nil {
			return fmt.Errorf("error marshaling to JSON: %w", err)
		}

		// Write to file
		filename := filepath.Join(dirname, session.SessionID+".json")
		slog.Info("saved recording JSON", "file", filename)
		err = os.WriteFile(filename, jsonData, 0644)
		if err != nil {
			return fmt.Errorf("error writing JSON to file: %w", err)
		}
	}

	return nil
}

// NewPVWAConfig creates a new authenticated PVWA API client.
// It requires a base URL for the API endpoint and a username.
// The password will be read from the PVWA_PASSWORD environment variable,
// or if not set, the user will be prompted to enter it securely.
// Returns an error if authentication fails or if required parameters are missing.
func NewPVWAConfig(baseURL string, username string) (*pvwaClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL cannot be empty")
	}

	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	password := os.Getenv("PVWA_PASSWORD")
	if password == "" {
		fmt.Printf("Please enter password for user %s: ", username)
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, fmt.Errorf("error reading password: %w", err)
		}
		fmt.Println() // Add a newline after the password input
		password = strings.TrimSpace(string(bytePassword))
		if password == "" {
			return nil, fmt.Errorf("password cannot be empty")
		}
	}

	pvwaConfig := &pvwaClient{
		BaseURL:  baseURL,
		Username: username,
		Client:   resty.New(),
	}

	err := pvwaConfig.GetAuthToken(password)
	if err != nil {
		return nil, fmt.Errorf("could not get an authorization token %w", err)
	}

	return pvwaConfig, nil

}
