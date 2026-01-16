package api

import "time"

type ErrorResponse struct {
	Error string `json:"error"`
}

type StartStopResponse struct {
	Server    string    `json:"server"`
	Running   bool      `json:"running"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at,omitempty"`
	ExitedAt  time.Time `json:"exited_at,omitempty"`
	ExitCode  int       `json:"exit_code,omitempty"`
	LastError string    `json:"last_error,omitempty"`
	LogPath   string    `json:"log_path,omitempty"`
}

type ListResponse struct {
	Servers []StartStopResponse `json:"servers"`
}
