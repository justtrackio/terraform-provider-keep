package keep

import (
	"context"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"strconv"
)

type Mapping struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	FileName    string   `json:"file_name"`
	Matchers    []string `json:"matchers"`
	Attributes  []string `json:"attributes"`
	CreatedAt   string   `json:"created_at"`
	CreatedBy   string   `json:"created_by"`
}

func dataSourceMapping() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceReadMapping,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the mapping",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the mapping",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Description of the mapping",
			},
			"file_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the mapping file",
			},
			"matchers": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of matchers",
			},
			"attributes": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of attributes",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Creation time of the mapping",
			},
			"created_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Creator of the mapping",
			},
		},
	}
}

func dataSourceReadMapping(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)
	id := d.Get("id").(int)

	mappings, errResp, err := client.GetMappings()
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error reading mappings: %s", err)
	}

	for _, m := range mappings {
		mapping := m.(map[string]interface{})
		if int(mapping["id"].(float64)) == id {
			d.SetId(strconv.Itoa(id))
			d.Set("id", strconv.Itoa(id))
			d.Set("name", mapping["name"])
			d.Set("description", mapping["description"])
			d.Set("file_name", mapping["file_name"])
			d.Set("matchers", mapping["matchers"])
			d.Set("attributes", mapping["attributes"])
			d.Set("created_at", mapping["created_at"])
			d.Set("created_by", mapping["created_by"])
			return nil
		}
	}

	return diag.Errorf("mapping with ID %d not found", id)
}
