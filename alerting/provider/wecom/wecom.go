package wecom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/TwiN/gatus/v5/alerting/alert"
	"github.com/TwiN/gatus/v5/client"
	"github.com/TwiN/gatus/v5/core"
	"io"
	"net/http"
	"time"
)

// AlertProvider is the configuration necessary for sending an alert using Slack
type AlertProvider struct {
	WebhookURL string `yaml:"webhook-url"` // Slack webhook URL
	// DefaultAlert is the default alert configuration to use for endpoints with an alert of the appropriate type
	DefaultAlert *alert.Alert `yaml:"default-alert,omitempty"`
	// Overrides is a list of Override that may be prioritized over the default configuration
	Overrides []Override `yaml:"overrides,omitempty"`
}

// Override is a case under which the default integration is overridden
type Override struct {
	Group      string `yaml:"group"`
	WebhookURL string `yaml:"webhook-url"`
}

// IsValid returns whether the provider's configuration is valid
func (provider *AlertProvider) IsValid() bool {
	registeredGroups := make(map[string]bool)
	if provider.Overrides != nil {
		for _, override := range provider.Overrides {
			if isAlreadyRegistered := registeredGroups[override.Group]; isAlreadyRegistered || override.Group == "" || len(override.WebhookURL) == 0 {
				return false
			}
			registeredGroups[override.Group] = true
		}
	}
	return len(provider.WebhookURL) > 0
}

// Send an alert using the provider
func (provider *AlertProvider) Send(endpoint *core.Endpoint, alert *alert.Alert, result *core.Result, resolved bool) error {
	buffer := bytes.NewBuffer(provider.buildRequestBody(endpoint, alert, result, resolved))
	request, err := http.NewRequest(http.MethodPost, provider.getWebhookURLForGroup(endpoint.Group), buffer)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.GetHTTPClient(nil).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode > 399 {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("call to provider alert returned status code %d: %s", response.StatusCode, string(body))
	}
	return err
}

type Body struct {
	Msgtype  string   `json:"msgtype"`
	Markdown Markdown `json:"markdown"`
}

type Markdown struct {
	Content string `json:"content"`
}

func (provider *AlertProvider) buildRequestBody(endpoint *core.Endpoint, alert *alert.Alert, result *core.Result, resolved bool) []byte {
	var title, conditions, message string
	if resolved {
		title = fmt.Sprint("# <font color=\"info\">Alert Resolved</font>\n")
	} else {
		title = fmt.Sprint("# <font color=\"warning\">Alert Triggered</font>\n")
	}
	conditions = "## Condition:\n"
	for _, conditionResult := range result.ConditionResults {
		var prefix string
		if conditionResult.Success {
			prefix = "✅"
		} else {
			prefix = "❌"
		}
		conditions += fmt.Sprintf("%s - `%s`\n", prefix, conditionResult.Condition)
	}
	var description string
	if alertDescription := alert.GetDescription(); len(alertDescription) > 0 {
		description = alertDescription
	}
	var info string
	info = "## Endpoint Info\n"
	info += fmt.Sprintf("> group: <font color=\"comment\">%s</font>\n", endpoint.Group)
	info += fmt.Sprintf("> name: <font color=\"comment\">%s</font>\n", endpoint.Name)
	info += fmt.Sprintf("> url: [%s](%s)\n", endpoint.URL, endpoint.URL)
	info += fmt.Sprintf("> describe: <font color=\"comment\">%s</font>\n", description)
	info += fmt.Sprintf("> update time: %s\n\n", genUTC8time())
	message = title + info + conditions
	body, _ := json.Marshal(Body{
		Msgtype: "markdown",
		Markdown: Markdown{
			Content: message,
		},
	})
	return body
}

// getWebhookURLForGroup returns the appropriate Webhook URL integration to for a given group
func (provider *AlertProvider) getWebhookURLForGroup(group string) string {
	if provider.Overrides != nil {
		for _, override := range provider.Overrides {
			if group == override.Group {
				return override.WebhookURL
			}
		}
	}
	return provider.WebhookURL
}

// GetDefaultAlert returns the provider's default alert configuration
func (provider AlertProvider) GetDefaultAlert() *alert.Alert {
	return provider.DefaultAlert
}

func genUTC8time() string {
	timelocal := time.FixedZone("UTC", 3600*8)
	time.Local = timelocal
	return time.Now().Local().Format("2006-01-02 15:04:05")
}
