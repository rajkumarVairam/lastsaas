package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"saasquickstart/internal/models"
)

// Handler executes jobs of a specific type.
//
// Idempotency contract: if job.Attempts > 0 and job.Result is non-nil, a prior
// attempt already reached the external API. Check job.Result for a stored ID
// (e.g., a social media post ID) and skip the API call if the work was done.
// Store the external ID in job.Result before returning nil so the next crash-retry
// can detect it.
type Handler interface {
	Type() string
	Execute(ctx context.Context, job *models.Job) error
}

// DecodePayload unmarshals the job payload into v using JSON round-trip.
// Use this in handlers to get a typed struct from the opaque payload map.
func DecodePayload(job *models.Job, v interface{}) error {
	b, err := json.Marshal(job.Payload)
	if err != nil {
		return fmt.Errorf("jobs: marshal payload: %w", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("jobs: unmarshal payload into %T: %w", v, err)
	}
	return nil
}

// DecodeResult unmarshals job.Result into v using JSON round-trip.
func DecodeResult(job *models.Job, v interface{}) error {
	if len(job.Result) == 0 {
		return nil
	}
	b, err := json.Marshal(job.Result)
	if err != nil {
		return fmt.Errorf("jobs: marshal result: %w", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("jobs: unmarshal result into %T: %w", v, err)
	}
	return nil
}
