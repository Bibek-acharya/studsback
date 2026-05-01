package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	APIBase   = "http://localhost:8080"
	Email     = "durgeshbhandari834@gmail.com"
	Password  = "QdDeC5SxMXPw"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		User  User   `json:"user"`
		Token string `json:"token"`
	} `json:"data"`
}

type User struct {
	ID           int    `json:"id"`
	ProviderName string `json:"provider_name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
}

type ScholarshipRequest struct {
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	Value                string   `json:"value"`
	ScholarshipType      string   `json:"scholarship_type"`
	FundingType          string   `json:"funding_type"`
	DegreeLevel          string   `json:"degree_level"`
	Location             string   `json:"location"`
	Status               string   `json:"status"`
	FieldOfStudy         []string `json:"field_of_study"`
	EligibilityCriteria  []string `json:"eligibility_criteria"`
	RequiredDocuments    []string `json:"required_documents"`
	TotalSeats           int      `json:"total_seats"`
	AmountPerStudent     int      `json:"amount_per_student"`
	ApplicationStartDate string   `json:"application_start_date"`
	ApplicationEndDate   string   `json:"application_end_date"`
}

type ScholarshipResponse struct {
	Success bool   `json:"success"`
	Data    struct {
		ID     int    `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	} `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

func main() {
	fmt.Println("=== Seed Script: Create Draft Scholarship ===")
	fmt.Println()

	// Step 1: Login
	fmt.Println("1. Logging in as scholarship provider...")
	token, providerID, err := login()
	if err != nil {
		fmt.Printf("✗ Login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Login successful! Provider ID: %d\n", providerID)

	// Step 2: Create scholarship
	fmt.Println("\n2. Creating scholarship draft...")
	scholarshipID, err := createScholarship(token)
	if err != nil {
		fmt.Printf("✗ Create failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Seed Complete ===")
	fmt.Printf("✓ Scholarship draft created successfully!\n")
	fmt.Printf("  ID: %d\n", scholarshipID)
	fmt.Printf("  Title: Project Sikshya Merit Scholarship 2026\n")
	fmt.Printf("  Status: draft\n")
	fmt.Println()
	fmt.Println("You can view/edit this draft in the scholarship provider dashboard.")
}

func login() (string, int, error) {
	reqBody := LoginRequest{Email: Email, Password: Password}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", APIBase+"/api/v1/scholarship-providers/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", 0, fmt.Errorf("decode failed: %w", err)
	}

	if !loginResp.Success || loginResp.Data.Token == "" {
		return "", 0, fmt.Errorf("login failed")
	}

	return loginResp.Data.Token, loginResp.Data.User.ID, nil
}

func createScholarship(token string) (int, error) {
	now := time.Now()
	endDate := now.Add(90 * 24 * time.Hour)

	scholarship := ScholarshipRequest{
		Title:               "Project Sikshya Merit Scholarship 2026",
		Description:         "A merit-based scholarship for outstanding students pursuing undergraduate studies in Nepal. This scholarship aims to support academically gifted students who demonstrate financial need and have a track record of extracurricular achievements.",
		Value:               "50000",
		ScholarshipType:     "merit-based",
		FundingType:         "partial",
		DegreeLevel:         "bachelor",
		Location:            "Kathmandu, Nepal",
		Status:              "draft",
		FieldOfStudy:        []string{"Science", "Management", "Engineering", "Humanities"},
		EligibilityCriteria: []string{
			"Must have completed +2 or equivalent with minimum 75% marks",
			"Annual family income should be less than NPR 500,000",
			"Must be a Nepali citizen",
			"Minimum GPA requirement: 3.0/4.0 in previous academic year",
		},
		RequiredDocuments: []string{
			"Marksheet of +2 or equivalent",
			"Character certificate",
			"Income certificate",
			"Citizenship copy",
			"Recent passport size photos",
		},
		TotalSeats:           25,
		AmountPerStudent:     50000,
		ApplicationStartDate: now.Format(time.RFC3339),
		ApplicationEndDate:   endDate.Format(time.RFC3339),
	}

	body, _ := json.Marshal(scholarship)
	req, _ := http.NewRequest("POST", APIBase+"/api/v1/scholarship-providers/scholarships", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var scholarshipResp ScholarshipResponse
	if err := json.NewDecoder(resp.Body).Decode(&scholarshipResp); err != nil {
		return 0, fmt.Errorf("decode failed: %w", err)
	}

	if !scholarshipResp.Success {
		return 0, fmt.Errorf("create failed: %s", scholarshipResp.Message)
	}

	return scholarshipResp.Data.ID, nil
}