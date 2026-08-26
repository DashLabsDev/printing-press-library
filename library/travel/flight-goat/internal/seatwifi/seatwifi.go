// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Package seatwifi is a thin native client for the SeatWifi public JSON API
// (https://seatwifi.com/for-developers). SeatWifi aggregates airline WiFi
// provider predictions per flight/tail/airline plus Starlink rollout tracking
// and crowdsourced speed reports. All endpoints are public (no auth) and return
// JSON, so flight-goat can surface them as read-only enrichment alongside
// Google Flights / Seats.aero without gating on a SeatWifi key.
//
// This mirrors the hand-written seatsaero backend pattern: not generated from
// OpenAPI, kept small, testable against httptest.

package seatwifi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the SeatWifi JSON API root (see https://seatwifi.com/for-developers).
const DefaultBaseURL = "https://seatwifi.com"

// Client talks to the SeatWifi JSON API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient builds a Client with defaults.
func NewClient() *Client {
	return &Client{
		BaseURL: strings.TrimRight(DefaultBaseURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Airline describes /api/airlines entries.
type Airline struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	WifiProvider []string `json:"wifiProvider"`
	FleetInfo    string   `json:"fleetInfo"`
	LastUpdated  string   `json:"lastUpdated"`
	Vertical     string   `json:"vertical"`
}

// FlightWifi describes GET /api/v1/flights/:flightNumber.
type FlightWifi struct {
	FlightNumber       string  `json:"flight_number"`
	WifiProvider       string  `json:"wifi_provider"`
	Confidence         float64 `json:"confidence"`
	Airline            string  `json:"airline"`
	AirlineCode        string  `json:"airline_code"`
	AircraftType       string  `json:"aircraft_type"`
	TailNumber         string  `json:"tail_number,omitempty"`
	Method             string  `json:"method,omitempty"`
	Explanation        string  `json:"explanation,omitempty"`
	Details            *string `json:"details"`
	StarlinkStatus     string  `json:"starlink_status,omitempty"`
	StarlinkLikelihood string  `json:"starlink_likelihood,omitempty"`
	FleetPercentage    *int    `json:"fleet_percentage,omitempty"`
	LastVerified       *string `json:"last_verified"`
	Source             string  `json:"source,omitempty"`
}

// Rollout describes one entry under /api/rollouts.
type Rollout struct {
	AirlineCode        string  `json:"airlineCode"`
	AircraftType       string  `json:"aircraftType"`
	Status             string  `json:"status"`
	Notes              string  `json:"notes"`
	SourceURL          *string `json:"sourceUrl"`
	LastChecked        string  `json:"lastChecked"`
	UpdatedAt          string  `json:"updatedAt"`
	FleetPercentage    *int    `json:"fleetPercentage"`
	AnnouncedDate      string  `json:"announcedDate"`
	ExpectedCompletion *string `json:"expectedCompletion"`
}

// RolloutsResponse is GET /api/rollouts.
type RolloutsResponse struct {
	TotalAirlines int                  `json:"totalAirlines"`
	TotalRollouts int                  `json:"totalRollouts"`
	ByAirline     map[string][]Rollout `json:"byAirline"`
}

// AirlineRolloutsResponse is GET /api/rollouts/:code.
type AirlineRolloutsResponse struct {
	Airline  string    `json:"airline"`
	Rollouts []Rollout `json:"rollouts"`
}

// SpeedStats describes GET /api/speed-reports/stats/:flight and /api/speed-reports/airline/:code.
type SpeedStats struct {
	AvgDownload     float64                    `json:"avgDownload"`
	AvgUpload       float64                    `json:"avgUpload"`
	AvgLatency      float64                    `json:"avgLatency"`
	MinDownload     float64                    `json:"minDownload"`
	MaxDownload     float64                    `json:"maxDownload"`
	TotalReports    int                        `json:"totalReports"`
	VerifiedReports int                        `json:"verifiedReports"`
	ByProvider      map[string]json.RawMessage `json:"byProvider"`
}

// SearchResult describes one entry of GET /api/search?q=.
type SearchResult struct {
	FlightNumber string `json:"flightNumber"`
	Airline      struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"airline"`
	Aircraft     string  `json:"aircraft"`
	WifiStatus   string  `json:"wifiStatus"`
	Confidence   float64 `json:"confidence"`
	LastVerified string  `json:"lastVerified"`
	LastUpdated  string  `json:"lastUpdated"`
	Details      string  `json:"details"`
	IsOutdated   bool    `json:"isOutdated"`
	DataAge      int     `json:"dataAge"`
}

func (c *Client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flight-goat-pp-cli/seatwifi")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("seatwifi %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("seatwifi %s: read body: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("seatwifi %s: HTTP %d: %s", path, resp.StatusCode, truncate(body))
	}
	return body, nil
}

// GetFlight returns WiFi prediction for a flight number (e.g. "UA1234").
func (c *Client) GetFlight(ctx context.Context, flightNumber string) (*FlightWifi, error) {
	flightNumber = strings.ToUpper(strings.TrimSpace(flightNumber))
	if flightNumber == "" {
		return nil, fmt.Errorf("flight number required")
	}
	body, err := c.get(ctx, "/api/v1/flights/"+url.PathEscape(flightNumber), nil)
	if err != nil {
		return nil, err
	}
	var out FlightWifi
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("seatwifi flight decode: %w (body=%s)", err, truncate(body))
	}
	return &out, nil
}

// ListAirlines returns all tracked airlines.
func (c *Client) ListAirlines(ctx context.Context) ([]Airline, error) {
	body, err := c.get(ctx, "/api/airlines", nil)
	if err != nil {
		return nil, err
	}
	var out []Airline
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("seatwifi airlines decode: %w", err)
	}
	return out, nil
}

// GetAirline returns WiFi details for one IATA airline code.
func (c *Client) GetAirline(ctx context.Context, code string) (*Airline, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return nil, fmt.Errorf("airline code required")
	}
	body, err := c.get(ctx, "/api/airlines/"+url.PathEscape(code), nil)
	if err != nil {
		return nil, err
	}
	var out Airline
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("seatwifi airline decode: %w", err)
	}
	return &out, nil
}

// GetRollouts returns Starlink rollout status for every tracked airline.
func (c *Client) GetRollouts(ctx context.Context) (*RolloutsResponse, error) {
	body, err := c.get(ctx, "/api/rollouts", nil)
	if err != nil {
		return nil, err
	}
	var out RolloutsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("seatwifi rollouts decode: %w", err)
	}
	return &out, nil
}

// GetAirlineRollouts returns Starlink rollout status for one IATA airline code.
func (c *Client) GetAirlineRollouts(ctx context.Context, airlineCode string) (*AirlineRolloutsResponse, error) {
	airlineCode = strings.TrimSpace(strings.ToUpper(airlineCode))
	if airlineCode == "" {
		return nil, fmt.Errorf("airline code required")
	}
	body, err := c.get(ctx, "/api/rollouts/"+url.PathEscape(airlineCode), nil)
	if err != nil {
		return nil, err
	}
	var out AirlineRolloutsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("seatwifi airline rollouts decode: %w", err)
	}
	return &out, nil
}

// GetSpeedStats returns crowdsourced speed stats for a flight number.
func (c *Client) GetSpeedStats(ctx context.Context, flight string) (*SpeedStats, error) {
	flight = strings.ToUpper(strings.TrimSpace(flight))
	if flight == "" {
		return nil, fmt.Errorf("flight required")
	}
	body, err := c.get(ctx, "/api/speed-reports/stats/"+url.PathEscape(flight), nil)
	if err != nil {
		return nil, err
	}
	var out SpeedStats
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("seatwifi speed decode: %w", err)
	}
	return &out, nil
}

// GetAirlineSpeedStats returns speed stats aggregated by airline code.
func (c *Client) GetAirlineSpeedStats(ctx context.Context, code string) (*SpeedStats, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return nil, fmt.Errorf("airline code required")
	}
	body, err := c.get(ctx, "/api/speed-reports/airline/"+url.PathEscape(code), nil)
	if err != nil {
		return nil, err
	}
	var out SpeedStats
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("seatwifi airline speed decode: %w", err)
	}
	return &out, nil
}

// Search searches airlines & flights (GET /api/search?q=).
func (c *Client) Search(ctx context.Context, q string) ([]SearchResult, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, fmt.Errorf("query required")
	}
	body, err := c.get(ctx, "/api/search", url.Values{"q": []string{q}})
	if err != nil {
		return nil, err
	}
	var out []SearchResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("seatwifi search decode: %w", err)
	}
	return out, nil
}

func truncate(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
