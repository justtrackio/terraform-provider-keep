package keep

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func initTestClient() *Client {
	return NewClient(
		os.Getenv("KEEP_BACKEND_URL"),
		os.Getenv("KEEP_API_KEY"),
		30*time.Second,
	)
}

func testAccProviderConfig(backendURL, apiKey string) string {
	return fmt.Sprintf(`
provider "keep" {
  backend_url = "%s"
  api_key     = "%s"
}`, backendURL, apiKey)
}

func testAccProviderBasicConfig() string {
	return fmt.Sprintf(`
provider "keep" {
  backend_url = "%s"
  api_key     = "%s"
  timeout     = "30s"
}`, os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY"))
}

var testAccProvider *schema.Provider
var testAccProviderFactories map[string]func() (*schema.Provider, error)

func init() {
	testAccProvider = Provider()
	err := testAccProvider.Configure(context.Background(), &terraform.ResourceConfig{
		Raw: map[string]interface{}{
			"backend_url": os.Getenv("KEEP_BACKEND_URL"),
			"api_key":     os.Getenv("KEEP_API_KEY"),
			"timeout":     os.Getenv("KEEP_TIMEOUT"),
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to configure provider: %v", err))
	}

	testAccProviderFactories = map[string]func() (*schema.Provider, error){
		"keep": func() (*schema.Provider, error) {
			return testAccProvider, nil
		},
	}
}

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvider_impl(t *testing.T) {
	var _ *schema.Provider = Provider()
}

func testAccPreCheck(t *testing.T) {
	requiredEnvVars := []string{
		"KEEP_BACKEND_URL",
		"KEEP_API_KEY",
		"AKS_SUBSCRIPTION_ID",
		"AKS_CLIENT_ID",
		"AKS_CLIENT_SECRET",
		"AKS_TENANT_ID",
		"AKS_RESOURCE_GROUP_NAME",
		"AKS_RESOURCE_NAME",
	}

	for _, envVar := range requiredEnvVars {
		if v := os.Getenv(envVar); v == "" {
			t.Skipf("%s must be set for acceptance tests", envVar)
		}
	}

	// Clean up any existing test providers
	client := testAccProvider.Meta().(*Client)
	cleanupTestProviders(t, client, []string{"test-aks", "test-aks-updated"})

	// Check if API is accessible
	providers, errResp, err := client.GetAvailableProviders()
	if err != nil {
		if errResp != nil {
			t.Fatalf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		t.Fatalf("Error checking provider API: %s", err)
	}
	if len(providers) == 0 {
		t.Fatalf("No providers found")
	}
}

func cleanupTestProviders(t *testing.T, client *Client, names []string) {
	// Get all installed providers
	providers, errResp, err := client.GetInstalledProviders()
	if err != nil {
		if errResp != nil {
			t.Logf("Warning: API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		t.Logf("Warning: Failed to get installed providers: %s", err)
		return
	}

	// Create a map for quick lookup
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	// Try to delete each matching provider
	for _, provider := range providers {
		p := provider.(map[string]interface{})
		if details, ok := p["details"].(map[string]interface{}); ok {
			if name, exists := details["name"].(string); exists && nameMap[name] {
				providerType := p["type"].(string)
				providerID := p["id"].(string)

				// Try to delete the provider
				errResp, err := client.DeleteProvider(providerType, providerID)
				if err != nil {
					if errResp != nil {
						t.Logf("Warning: API Error: %s. Details: %s", errResp.Error, errResp.Details)
					}
					t.Logf("Warning: Failed to cleanup provider %s: %s", name, err)
					continue
				}

				// Wait for deletion to complete
				time.Sleep(2 * time.Second)
				t.Logf("Successfully cleaned up provider %s", name)
			}
		}
	}
}

func TestAccProvider_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderBasicConfig(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckProviderBasicConfig(),
				),
			},
		},
	})
}

func testAccCheckProviderBasicConfig() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccProvider.Meta().(*Client)
		if client.HostURL == "" {
			return fmt.Errorf("provider backend_url not configured")
		}

		if client.ApiKey == "" {
			return fmt.Errorf("provider api_key not configured")
		}

		return nil
	}
}
