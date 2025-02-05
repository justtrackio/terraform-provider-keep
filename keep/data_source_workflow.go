package keep

import (
	"context"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceWorkflows() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceReadWorkflow,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the workflow.",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The name of the workflow.",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The description of the workflow.",
			},
			"created_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The user who created the workflow.",
			},
			"creation_time": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The time when the workflow was created.",
			},
			"triggers": {
				Type: schema.TypeString,
				//Elem:        false,
				Computed:    true,
				Description: "The triggers of the workflow.",
			},
			"interval": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The interval of the workflow.",
			},
			"last_execution_time": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The time when the workflow was last executed.",
			},
			"last_execution_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The status of the last execution of the workflow.",
			},
			"keep_providers": {
				Type: schema.TypeString,
				//Elem:        false,
				Computed:    true,
				Description: "The providers of the workflow.",
			},
			"workflow_raw_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID of the raw workflow.",
			},
			"workflow_raw": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The raw workflow.",
			},
			"revision": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The revision of the workflow.",
			},
			"last_updated": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The time when the workflow was last updated.",
			},
			"invalid": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "The invalid status of the workflow.",
			},
		},
	}
}

func dataSourceReadWorkflow(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)
	id := d.Get("id").(string)

	response, errResp, err := client.GetWorkflow(id)
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error reading workflow: %s", err)
	}

	d.SetId(id)
	d.Set("name", response["name"])
	d.Set("description", response["description"])
	d.Set("created_by", response["created_by"])
	d.Set("creation_time", response["creation_time"])
	d.Set("triggers", response["triggers"])
	d.Set("interval", response["interval"])
	d.Set("last_execution_time", response["last_execution_time"])
	d.Set("last_execution_status", response["last_execution_status"])
	d.Set("keep_providers", response["providers"])
	d.Set("workflow_raw_id", response["workflow_raw_id"])
	d.Set("workflow_raw", response["workflow_raw"])
	d.Set("revision", response["revision"])
	d.Set("last_updated", response["last_updated"])
	d.Set("invalid", response["invalid"])

	return nil
}
