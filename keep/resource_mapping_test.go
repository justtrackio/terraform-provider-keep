package keep

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func setupTestFiles(t testing.TB) (string, func()) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "keep_test")
	if err != nil {
		t.Fatal(err)
	}

	// Create mapping file
	mappingContent := `alert_name,severity,team,action
high_error_rate,critical,platform,page
memory_usage,warning,infrastructure,notify
disk_space,critical,infrastructure,page
api_latency,warning,backend,notify`

	mappingPath := filepath.Join(tmpDir, "alerts.csv")
	if err := os.WriteFile(mappingPath, []byte(mappingContent), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func testAccMappingConfig(mappingPath string) string {
	return fmt.Sprintf(`
resource "keep_mapping" "test" {
  name              = "alerts-mapping"
  description       = "Mapping for alert rules"
  mapping_file_path = "%s"
  matchers = [
    "alert_name=~'.*error.*'",
    "severity='critical'"
  ]
  priority = 1
}`, mappingPath)
}

func cleanupExistingMappings() error {
	client := initTestClient()

	mappings, errResp, err := client.GetMappings()
	if err != nil {
		if errResp != nil {
			return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return fmt.Errorf("error getting mappings: %s", err)
	}

	for _, m := range mappings {
		mapping := m.(map[string]interface{})
		errResp, err := client.DeleteMapping(fmt.Sprintf("%v", mapping["id"]))
		if err != nil {
			if errResp != nil {
				return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
			}
			return fmt.Errorf("error deleting mapping %v: %s", mapping["id"], err)
		}
	}

	return nil
}

func TestAccResourceMapping_basic(t *testing.T) {
	tmpDir, cleanup := setupTestFiles(t)
	defer cleanup()

	mappingPath := filepath.Join(tmpDir, "alerts.csv")
	resourceName := "keep_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if err := cleanupExistingMappings(); err != nil {
				t.Fatalf("error cleaning up mappings: %s", err)
			}
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWorkflowDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + "\n" +
					testAccMappingConfig(mappingPath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "alerts-mapping"),
					resource.TestCheckResourceAttr(resourceName, "description", "Mapping for alert rules"),
					resource.TestCheckResourceAttr(resourceName, "priority", "1"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"csv_content_hash"},
			},
		},
	})
}

func TestAccResourceMapping_prometheusLabels(t *testing.T) {
	csvContent := `source,labels.priority,priority,severity,priority:color
prometheus,critical,P1,critical,#fc164e
prometheus,high,P2,high,#ef5d1e
prometheus,warning,P3,warning,#e0a426
prometheus,info,P4,info,#5484cc
prometheus,low,P5,low,#7e7e7e`

	tmpDir, err := os.MkdirTemp("", "mapping_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpfilePath := filepath.Join(tmpDir, "prometheus_priority.csv")

	if err := os.WriteFile(tmpfilePath, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if err := cleanupExistingMappings(); err != nil {
				t.Fatalf("error cleaning up mappings: %s", err)
			}
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWorkflowDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + fmt.Sprintf(`
resource "keep_mapping" "prometheus_priority" {
    name        = "prometheus-priority"
    description = "Prometheus priority mapping"
    priority    = 1
    matchers    = ["source && labels.priority"]
    mapping_file_path = "%s"
}
`, tmpfilePath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceExists("keep_mapping.prometheus_priority"),
					resource.TestCheckResourceAttr("keep_mapping.prometheus_priority", "name", "prometheus-priority"),
					resource.TestCheckResourceAttr("keep_mapping.prometheus_priority", "description", "Prometheus priority mapping"),
				),
			},
		},
	})
}

// Add helper function to check mapping count
func testAccCheckMappingCount(expected int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccProvider.Meta().(*Client)

		mappings, errResp, err := client.GetMappings()
		if err != nil {
			if errResp != nil {
				return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
			}
			return fmt.Errorf("error getting mappings: %s", err)
		}

		if len(mappings) != expected {
			return fmt.Errorf("expected %d mappings, got %d", expected, len(mappings))
		}

		return nil
	}
}

func TestAccResourceMapping_disappears(t *testing.T) {
	tmpDir, cleanup := setupTestFiles(t)
	defer cleanup()

	mappingPath := filepath.Join(tmpDir, "alerts.csv")
	resourceName := "keep_mapping.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if err := cleanupExistingMappings(); err != nil {
				t.Fatalf("error cleaning up mappings: %s", err)
			}
		},
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + "\n" +
					testAccMappingConfig(mappingPath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceExists(resourceName),
					testAccCheckResourceDisappears(resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckResourceExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("resource ID is not set")
		}

		return nil
	}
}

func testAccCheckResourceDisappears(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("resource ID is not set")
		}

		// Extract mapping ID from composite ID if present
		id := rs.Primary.ID
		var mappingID string
		if strings.Contains(id, ":") {
			parts := strings.Split(id, ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid resource ID format")
			}
			mappingID = parts[0]
		} else {
			mappingID = id
		}

		client := testAccProvider.Meta().(*Client)
		errResp, err := client.DeleteMapping(mappingID)
		if err != nil {
			if errResp != nil {
				return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
			}
			return fmt.Errorf("error deleting mapping: %s", err)
		}

		return nil

	}
}

func TestAccResourceMapping_csvContentChange(t *testing.T) {
	tmpDir, cleanup := setupTestFiles(t)
	defer cleanup()

	mappingPath := filepath.Join(tmpDir, "alerts.csv")
	resourceName := "keep_mapping.test"

	// Updated content with additional rules
	updatedContent := `alert_name,severity,team,action
high_error_rate,critical,platform,page
memory_usage,warning,infrastructure,notify
disk_space,critical,infrastructure,page
api_latency,warning,backend,notify
database_errors,critical,database,page
network_issues,warning,network,notify`

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if err := cleanupExistingMappings(); err != nil {
				t.Fatalf("error cleaning up mappings: %s", err)
			}
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWorkflowDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + "\n" +
					testAccMappingConfig(mappingPath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "alerts-mapping"),
				),
			},
			{
				PreConfig: func() {
					if err := os.WriteFile(mappingPath, []byte(updatedContent), 0644); err != nil {
						t.Fatal(err)
					}
				},
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + "\n" +
					testAccMappingConfig(mappingPath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "alerts-mapping"),
				),
			},
		},
	})
}
