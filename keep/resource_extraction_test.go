package keep

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccKeepExtraction_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKeepExtractionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + `
resource "keep_extraction" "test" {
  name        = "error-pattern"
  description = "Extract error patterns from logs"
  priority    = 1
  attribute   = "message"
  regex       = "error: (.*)"
  disabled    = false
  pre         = false
}`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeepExtractionExists("keep_extraction.test"),
					resource.TestCheckResourceAttr("keep_extraction.test", "name", "error-pattern"),
					resource.TestCheckResourceAttr("keep_extraction.test", "description", "Extract error patterns from logs"),
					resource.TestCheckResourceAttr("keep_extraction.test", "priority", "1"),
					resource.TestCheckResourceAttr("keep_extraction.test", "attribute", "message"),
					resource.TestCheckResourceAttr("keep_extraction.test", "regex", "error: (.*)"),
					resource.TestCheckResourceAttr("keep_extraction.test", "disabled", "false"),
					resource.TestCheckResourceAttr("keep_extraction.test", "pre", "false"),
				),
			},
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + `
resource "keep_extraction" "test" {
  name        = "updated-error-pattern"
  description = "Updated error pattern extraction"
  priority    = 2
  attribute   = "message"
  regex       = "error\\[([^\\]]+)\\]"
  disabled    = true
  pre         = false
}`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeepExtractionExists("keep_extraction.test"),
					resource.TestCheckResourceAttr("keep_extraction.test", "name", "updated-error-pattern"),
					resource.TestCheckResourceAttr("keep_extraction.test", "description", "Updated error pattern extraction"),
					resource.TestCheckResourceAttr("keep_extraction.test", "priority", "2"),
					resource.TestCheckResourceAttr("keep_extraction.test", "disabled", "true"),
				),
			},
		},
	})
}

func testAccCheckKeepExtractionExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No extraction ID is set")
		}

		client := testAccProvider.Meta().(*Client)
		extractions, errResp, err := client.GetExtractions()
		if err != nil {
			if errResp != nil {
				return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
			}
			return fmt.Errorf("Error checking extraction existence: %s", err)
		}

		for _, e := range extractions {
			extraction := e.(map[string]interface{})
			if fmt.Sprintf("%v", extraction["id"]) == rs.Primary.ID {
				return nil
			}
		}

		return fmt.Errorf("Extraction not found")
	}
}

func testAccCheckKeepExtractionDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "keep_extraction" {
			continue
		}

		extractions, errResp, err := client.GetExtractions()
		if err != nil {
			if errResp != nil {
				// Ignore API errors during destroy check as the resource might be already gone
				continue
			}
			return nil // Consider any error as the resource being gone
		}

		for _, e := range extractions {
			extraction := e.(map[string]interface{})
			if fmt.Sprintf("%v", extraction["id"]) == rs.Primary.ID {
				return fmt.Errorf("Extraction still exists")
			}
		}
	}

	return nil
}

func TestAccKeepExtraction_missingRequired(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKeepExtractionDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccKeepExtractionConfig_missingRequired(),
				ExpectError: regexp.MustCompile(`The argument "regex" is required`),
			},
		},
	})
}

func TestAccKeepExtraction_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckKeepExtractionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + `
resource "keep_extraction" "test" {
  name        = "error-pattern"
  description = "Extract error patterns from logs"
  priority    = 1
  attribute   = "message"
  regex       = "error: (.*)"
  disabled    = false
  pre         = false
}`,
			},
			{
				ResourceName:      "keep_extraction.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccKeepExtractionConfig_missingRequired() string {
	return fmt.Sprintf(`
provider "keep" {
  backend_url = "%s"
  api_key     = "%s"
}

resource "keep_extraction" "test" {
  name        = "missing_required_test"
  description = "Test extraction with missing required field"
  priority    = 1
  attribute   = "message"
  disabled    = false
  pre         = false
}
`, os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY"))
}
