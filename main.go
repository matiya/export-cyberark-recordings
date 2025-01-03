package main

import (
	"export-recordings/api"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	// Configure structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting recording export")
	// Get options
	pvwaAddress := flag.String("baseURL", "https://pvwa.example.com", "The base URL for PVWA")
	username := flag.String("username", "svc-session-checker", "The username for a user with auditor rights")
	monthsFlag := flag.String("months", "1-12", "Months to process (e.g. '5,6,7' or '1-12')")
	flag.Parse()

	// Parse months flag
	var months []int = parseMonths(*monthsFlag)

	// Initialize the client
	pvwaClient, err := pvwaAPI.NewPVWAConfig(
		*pvwaAddress,
		*username,
	)

	if err != nil {
		log.Fatal("error at pvwaClient: \n", err)
	}

	for _, m := range months {
		slog.Info("processing month", "month", m)

		sessions, err := pvwaClient.GetRecordingsByMonth(m)
		if err != nil {
			log.Fatal("error getting recordings for month: ", m, "\n", err)
		}

		slog.Info("found recordings",
			"month", m,
			"count", sessions.Total,
			"retrieved", len(sessions.Recordings))
		outputPath := filepath.Join(".", "downloaded_recordings/", fmt.Sprintf("%d/", m))
		sessions.SaveToJSON(outputPath)
		pvwaClient.DownloadRecordings(outputPath, sessions)

	}

}

func parseMonths(monthsFlag string) []int {
	var months []int

	if strings.Contains(monthsFlag, "-") {
		// Handle range format (e.g. "1-12")
		parts := strings.Split(monthsFlag, "-")
		if len(parts) != 2 {
			log.Fatal("invalid month range format. Use 'start-end' (e.g. '1-12')")
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Fatal("invalid start month:", err)
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Fatal("invalid end month:", err)
		}
		for i := start; i <= end; i++ {
			if i < 1 || i > 12 {
				log.Fatal("months must be between 1 and 12")
			}
			months = append(months, i)
		}
	} else {
		// Handle comma-separated format (e.g. "5,6,7")
		for _, m := range strings.Split(monthsFlag, ",") {
			month, err := strconv.Atoi(strings.TrimSpace(m))
			if err != nil {
				log.Fatal("invalid month:", err)
			}
			if month < 1 || month > 12 {
				log.Fatal("months must be between 1 and 12")
			}
			months = append(months, month)
		}
	}
	return months
}
