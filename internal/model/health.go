package model

// HealthResponse is the JSON response returned by the health endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}
