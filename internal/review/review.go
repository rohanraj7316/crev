package review

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/vossenwout/crev/internal/files"
)

type ReviewInput struct {
	Code string `json:"code"`
}

type ReviewOutput struct {
	Review string `json:"review"`
}

const geminiURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"

// Gemini API request structures
// See: https://ai.google.dev/api/rest/v1beta/GenerateContentRequest

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func prepareGeminiRequest(prompt string, apiKey string) (*http.Request, error) {
	requestBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{{Text: prompt}},
			},
		},
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	// Add API key as query param
	urlWithKey := geminiURL + "?key=" + apiKey

	req, err := http.NewRequest("POST", urlWithKey, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// Gemini uses X-goog-api-key as a query param, not header, so no need to set header
	return req, nil
}

func sendRequest(req *http.Request) (*http.Response, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending review request to %s: %v", geminiURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error: received status code %d: %s", resp.StatusCode, string(body))
		if resp.StatusCode == http.StatusUnauthorized {
			log.Fatalf("Unauthorized: you have provided an invalid CREV API key.")
		} else {
			log.Fatalf("Failed to review code: status code %d", resp.StatusCode)
		}
		return nil, err
	}
	return resp, nil
}

func saveReviewToFile(output ReviewOutput) error {
	err := files.SaveStringToFile(output.Review, "crev-review.md")
	if err != nil {
		return err
	}
	log.Printf("Successfully saved code review to crev-review.md")
	return nil
}

func Review(codeToReview string, apiKey string) {
	log.Printf("Reviewing code please wait...")

	// Prepare the Gemini request
	req, err := prepareGeminiRequest(codeToReview, apiKey)
	if err != nil {
		log.Fatalf("Error preparing Gemini request: %v", err)
	}

	resp, err := sendRequest(req)
	if err != nil {
		log.Fatalf("Error sending Gemini request: %v", err)
	}
	defer resp.Body.Close()

	var geminiResp GeminiResponse
	err = json.NewDecoder(resp.Body).Decode(&geminiResp)
	if err != nil {
		log.Fatalf("Error decoding Gemini response: %v", err)
	}

	var reviewText string
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		reviewText = geminiResp.Candidates[0].Content.Parts[0].Text
	} else {
		reviewText = "No review generated."
	}

	output := ReviewOutput{Review: reviewText}
	err = saveReviewToFile(output)
	if err != nil {
		log.Fatalf("Error saving review to file: %v", err)
	}
}
