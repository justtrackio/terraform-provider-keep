package keep

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceExtraction() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCreateExtraction,
		ReadContext:   resourceReadExtraction,
		UpdateContext: resourceUpdateExtraction,
		DeleteContext: resourceDeleteExtraction,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ID of the extraction",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the extraction",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the extraction",
				Default:     "",
			},
			"priority": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Priority of the extraction",
				Default:     0,
			},
			"attribute": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Attribute of the extraction",
			},
			"condition": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Condition of the extraction",
				Default:     "",
			},
			"disabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"regex": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Regex of the extraction",
			},
			"pre": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Pre of the extraction",
			},
		},
	}
}

func resourceCreateExtraction(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)

	extraction := map[string]interface{}{
		"name":        d.Get("name").(string),
		"description": d.Get("description").(string),
		"priority":    d.Get("priority").(int),
		"attribute":   d.Get("attribute").(string),
		"condition":   d.Get("condition").(string),
		"disabled":    d.Get("disabled").(bool),
		"regex":       d.Get("regex").(string),
		"pre":         d.Get("pre").(bool),
	}

	response, errResp, err := client.CreateExtraction(extraction)
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error creating extraction: %s", err)
	}

	if id, ok := response["id"]; ok {
		d.SetId(fmt.Sprintf("%v", id))
	} else {
		return diag.Errorf("no id found in response")
	}

	return resourceReadExtraction(ctx, d, m)
}

func resourceReadExtraction(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)

	extractions, errResp, err := client.GetExtractions()
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error reading extractions: %s", err)
	}

	id := d.Id()
	var extraction map[string]interface{}
	for _, e := range extractions {
		ext := e.(map[string]interface{})
		if fmt.Sprintf("%v", ext["id"]) == id {
			extraction = ext
			break
		}
	}

	if extraction == nil {
		d.SetId("")
		return nil
	}

	d.Set("name", extraction["name"])
	d.Set("description", extraction["description"])
	d.Set("priority", extraction["priority"])
	d.Set("attribute", extraction["attribute"])
	d.Set("condition", extraction["condition"])
	d.Set("disabled", extraction["disabled"])
	d.Set("regex", extraction["regex"])
	d.Set("pre", extraction["pre"])

	return nil
}

func resourceUpdateExtraction(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)

	extraction := map[string]interface{}{
		"name":        d.Get("name").(string),
		"description": d.Get("description").(string),
		"priority":    d.Get("priority").(int),
		"attribute":   d.Get("attribute").(string),
		"condition":   d.Get("condition").(string),
		"disabled":    d.Get("disabled").(bool),
		"regex":       d.Get("regex").(string),
		"pre":         d.Get("pre").(bool),
	}

	errResp, err := client.UpdateExtraction(d.Id(), extraction)
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error updating extraction: %s", err)
	}

	return resourceReadExtraction(ctx, d, m)
}

func resourceDeleteExtraction(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)

	// First verify the extraction exists
	extractions, errResp, err := client.GetExtractions()
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error reading extractions: %s", err)
	}

	id := d.Id()
	exists := false
	for _, e := range extractions {
		ext := e.(map[string]interface{})
		if fmt.Sprintf("%v", ext["id"]) == id {
			exists = true
			break
		}
	}

	if !exists {
		d.SetId("")
		return nil
	}

	errResp, err = client.DeleteExtraction(id)
	if err != nil {
		// If we get a 405, the API might not support DELETE
		// In this case, we'll just remove it from state
		if strings.Contains(err.Error(), "405") {
			d.SetId("")
			return nil
		}
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error deleting extraction: %s", err)
	}

	d.SetId("")
	return nil
}
