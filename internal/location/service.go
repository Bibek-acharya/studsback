package location

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Service struct {
	httpClient *http.Client
}

type NominatimResult struct {
	PlaceID     int64    `json:"place_id"`
	Lat         string   `json:"lat"`
	Lon         string   `json:"lon"`
	DisplayName string   `json:"display_name"`
	Type        string   `json:"type"`
	BoundingBox []string `json:"boundingbox"`
}

func NewService() *Service {
	return &Service{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *Service) Geocode(query string) ([]NominatimResult, error) {
	if query == "" {
		return nil, nil
	}

	encodedQuery := url.QueryEscape(query + ", Nepal")
	geocodeURL := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=5&addressdetails=1", encodedQuery)

	req, err := http.NewRequest("GET", geocodeURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Studsphere/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding failed with status: %d", resp.StatusCode)
	}

	var results []NominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}
