package keep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

// KeepClient interface defines the methods that need to be implemented
type KeepClient interface {
	GetAvailableProviders() ([]interface{}, *ErrorResponse, error)
	GetInstalledProviders() ([]interface{}, *ErrorResponse, error)
	InstallProvider(providerConfig map[string]interface{}) (map[string]interface{}, *ErrorResponse, error)
	DeleteProvider(providerType, providerID string) (*ErrorResponse, error)
	InstallProviderWebhook(providerType, providerID string) (*ErrorResponse, error)
}

// Client struct with Api Key needed to authenticate against keep
type Client struct {
	HostURL    string
	HTTPClient *http.Client
	ApiKey     string
}

// Ensure Client implements KeepClient interface
var _ KeepClient = &Client{}

// Helper function to check if the error is related to missing scopes
func isScopesError(body []byte) (bool, string) {
	var errorResp struct {
		Detail map[string]string `json:"detail"`
	}
	if err := json.Unmarshal(body, &errorResp); err != nil {
		return false, ""
	}

	if len(errorResp.Detail) > 0 {
		missingScopes := make([]string, 0)
		for scope, msg := range errorResp.Detail {
			if msg == "Missing scope" {
				missingScopes = append(missingScopes, scope)
			}
		}
		if len(missingScopes) > 0 {
			return true, fmt.Sprintf("Missing required scopes: %v", missingScopes)
		}
	}
	return false, ""
}

// ErrorResponse struct for API error responses
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// NewClient func creates new client
func NewClient(hostUrl string, apiKey string, timeout time.Duration) *Client {
	c := Client{
		HTTPClient: &http.Client{Timeout: timeout},
		HostURL:    hostUrl,
		ApiKey:     apiKey,
	}
	return &c
}

// doReq func does the api requests
func (c *Client) doReq(req *http.Request) ([]byte, *ErrorResponse, error) {
	req.Header.Set("X-API-Key", c.ApiKey)

	// Only set Content-Type if not already set
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if isScopeError, scopeDetails := isScopesError(body); isScopeError {
			return nil, &ErrorResponse{
				Error:   "Insufficient permissions",
				Details: scopeDetails,
			}, fmt.Errorf("API request failed: insufficient permissions")
		}

		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && (errResp.Error != "" || errResp.Details != "") {
			return nil, &errResp, fmt.Errorf("API request failed with status %d", resp.StatusCode)
		}
		return nil, &ErrorResponse{
			Error:   fmt.Sprintf("request failed with status %d", resp.StatusCode),
			Details: string(body),
		}, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil, nil

}

// Provider-specific API methods

func (c *Client) GetAvailableProviders() ([]interface{}, *ErrorResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/providers", c.HostURL), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %v", err)
	}

	body, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, fmt.Errorf("failed to get available providers: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %v. Response body: %s", err, string(body))
	}

	providers, ok := response["providers"].([]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("invalid response format: 'providers' field is missing or has wrong type. Response: %v", response)
	}

	return providers, nil, nil
}

func (c *Client) GetInstalledProviders() ([]interface{}, *ErrorResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/providers/export", c.HostURL), nil)
	if err != nil {
		return nil, nil, err
	}

	body, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var providers []interface{}
	if err := json.Unmarshal(body, &providers); err != nil {
		return nil, nil, err
	}

	return providers, nil, nil
}

func (c *Client) InstallProvider(providerConfig map[string]interface{}) (map[string]interface{}, *ErrorResponse, error) {
	payload, err := json.Marshal(providerConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal provider config: %v", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/providers/install", c.HostURL),
		strings.NewReader(string(payload)))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %v", err)
	}

	body, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, fmt.Errorf("failed to install provider: %v", err)
	}

	if body == nil {
		return nil, nil, fmt.Errorf("received empty response body")
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %v. Response body: %s", err, string(body))
	}

	return response, nil, nil
}

func (c *Client) InstallProviderWebhook(providerType, providerID string) (*ErrorResponse, error) {
	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/providers/install/webhook/%s/%s", c.HostURL, providerType, providerID),
		nil)
	if err != nil {
		return nil, err
	}

	_, errResp, err := c.doReq(req)
	if err != nil {
		return errResp, err
	}

	return nil, nil
}

func (c *Client) DeleteProvider(providerType, providerID string) (*ErrorResponse, error) {
	req, err := http.NewRequest("DELETE",
		fmt.Sprintf("%s/providers/%s/%s", c.HostURL, providerType, providerID),
		nil)
	if err != nil {
		return nil, err
	}

	_, errResp, err := c.doReq(req)
	if err != nil {
		return errResp, err
	}

	return nil, nil
}

func (c *Client) TestProvider(providerType, providerID string) (*ErrorResponse, error) {
	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/providers/%s/%s/test", c.HostURL, providerType, providerID),
		nil)
	if err != nil {
		return nil, err
	}

	_, errResp, err := c.doReq(req)
	if err != nil {
		return errResp, err
	}

	return nil, nil
}

// Workflow API methods
func (c *Client) ListWorkflows() ([]interface{}, *ErrorResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/workflows", c.HostURL), nil)
	if err != nil {
		return nil, nil, err
	}

	body, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var workflows []interface{}
	if err := json.Unmarshal(body, &workflows); err != nil {
		return nil, nil, err
	}

	return workflows, nil, nil
}

func (c *Client) GetWorkflow(id string) (map[string]interface{}, *ErrorResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/workflows/%s", c.HostURL, id), nil)
	if err != nil {
		return nil, nil, err
	}

	body, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, nil, err
	}

	return response, nil, nil
}

func (c *Client) CreateWorkflow(filePath string) (map[string]interface{}, *ErrorResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := writer.CreateFormFile("file", filePath)
	if err != nil {
		return nil, nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	if _, err := file.Write(content); err != nil {
		return nil, nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/workflows", c.HostURL), body)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	respBody, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, nil, err
	}

	return response, nil, nil
}

func (c *Client) UpdateWorkflow(id string, filePath string) (map[string]interface{}, *ErrorResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := writer.CreateFormFile("file", filePath)
	if err != nil {
		return nil, nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	if _, err := file.Write(content); err != nil {
		return nil, nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/workflows/%s", c.HostURL, id), body)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	respBody, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, nil, err
	}

	return response, nil, nil
}

func (c *Client) DeleteWorkflow(id string) (*ErrorResponse, error) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/workflows/%s", c.HostURL, id), nil)
	if err != nil {
		return nil, err
	}

	_, errResp, err := c.doReq(req)
	if err != nil {
		return errResp, err
	}

	return nil, nil
}

// Mapping API methods
func (c *Client) GetMappings() ([]interface{}, *ErrorResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/mapping", c.HostURL), nil)
	if err != nil {
		return nil, nil, err
	}

	body, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var mappings []interface{}
	if err := json.Unmarshal(body, &mappings); err != nil {
		return nil, nil, err
	}

	return mappings, nil, nil
}

func (c *Client) CreateMapping(mapping map[string]interface{}) (map[string]interface{}, *ErrorResponse, error) {
	payload, err := json.Marshal(mapping)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/mapping", c.HostURL),
		strings.NewReader(string(payload)))
	if err != nil {
		return nil, nil, err
	}

	body, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, nil, err
	}

	return response, nil, nil
}

func (c *Client) DeleteMapping(id string) (*ErrorResponse, error) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/mapping/%s", c.HostURL, id), nil)
	if err != nil {
		return nil, err
	}

	_, errResp, err := c.doReq(req)
	if err != nil {
		return errResp, err
	}

	return nil, nil
}

// Extraction API methods
func (c *Client) GetExtractions() ([]interface{}, *ErrorResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/extraction", c.HostURL), nil)
	if err != nil {
		return nil, nil, err
	}

	body, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var extractions []interface{}
	if err := json.Unmarshal(body, &extractions); err != nil {
		return nil, nil, err
	}

	return extractions, nil, nil
}

func (c *Client) CreateExtraction(extraction map[string]interface{}) (map[string]interface{}, *ErrorResponse, error) {
	payload, err := json.Marshal(extraction)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/extraction", c.HostURL),
		strings.NewReader(string(payload)))
	if err != nil {
		return nil, nil, err
	}

	body, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, nil, err
	}

	return response, nil, nil
}

func (c *Client) UpdateExtraction(id string, extraction map[string]interface{}) (*ErrorResponse, error) {
	payload, err := json.Marshal(extraction)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/extraction/%s", c.HostURL, id),
		strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}

	_, errResp, err := c.doReq(req)
	if err != nil {
		return errResp, err
	}

	return nil, nil
}

func (c *Client) DeleteExtraction(id string) (*ErrorResponse, error) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/extraction/%s", c.HostURL, id), nil)
	if err != nil {
		return nil, err
	}

	_, errResp, err := c.doReq(req)
	if err != nil {
		return errResp, err
	}

	return nil, nil
}

// Helper function to convert YAML to JSON-compatible map
func yamlToJSONMap(content []byte) (map[string]interface{}, error) {
	var yamlData map[interface{}]interface{}
	if err := yaml.Unmarshal(content, &yamlData); err != nil {
		return nil, fmt.Errorf("invalid YAML: %s", err)
	}

	// Convert YAML map to JSON-compatible map
	jsonData := make(map[string]interface{})
	for k, v := range yamlData {
		switch val := v.(type) {
		case map[interface{}]interface{}:
			jsonData[k.(string)] = convertToStringMap(val)
		case []interface{}:
			jsonData[k.(string)] = convertToStringSlice(val)
		default:
			jsonData[k.(string)] = val
		}
	}
	return jsonData, nil
}

// Helper function to convert map[interface{}]interface{} to map[string]interface{}
func convertToStringMap(m map[interface{}]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case map[interface{}]interface{}:
			result[k.(string)] = convertToStringMap(val)
		case []interface{}:
			result[k.(string)] = convertToStringSlice(val)
		default:
			result[k.(string)] = val
		}
	}
	return result
}

// Helper function to convert []interface{} elements
func convertToStringSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[interface{}]interface{}:
			result[i] = convertToStringMap(val)
		case []interface{}:
			result[i] = convertToStringSlice(val)
		default:
			result[i] = val
		}
	}
	return result
}

func (c *Client) CreateWorkflowJSON(workflow map[string]interface{}) (map[string]interface{}, *ErrorResponse, error) {
	payload, err := json.Marshal(workflow)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/workflows/json", c.HostURL), strings.NewReader(string(payload)))
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	respBody, errResp, err := c.doReq(req)
	if err != nil {
		return nil, errResp, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, nil, err
	}

	return response, nil, nil
}

func ClientConfigurer(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	host, err := url.Parse(d.Get("backend_url").(string))
	if err != nil {
		return nil, diag.Errorf("backend_url was not a valid url: %s", err.Error())
	}

	timeout, err := time.ParseDuration(d.Get("timeout").(string))
	if err != nil {
		return nil, diag.Errorf("timeout was not a valid duration: %s", err.Error())
	}

	return NewClient(host.String(), d.Get("api_key").(string), timeout), nil
}
