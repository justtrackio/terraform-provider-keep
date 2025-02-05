package keep

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
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
