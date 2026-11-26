package controller

import "go-conference/core"

// Tipe di bawah dipakai untuk dokumentasi Swagger (swag init); bentuknya sama dengan respons.

// ErrorResponse is returned with 4xx/5xx statuses.
type ErrorResponse struct {
	Error string `json:"error" example:"isSender must be a boolean"`
}

// IceServerDoc is one RTCIceServer entry.
type IceServerDoc struct {
	URLs       []string `json:"urls" example:"turn:turn.cloudflare.com:3478?transport=tcp"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// IceResponse is the body of GET /webrtc/ice.
type IceResponse struct {
	IceServers []IceServerDoc `json:"iceServers"`
}

// PeersResponse is the body of GET /webrtc/m/{meetingId}/peers.
type PeersResponse struct {
	Peers []string `json:"peers" example:"u-abc,u-def"`
}

// MeetingsResponse is the body of GET /webrtc/meetings.
type MeetingsResponse struct {
	Meetings []core.MeetingInfo `json:"meetings"`
}

// HealthResponse is the body of GET /health.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}
