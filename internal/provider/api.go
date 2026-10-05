package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// apiError appends the API's response body to an error message. go-atlassian
// reduces a 400 to "client: atlassian invalid payload" and drops the errors
// object, which is the only part that says which field was rejected.
func apiError(err error, response *models.ResponseScheme) string {
	if response == nil {
		return err.Error()
	}

	body := strings.TrimSpace(response.Bytes.String())
	if body == "" {
		return err.Error()
	}

	if len(body) > 512 {
		body = body[:512] + "..."
	}

	return fmt.Sprintf("%s (HTTP %d): %s", err.Error(), response.Code, body)
}

func logAPIError(ctx context.Context, message string, response *models.ResponseScheme) {
	if response == nil {
		return
	}

	tflog.Error(ctx, message, map[string]interface{}{
		"url":         response.Endpoint,
		"method":      response.Method,
		"status_code": response.Code,
		"body":        response.Bytes.String(),
	})
}
