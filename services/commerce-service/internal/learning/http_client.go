package learning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrProvisionFailed means learning-service did not confirm enrollment.
// Callers preserve the paid order, record the failure, and leave retry
// possible later — a purchase is never lost to a provisioning outage.
var ErrProvisionFailed = errors.New("learning provisioning failed")

// HTTPProvisioner enrolls users via learning-service's manual path. v1
// development seam: it posts a trusted internal enrollment request. The
// future payment-service/commerce event flow will replace the transport,
// not the interface.
type HTTPProvisioner struct {
	baseURL string
	client  *http.Client
}

func NewHTTPProvisioner(baseURL string, timeout time.Duration) *HTTPProvisioner {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &HTTPProvisioner{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *HTTPProvisioner) ProvisionEnrollment(
	ctx context.Context,
	userID uuid.UUID,
	courseID uuid.UUID,
	source string,
) error {
	payload, err := json.Marshal(map[string]string{
		"userId":   userID.String(),
		"courseId": courseID.String(),
		"source":   source,
	})
	if err != nil {
		return fmt.Errorf("%w: encode: %v", ErrProvisionFailed, err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/internal/enrollments",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProvisionFailed, err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProvisionFailed, err)
	}
	defer response.Body.Close()

	_, _ = io.Copy(io.Discard, response.Body)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrProvisionFailed, response.StatusCode)
	}

	return nil
}
