package keep

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func resourceWorkflow() *schema.Resource {
	hasher := &FileHasher{
		HashField:   "workflow_content_hash",
		Description: "Hash of the workflow file content for change detection",
	}

	schemaMap := map[string]*schema.Schema{
		"workflow_file_path": {
			Type:         schema.TypeString,
			Optional:     true,
			ExactlyOneOf: []string{"file", "workflow_file_path"},
			Description:  "Path of the workflow file (deprecated, use 'file' instead)",
		},
		"file": {
			Type:         schema.TypeString,
			Optional:     true,
			ExactlyOneOf: []string{"file", "workflow_file_path"},
			Description:  "Path of the workflow file",
		},
		"name": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"description": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"revision": {
			Type:     schema.TypeInt,
			Computed: true,
		},
	}

	// Add hash field to schema
	hasher.AddHashFieldToSchema(schemaMap)

	return &schema.Resource{
		CreateContext: resourceCreateWorkflow,
		ReadContext:   resourceReadWorkflow,
		UpdateContext: resourceUpdateWorkflow,
		DeleteContext: resourceDeleteWorkflow,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
			workflowFilePath := getWorkflowFilePath(d)
			hasher.FilePath = workflowFilePath
			return hasher.CustomizeDiff(ctx, d)
		},
		Schema: schemaMap,
	}
}

func validateWorkflowFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read workflow file: %s", err)
	}

	var workflowWrapper struct {
		Workflow struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
			Actions     []struct {
				Name     string `yaml:"name"`
				Provider struct {
					Type string `yaml:"type"`
				} `yaml:"provider"`
			} `yaml:"actions"`
		} `yaml:"workflow"`
	}

	if err := yaml.Unmarshal(content, &workflowWrapper); err != nil {
		return fmt.Errorf("invalid workflow YAML: %s", err)
	}

	if workflowWrapper.Workflow.Name == "" {
		return fmt.Errorf("workflow name is required")
	}

	return nil
}

func getWorkflowFilePath(d interface{}) string {
	var getter interface {
		GetOk(string) (interface{}, bool)
		Get(string) interface{}
	}

	switch v := d.(type) {
	case *schema.ResourceData:
		getter = v
	case *schema.ResourceDiff:
		getter = v
	default:
		return ""
	}

	if v, ok := getter.GetOk("file"); ok {
		return v.(string)
	}
	return getter.Get("workflow_file_path").(string)
}

func resourceCreateWorkflow(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)
	workflowFilePath := getWorkflowFilePath(d)
	if workflowFilePath == "" {
		return diag.Errorf("either file or workflow_file_path is required for creation")
	}

	hasher := &FileHasher{
		FilePath:  workflowFilePath,
		HashField: "workflow_content_hash",
	}
	if err := hasher.SetFileHash(d); err != nil {
		return diag.FromErr(err)
	}

	content, err := os.ReadFile(workflowFilePath)
	if err != nil {
		return diag.FromErr(err)
	}

	var workflowWrapper map[string]interface{}
	if err := yaml.Unmarshal(content, &workflowWrapper); err != nil {
		return diag.Errorf("invalid workflow YAML: %s", err)
	}

	// Validate workflow name
	if workflow, ok := workflowWrapper["workflow"].(map[interface{}]interface{}); ok {
		if name, ok := workflow["name"].(string); !ok || name == "" {
			return diag.Errorf("workflow name is required")
		}
	} else {
		return diag.Errorf("invalid workflow structure")
	}

	workflowData, err := yamlToJSONMap(content)
	if err != nil {
		return diag.Errorf("invalid workflow YAML: %s", err)
	}

	response, errResp, err := client.CreateWorkflowJSON(workflowData)
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error creating workflow: %s", err)
	}

	if id, ok := response["workflow_id"].(string); ok && id != "" {
		d.SetId(id)
		if workflow, ok := workflowWrapper["workflow"].(map[interface{}]interface{}); ok {
			if name, ok := workflow["name"].(string); ok {
				d.Set("name", name)
			}
			if desc, ok := workflow["description"].(string); ok {
				d.Set("description", desc)
			}
		}
		if revision, ok := response["revision"].(float64); ok {
			d.Set("revision", int(revision))
		}
		return resourceReadWorkflow(ctx, d, m)
	}
	return diag.Errorf("workflow ID not found in response")
}

func resourceDeleteWorkflow(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)

	errResp, err := client.DeleteWorkflow(d.Id())
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error deleting workflow: %s", err)
	}

	return nil
}

func resourceUpdateWorkflow(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)
	workflowFilePath := getWorkflowFilePath(d)

	hasher := &FileHasher{
		FilePath:  workflowFilePath,
		HashField: "workflow_content_hash",
	}
	if err := hasher.SetFileHash(d); err != nil {
		return diag.FromErr(err)
	}

	content, err := os.ReadFile(workflowFilePath)
	if err != nil {
		return diag.FromErr(err)
	}

	var workflowWrapper map[string]interface{}
	if err := yaml.Unmarshal(content, &workflowWrapper); err != nil {
		return diag.Errorf("invalid workflow YAML: %s", err)
	}

	// Validate workflow name
	if workflow, ok := workflowWrapper["workflow"].(map[interface{}]interface{}); ok {
		if name, ok := workflow["name"].(string); !ok || name == "" {
			return diag.Errorf("workflow name is required")
		}
	} else {
		return diag.Errorf("invalid workflow structure")
	}

	workflowData, err := yamlToJSONMap(content)
	if err != nil {
		return diag.Errorf("invalid workflow YAML: %s", err)
	}

	response, errResp, err := client.CreateWorkflowJSON(workflowData)
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error updating workflow: %s", err)
	}

	if id, ok := response["workflow_id"].(string); ok && id != "" {
		d.SetId(id)
		if workflow, ok := workflowWrapper["workflow"].(map[interface{}]interface{}); ok {
			if name, ok := workflow["name"].(string); ok {
				d.Set("name", name)
			}
			if desc, ok := workflow["description"].(string); ok {
				d.Set("description", desc)
			}
		}
		if revision, ok := response["revision"].(float64); ok {
			d.Set("revision", int(revision))
		}
		return resourceReadWorkflow(ctx, d, m)
	}
	return diag.Errorf("workflow ID not found in response")
}

func resourceReadWorkflow(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)

	response, errResp, err := client.GetWorkflow(d.Id())
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		d.SetId("")
		return nil
	}

	if id, ok := response["id"].(string); ok && id != "" {
		d.SetId(id)
		if raw, ok := response["workflow_raw"].(string); ok && raw != "" {
			var workflowWrapper struct {
				Workflow struct {
					Name        string `yaml:"name"`
					Description string `yaml:"description"`
					Actions     []struct {
						Name     string `yaml:"name"`
						Provider struct {
							Type string `yaml:"type"`
						} `yaml:"provider"`
					} `yaml:"actions"`
				} `yaml:"workflow"`
			}
			if err := yaml.Unmarshal([]byte(raw), &workflowWrapper); err == nil {
				d.Set("name", workflowWrapper.Workflow.Name)
				d.Set("description", workflowWrapper.Workflow.Description)
			}
		}
		if revision, ok := response["revision"].(float64); ok {
			d.Set("revision", int(revision))
		}
		return nil
	}

	d.SetId("")
	return nil
}
