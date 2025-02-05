package keep

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func testAccWorkflowConfig(workflowPath string) string {
	return fmt.Sprintf(`
resource "keep_workflow" "test" {
  workflow_file_path = "%s"
}`, workflowPath)
}

func TestAccKeepWorkflow_Change(t *testing.T) {
	workflowContent := `workflow:
  name: on-field-change
  description: demonstrates how to trigger a workflow when a field changes
  triggers:
    - type: alert
      only_on_change:
        - status
  actions:
    - name: echo-test
      provider:
        type: console
        with:
          message: "Hello world"`

	tmpDir, err := os.MkdirTemp("", "workflow_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpfilePath := filepath.Join(tmpDir, "change.yml")

	if err := os.WriteFile(tmpfilePath, []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWorkflowDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + "\n" +
					testAccWorkflowConfig(tmpfilePath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkflowExists("keep_workflow.test"),
					resource.TestCheckResourceAttr("keep_workflow.test", "name", "on-field-change"),
					resource.TestCheckResourceAttr("keep_workflow.test", "description", "demonstrates how to trigger a workflow when a field changes"),
				),
			},
			{
				ResourceName:            "keep_workflow.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"workflow_file_path", "workflow_content_hash"},
			},
		},
	})
}

func TestAccKeepWorkflow_Discord(t *testing.T) {
	workflowContent := `workflow:
  name: discord-example
  description: Discord example
  triggers:
    - type: manual
  actions:
    - name: discord
      provider:
        type: discord
        config: "{{ providers.discordtest }}"
        with:
          content: Alerta!
          components:
          - type: 1
            components:
              - type: 2
                style: 1
                label: "Click Me!"
                custom_id: "button_click"`

	tmpDir, err := os.MkdirTemp("", "workflow_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpfilePath := filepath.Join(tmpDir, "discord.yml")

	if err := os.WriteFile(tmpfilePath, []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWorkflowDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + "\n" +
					testAccWorkflowConfig(tmpfilePath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkflowExists("keep_workflow.test"),
					resource.TestCheckResourceAttr("keep_workflow.test", "name", "discord-example"),
					resource.TestCheckResourceAttr("keep_workflow.test", "description", "Discord example"),
				),
			},
			{
				ResourceName:            "keep_workflow.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"workflow_file_path", "workflow_content_hash"},
			},
		},
	})
}

func TestAccKeepWorkflow_Slack(t *testing.T) {
	workflowContent := `workflow:
  name: slack-basic-demo
  description: Send a slack message when a cloudwatch alarm is triggered
  triggers:
    - type: alert
      filters:
        - key: source
          value: cloudwatch
    - type: manual
  actions:
    - name: trigger-slack
      provider:
        type: slack
        config: " {{ providers.slack-prod }} "
        with:
          message: "Got alarm from aws cloudwatch! {{ alert.name }}"`

	tmpDir, err := os.MkdirTemp("", "workflow_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpfilePath := filepath.Join(tmpDir, "slack.yml")

	if err := os.WriteFile(tmpfilePath, []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWorkflowDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + "\n" +
					testAccWorkflowConfig(tmpfilePath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkflowExists("keep_workflow.test"),
					resource.TestCheckResourceAttr("keep_workflow.test", "name", "slack-basic-demo"),
					resource.TestCheckResourceAttr("keep_workflow.test", "description", "Send a slack message when a cloudwatch alarm is triggered"),
				),
			},
			{
				ResourceName:            "keep_workflow.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"workflow_file_path", "workflow_content_hash"},
			},
		},
	})
}

func testAccCheckWorkflowExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("workflow not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("workflow ID is not set")
		}

		client := testAccProvider.Meta().(*Client)
		workflow, errResp, err := client.GetWorkflow(rs.Primary.ID)
		if err != nil {
			if errResp != nil {
				return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
			}
			return fmt.Errorf("error getting workflow: %s", err)
		}

		if workflow == nil {
			return fmt.Errorf("workflow not found")
		}

		return nil
	}
}

func testAccCheckWorkflowDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "keep_workflow" {
			continue
		}

		workflow, errResp, err := client.GetWorkflow(rs.Primary.ID)
		if err == nil && workflow != nil {
			return fmt.Errorf("workflow still exists")
		}
		if err != nil && errResp != nil {
			// Ignore API errors during destroy check as the resource might be already gone
			continue
		}
	}

	return nil
}

func TestAccKeepWorkflow_ContentChange(t *testing.T) {
	workflowContent := `workflow:
  name: content-change-test
  description: Initial workflow content
  triggers:
    - type: manual
  actions:
    - name: echo-test
      provider:
        type: console
        with:
          message: "Initial message"`

	updatedContent := `workflow:
  name: content-change-test
  description: Updated workflow content
  triggers:
    - type: manual
  actions:
    - name: echo-test
      provider:
        type: console
        with:
          message: "Updated message"
    - name: new-action
      provider:
        type: console
        with:
          message: "New action added"`

	tmpDir, err := os.MkdirTemp("", "workflow_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpfilePath := filepath.Join(tmpDir, "content_change.yml")

	if err := os.WriteFile(tmpfilePath, []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWorkflowDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + "\n" +
					testAccWorkflowConfig(tmpfilePath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkflowExists("keep_workflow.test"),
					resource.TestCheckResourceAttr("keep_workflow.test", "name", "content-change-test"),
					resource.TestCheckResourceAttr("keep_workflow.test", "description", "Initial workflow content"),
				),
			},
			{
				PreConfig: func() {
					if err := os.WriteFile(tmpfilePath, []byte(updatedContent), 0644); err != nil {
						t.Fatal(err)
					}
				},
				Config: testAccProviderConfig(os.Getenv("KEEP_BACKEND_URL"), os.Getenv("KEEP_API_KEY")) + "\n" +
					testAccWorkflowConfig(tmpfilePath),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWorkflowExists("keep_workflow.test"),
					resource.TestCheckResourceAttr("keep_workflow.test", "name", "content-change-test"),
					resource.TestCheckResourceAttr("keep_workflow.test", "description", "Updated workflow content"),
				),
			},
		},
	})
}
