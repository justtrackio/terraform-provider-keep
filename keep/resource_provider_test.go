package keep

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccResourceProvider(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceProviderBasicConfig(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceProviderExists("keep_provider.test"),
					resource.TestCheckResourceAttr("keep_provider.test", "type", "aks"),
					resource.TestCheckResourceAttr("keep_provider.test", "name", "test-aks"),
				),
			},
		},
		CheckDestroy: testAccCheckResourceProviderDestroy,
	})
}

func testAccCheckResourceProviderExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("resource not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no provider ID is set")
		}

		client := testAccProvider.Meta().(*Client)
		time.Sleep(2 * time.Second) // Wait for provider creation

		providers, errResp, err := client.GetInstalledProviders()
		if err != nil {
			if errResp != nil {
				return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
			}
			return fmt.Errorf("error getting installed providers: %s", err)
		}

		for _, provider := range providers {
			p := provider.(map[string]interface{})
			if p["id"] == rs.Primary.ID {
				return nil
			}
		}

		return fmt.Errorf("provider not found")
	}
}

func testAccCheckResourceProviderDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "keep_provider" {
			continue
		}

		providers, errResp, err := client.GetInstalledProviders()
		if err != nil {
			if errResp != nil {
				// Ignore API errors during destroy check as the resource might be already gone
				continue
			}
			return fmt.Errorf("error getting installed providers: %s", err)
		}

		for _, provider := range providers {
			p := provider.(map[string]interface{})
			if p["id"] == rs.Primary.ID {
				return fmt.Errorf("provider still exists")
			}
		}
	}

	return nil
}

func testAccResourceProviderBasicConfig() string {
	return fmt.Sprintf(`
%s

resource "keep_provider" "test" {
  type = "aks"
  name = "test-aks"
  auth_config = {
    subscription_id     = "%s"
    client_id          = "%s"
    client_secret      = "%s"
    tenant_id          = "%s"
    resource_group_name = "%s"
    resource_name      = "%s"
  }
}
`, testAccProviderBasicConfig(),
		os.Getenv("AKS_SUBSCRIPTION_ID"), os.Getenv("AKS_CLIENT_ID"),
		os.Getenv("AKS_CLIENT_SECRET"), os.Getenv("AKS_TENANT_ID"),
		os.Getenv("AKS_RESOURCE_GROUP_NAME"), os.Getenv("AKS_RESOURCE_NAME"))
}

func TestAccResourceProvider_WebhookErrors(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceProviderWebhookConfig(),
				ExpectError: regexp.MustCompile(`Failed to install provider: request failed with status 412`),
			},
		},
	})
}

func TestAccResourceProvider_UpdateErrors(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceProviderBasicConfig(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceProviderExists("keep_provider.test"),
				),
			},
			{
				Config:      testAccResourceProviderUpdateConfig(),
				ExpectError: regexp.MustCompile(`Failed to install provider: request failed with status 412`),
			},
		},
	})
}

func testAccResourceProviderWebhookConfig() string {
	return fmt.Sprintf(`
%s

resource "keep_provider" "test_webhook" {
  type = "grafana"
  name = "test-grafana-webhook"
  auth_config = {
    host  = "https://grafana.example.com"
    token = "invalid-token"
  }
  install_webhook = true
}
`, testAccProviderBasicConfig())
}

func testAccResourceProviderUpdateConfig() string {
	return fmt.Sprintf(`
%s

resource "keep_provider" "test" {
  type = "grafana"
  name = "test-grafana-update"
  auth_config = {
    host  = "https://grafana.example.com"
    token = "invalid-token"
  }
}
`, testAccProviderBasicConfig())
}

func TestAccResourceProvider_UpdateWebhookError(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceProviderBasicConfig(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceProviderExists("keep_provider.test"),
				),
			},
			{
				Config:      testAccResourceProviderUpdateWithWebhookConfig(),
				ExpectError: regexp.MustCompile(`Failed to install provider: request failed with status 412`),
			},
		},
	})
}

func testAccResourceProviderUpdateWithWebhookConfig() string {
	return fmt.Sprintf(`
%s

resource "keep_provider" "test" {
  type = "grafana"
  name = "test-grafana-webhook"
  auth_config = {
    host  = "https://grafana.example.com"
    token = "invalid-token"
  }
  install_webhook = true
}
`, testAccProviderBasicConfig())
}

// Add mock client tests
func TestResourceProvider_MockWebhookError(t *testing.T) {
	client := &mockClient{
		response:   []byte(`{"detail":{"webhook.install:write":"Missing scope"}}`),
		statusCode: 412,
	}

	d := schema.TestResourceDataRaw(t, resourceProvider().Schema, map[string]interface{}{
		"type":            "test",
		"name":            "test",
		"auth_config":     map[string]interface{}{"key": "value"},
		"install_webhook": true,
	})

	diags := resourceCreateProvider(context.Background(), d, client)
	if diags == nil {
		t.Fatal("expected error diagnostics")
	}

	found := false
	for _, diag := range diags {
		if strings.Contains(diag.Summary, "Failed to install provider") {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected provider installation error")
	}
}

func TestResourceProvider_MockErrors(t *testing.T) {
	cases := []struct {
		name          string
		response      []byte
		statusCode    int
		expectedError string
	}{
		{
			name:          "missing scopes",
			response:      []byte(`{"detail":{"alert.rules:read":"Missing scope"}}`),
			statusCode:    412,
			expectedError: "Failed to install provider: request failed with status 412",
		},
		{
			name:          "empty response",
			response:      []byte(`{}`),
			statusCode:    200,
			expectedError: "Provider installation failed: no ID returned",
		},
		{
			name:          "invalid response",
			response:      []byte(`invalid json`),
			statusCode:    200,
			expectedError: "failed to parse response",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &mockClient{
				response:   tc.response,
				statusCode: tc.statusCode,
			}

			d := schema.TestResourceDataRaw(t, resourceProvider().Schema, map[string]interface{}{
				"type":        "test",
				"name":        "test",
				"auth_config": map[string]interface{}{"key": "value"},
			})

			diags := resourceCreateProvider(context.Background(), d, client)
			if diags == nil {
				t.Fatal("expected error diagnostics")
			}

			found := false
			for _, diag := range diags {
				if strings.Contains(diag.Summary, tc.expectedError) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("expected error containing %q, got %q", tc.expectedError, diags[0].Summary)
			}
		})
	}
}

// Mock client for unit tests
type mockClient struct {
	response   []byte
	statusCode int
}

func (m *mockClient) GetAvailableProviders() ([]interface{}, *ErrorResponse, error) {
	return []interface{}{
		map[string]interface{}{
			"type": "test",
		},
	}, nil, nil
}

func (m *mockClient) GetInstalledProviders() ([]interface{}, *ErrorResponse, error) {
	if m.statusCode != http.StatusOK {
		return nil, &ErrorResponse{
			Error:   fmt.Sprintf("request failed with status %d", m.statusCode),
			Details: string(m.response),
		}, fmt.Errorf("API request failed with status %d", m.statusCode)
	}
	return []interface{}{}, nil, nil
}

func (m *mockClient) InstallProvider(providerConfig map[string]interface{}) (map[string]interface{}, *ErrorResponse, error) {
	if m.statusCode != http.StatusOK && m.statusCode != http.StatusCreated {
		return nil, &ErrorResponse{
			Error:   fmt.Sprintf("request failed with status %d", m.statusCode),
			Details: string(m.response),
		}, fmt.Errorf("API request failed with status %d", m.statusCode)
	}

	if len(m.response) == 0 || string(m.response) == "{}" {
		return map[string]interface{}{}, nil, nil
	}

	var response map[string]interface{}
	if err := json.Unmarshal(m.response, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return response, nil, nil
}

func (m *mockClient) DeleteProvider(providerType, providerID string) (*ErrorResponse, error) {
	if m.statusCode != http.StatusOK {
		return &ErrorResponse{
			Error:   fmt.Sprintf("request failed with status %d", m.statusCode),
			Details: string(m.response),
		}, fmt.Errorf("API request failed with status %d", m.statusCode)
	}
	return nil, nil
}

func (m *mockClient) InstallProviderWebhook(providerType, providerID string) (*ErrorResponse, error) {
	if m.statusCode != http.StatusOK {
		return &ErrorResponse{
			Error:   fmt.Sprintf("request failed with status %d", m.statusCode),
			Details: string(m.response),
		}, fmt.Errorf("API request failed with status %d", m.statusCode)
	}
	return nil, nil
}
