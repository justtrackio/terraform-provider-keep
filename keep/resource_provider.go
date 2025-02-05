package keep

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceProvider() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCreateProvider,
		ReadContext:   resourceReadProvider,
		UpdateContext: resourceUpdateProvider,
		DeleteContext: resourceDeleteProvider,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Type of the keep provider",
				ForceNew:    true,
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the keep provider",
			},
			"auth_config": {
				Type:        schema.TypeMap,
				Required:    true,
				Sensitive:   true,
				Description: "Configuration of the keep provider authentication",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"install_webhook": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Install webhook for the provider (default: false)",
			},
		},
	}
}

func resourceCreateProvider(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(KeepClient)
	providerType := d.Get("type").(string)
	providerName := d.Get("name").(string)
	authConfig := d.Get("auth_config").(map[string]interface{})

	// First validate if the provider type exists
	providers, errResp, err := client.GetAvailableProviders()
	if err != nil {
		if errResp != nil {
			return diag.Errorf("Failed to get available providers: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("Failed to get available providers: %s", err.Error())
	}

	found := false
	availableTypes := make([]string, 0)
	for _, provider := range providers {
		if p, ok := provider.(map[string]interface{}); ok {
			if pType, exists := p["type"].(string); exists {
				availableTypes = append(availableTypes, pType)
				if pType == providerType {
					found = true
					break
				}
			}
		}
	}

	if !found {
		return diag.Errorf("Provider type '%s' not found. Available provider types: %v", providerType, availableTypes)
	}

	// Prepare installation payload
	installPayload := map[string]interface{}{
		"provider_id":   providerType,
		"provider_name": providerName,
	}
	for k, v := range authConfig {
		installPayload[k] = v
	}

	// Install provider
	response, errResp, err := client.InstallProvider(installPayload)
	if err != nil {
		if errResp != nil {
			if strings.Contains(errResp.Details, "Missing required scopes") {
				return diag.Errorf("Failed to install provider: insufficient permissions. %s", errResp.Details)
			}
			return diag.Errorf("Failed to install provider: %s. Details: %s. Payload: %v", errResp.Error, errResp.Details, installPayload)
		}
		return diag.Errorf("Failed to install provider: %s. Payload: %v", err.Error(), installPayload)
	}

	if response == nil {
		return diag.Errorf("Provider installation failed: received empty response. Payload: %v", installPayload)
	}

	if response["id"] == nil {
		return diag.Errorf("Provider installation failed: no ID returned in response. Response: %v, Payload: %v", response, installPayload)
	}

	id := response["id"].(string)
	d.SetId(id)

	// Install webhook if requested
	if d.Get("install_webhook").(bool) {
		errResp, err := client.InstallProviderWebhook(providerType, id)
		if err != nil {
			if errResp != nil {
				if strings.Contains(errResp.Details, "Missing required scopes") {
					return diag.Errorf("Failed to install webhook: insufficient permissions. %s", errResp.Details)
				}
				return diag.Errorf("Failed to install webhook: %s. Details: %s", errResp.Error, errResp.Details)
			}
			return diag.Errorf("Failed to install webhook: %s", err.Error())
		}
	}

	return resourceReadProvider(ctx, d, m)
}

func resourceDeleteProvider(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)

	id := d.Id()
	providerType := d.Get("type").(string)

	errResp, err := client.DeleteProvider(providerType, id)
	if err != nil {
		if errResp != nil {
			if strings.Contains(errResp.Details, "Missing required scopes") {
				return diag.Errorf("Failed to delete provider: insufficient permissions. %s", errResp.Details)
			}
			return diag.Errorf("Failed to delete provider: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("Failed to delete provider: %s", err.Error())
	}

	return nil
}

func resourceReadProvider(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(KeepClient)
	id := d.Id()

	providers, errResp, err := client.GetInstalledProviders()
	if err != nil {
		if errResp != nil {
			if strings.Contains(errResp.Details, "Missing required scopes") {
				return diag.Errorf("Failed to get installed providers: insufficient permissions. %s", errResp.Details)
			}
			return diag.Errorf("Failed to get installed providers: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("Failed to get installed providers: %s", err.Error())
	}

	for _, provider := range providers {
		p := provider.(map[string]interface{})
		if p["id"] == id {
			if err := d.Set("type", p["type"]); err != nil {
				return diag.Errorf("Failed to set type: %s", err.Error())
			}

			if details, ok := p["details"].(map[string]interface{}); ok {
				if name, exists := details["name"].(string); exists {
					if err := d.Set("name", name); err != nil {
						return diag.Errorf("Failed to set name: %s", err.Error())
					}
				}

				if auth, exists := details["authentication"].(map[string]interface{}); exists {
					authConfig := make(map[string]interface{})
					for key, value := range auth {
						authConfig[key] = value
					}
					if err := d.Set("auth_config", authConfig); err != nil {
						return diag.Errorf("Failed to set auth_config: %s", err.Error())
					}
				}
			}

			return nil
		}
	}

	d.SetId("")
	return nil
}

func resourceUpdateProvider(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(KeepClient)
	id := d.Id()
	providerType := d.Get("type").(string)

	if d.HasChanges("name", "auth_config", "install_webhook") {
		// Since updates are not supported, we need to delete and recreate
		// First delete the existing provider
		errResp, err := client.DeleteProvider(providerType, id)
		if err != nil {
			if errResp != nil {
				return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
			}
			return diag.FromErr(err)
		}

		// Then create a new one with updated configuration
		createPayload := map[string]interface{}{
			"provider_id":   providerType,
			"provider_name": d.Get("name").(string),
		}

		// Add auth config
		for k, v := range d.Get("auth_config").(map[string]interface{}) {
			createPayload[k] = v
		}

		// Create new provider
		response, errResp, err := client.InstallProvider(createPayload)
		if err != nil {
			if errResp != nil {
				if strings.Contains(errResp.Details, "Missing required scopes") {
					return diag.Errorf("Failed to install provider: insufficient permissions. %s", errResp.Details)
				}
				return diag.Errorf("Failed to install provider: %s. Details: %s. Payload: %v", errResp.Error, errResp.Details, createPayload)
			}
			return diag.Errorf("Failed to install provider: %s. Payload: %v", err.Error(), createPayload)
		}

		if response == nil {
			return diag.Errorf("Provider installation failed: received empty response. Payload: %v", createPayload)
		}

		if response["id"] == nil {
			return diag.Errorf("Provider installation failed: no ID returned in response. Response: %v, Payload: %v", response, createPayload)
		}

		// Set new ID
		newID := response["id"].(string)
		d.SetId(newID)

		// Handle webhook if needed
		if d.Get("install_webhook").(bool) {
			errResp, err := client.InstallProviderWebhook(providerType, newID)
			if err != nil {
				if errResp != nil {
					if strings.Contains(errResp.Details, "Missing required scopes") {
						return diag.Errorf("Failed to install webhook: insufficient permissions. %s", errResp.Details)
					}
					return diag.Errorf("Failed to install webhook: %s. Details: %s", errResp.Error, errResp.Details)
				}
				return diag.Errorf("Failed to install webhook: %s", err.Error())
			}
		}

	}

	return resourceReadProvider(ctx, d, m)
}
