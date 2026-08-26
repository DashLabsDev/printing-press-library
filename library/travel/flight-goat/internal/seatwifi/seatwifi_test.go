// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package seatwifi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetFlight_Parses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/flights/UA1234" {
			t.Errorf("path = %s, want /api/v1/flights/UA1234", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"flight_number": "UA1234",
			"wifi_provider": "starlink",
			"confidence":    0.92,
			"airline":       "United Airlines",
			"airline_code":  "UA",
			"aircraft_type": "BOEING 737-800 (738)",
			"source":        "database",
		})
	}))
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	got, err := c.GetFlight(context.Background(), "UA1234")
	if err != nil {
		t.Fatalf("GetFlight: %v", err)
	}
	if got.FlightNumber != "UA1234" || got.WifiProvider != "starlink" || got.AirlineCode != "UA" {
		t.Fatalf("got %+v", got)
	}
	if got.Confidence != 0.92 {
		t.Fatalf("confidence = %v", got.Confidence)
	}
}

func TestListAirlines_Parses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/airlines" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"code": "UA", "name": "United Airlines", "wifiProvider": []string{"starlink", "viasat"}},
			{"code": "AA", "name": "American Airlines", "wifiProvider": []string{"viasat"}},
		})
	}))
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	list, err := c.ListAirlines(context.Background())
	if err != nil {
		t.Fatalf("ListAirlines: %v", err)
	}
	if len(list) != 2 || list[0].Code != "UA" {
		t.Fatalf("list = %+v", list)
	}
}

func TestGetAirline_Parses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/airlines/UA" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "UA", "name": "United Airlines", "wifiProvider": []string{"starlink"}, "fleetInfo": "Free Starlink",
		})
	}))
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	got, err := c.GetAirline(context.Background(), "ua")
	if err != nil {
		t.Fatalf("GetAirline: %v", err)
	}
	if got.Code != "UA" || len(got.WifiProvider) == 0 {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(got.FleetInfo, "Starlink") {
		t.Fatalf("fleetInfo = %q", got.FleetInfo)
	}
}

func TestGetRollouts_AllAndByAirline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/rollouts":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalAirlines": 2, "totalRollouts": 3, "byAirline": map[string]interface{}{"UA": []interface{}{map[string]interface{}{"airlineCode": "UA", "aircraftType": "737", "status": "active"}}},
			})
		case "/api/rollouts/UA":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"airline": "UA", "rollouts": []interface{}{map[string]interface{}{"airlineCode": "UA", "aircraftType": "737", "status": "active"}},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	all, err := c.GetRollouts(context.Background())
	if err != nil {
		t.Fatalf("GetRollouts all: %v", err)
	}
	if all.TotalAirlines != 2 {
		t.Fatalf("all = %+v", all)
	}
	one, err := c.GetAirlineRollouts(context.Background(), "UA")
	if err != nil {
		t.Fatalf("GetAirlineRollouts UA: %v", err)
	}
	if one.Airline != "UA" || len(one.Rollouts) != 1 {
		t.Fatalf("one = %+v", one)
	}
}

func TestSearch_Parses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" || r.URL.Query().Get("q") != "united" {
			t.Errorf("req = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"flightNumber": "UA1234", "airline": map[string]string{"code": "UA", "name": "United Airlines"}, "wifiStatus": "starlink"},
		})
	}))
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	res, err := c.Search(context.Background(), "united")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 || res[0].FlightNumber != "UA1234" {
		t.Fatalf("res = %+v", res)
	}
}

func TestGetFlight_EmptyRejected(t *testing.T) {
	c := NewClient()
	if _, err := c.GetFlight(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty flight")
	}
}

func TestGetSpeedStats_Parses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/speed-reports/stats/UA1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"avgDownload": 42.5, "avgUpload": 8.1, "avgLatency": 80, "totalReports": 12,
		})
	}))
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	got, err := c.GetSpeedStats(context.Background(), "ua1")
	if err != nil {
		t.Fatalf("GetSpeedStats: %v", err)
	}
	if got.AvgDownload != 42.5 || got.TotalReports != 12 {
		t.Fatalf("got %+v", got)
	}
}

func TestClient_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	_, err := c.GetAirline(context.Background(), "ZZ")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404, got %v", err)
	}
}
