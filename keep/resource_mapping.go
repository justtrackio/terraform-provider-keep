package keep

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/spf13/cast"
)

// validateMatchersAgainstCSV validates that all matcher columns exist in the CSV data
func validateMatchersAgainstCSV(matchers []string, csvRows []map[string]string) error {
	if len(csvRows) == 0 {
		return fmt.Errorf("CSV file is empty")
	}

	// Get all available columns from the first row and create a map for quick lookup
	availableColumns := make(map[string]bool)
	for column := range csvRows[0] {
		availableColumns[column] = true
	}

	// Check each matcher against available columns
	for _, matcher := range matchers {
		// Split matcher into parts (assuming format like "source && labels.priority")
		parts := strings.Split(matcher, " && ")
		for _, part := range parts {
			// Extract column name by splitting on operators (=~, =, !=, etc.)
			columnName := strings.Split(strings.TrimSpace(part), "=")[0]
			columnName = strings.Split(columnName, "!")[0] // Handle != operator
			columnName = strings.Split(columnName, "~")[0] // Handle =~ operator
			columnName = strings.TrimSpace(columnName)

			// Check if the exact column name exists
			if !availableColumns[columnName] {
				// Get sorted column names for better error message readability
				availableKeys := getKeysFromMap(availableColumns)
				sort.Strings(availableKeys)
				return fmt.Errorf("matcher '%s' references column '%s' which is not present in the CSV file. Available columns: %v",
					matcher, columnName, availableKeys)
			}
		}
	}
	return nil
}

// getKeysFromMap extracts and returns all keys from a map
func getKeysFromMap(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// formatMatchers converts matcher strings to arrays as required by the API
func formatMatchers(matcherStrings []string) [][]string {
	formatted := make([][]string, len(matcherStrings))
	for i, matcher := range matcherStrings {
		parts := strings.Split(matcher, " && ")
		formatted[i] = parts
	}
	return formatted
}

// formatMatchersStringForState converts matcher arrays back to strings for state
func formatMatchersStringForState(matcherArrays interface{}) []string {
	switch v := matcherArrays.(type) {
	case []string:
		return v
	case []interface{}:
		formatted := make([]string, len(v))
		for i, matcher := range v {
			switch m := matcher.(type) {
			case []interface{}:
				parts := make([]string, len(m))
				for j, part := range m {
					if str, ok := part.(string); ok {
						parts[j] = str
					}
				}
				formatted[i] = strings.Join(parts, " && ")
			case string:
				formatted[i] = m
			default:
				formatted[i] = ""
			}
		}
		return formatted
	default:
		return []string{}
	}
}

func resourceMapping() *schema.Resource {
	hasher := &FileHasher{
		HashField:   "csv_content_hash",
		Description: "Hash of the CSV file content for change detection",
	}

	return &schema.Resource{
		CreateContext: resourceCreateMapping,
		ReadContext:   resourceReadMapping,
		UpdateContext: resourceUpdateMapping,
		DeleteContext: resourceDeleteMapping,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
				return []*schema.ResourceData{d}, nil
			},
		},
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
			mappingFilePath := filepath.Clean(d.Get("mapping_file_path").(string))
			hasher.FilePath = mappingFilePath
			return hasher.CustomizeDiff(ctx, d)
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the mapping",
			},

			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the mapping",
			},
			"matchers": {
				Type:        schema.TypeSet,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Set:         schema.HashString,
				Description: "List of matchers",
			},
			"priority": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Priority of the mapping",
				Default:     0,
			},
			"mapping_file_path": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Path of the mapping file",
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					// Get the base filename from both paths
					oldBase := filepath.Base(old)
					newBase := filepath.Base(new)
					return oldBase == newBase
				},
			},
			"csv_content_hash": {
				Type:        schema.TypeString,
				Computed:    true,
				ForceNew:    true,
				Description: "Hash of the CSV file content for change detection",
			},
		},
	}
}

// Add function to check for duplicate names
func checkDuplicateName(client *Client, name string, currentID string) error {
	mappings, errResp, err := client.GetMappings()
	if err != nil {
		if errResp != nil {
			return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return fmt.Errorf("error getting mappings: %s", err)
	}

	for _, m := range mappings {
		mapping := m.(map[string]interface{})
		if mapping["name"] == name {
			if id := cast.ToString(mapping["id"]); id != currentID {
				return fmt.Errorf("mapping with name '%s' already exists", name)
			}
		}
	}

	return nil
}

// Add helper function to clean up duplicate mappings
func cleanupDuplicateMappings(client *Client, currentID, name string) error {
	mappings, errResp, err := client.GetMappings()
	if err != nil {
		if errResp != nil {
			return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return fmt.Errorf("error getting mappings: %s", err)
	}

	for _, m := range mappings {
		mapping := m.(map[string]interface{})
		if mapping["name"] == name {
			if id := cast.ToString(mapping["id"]); id != currentID {
				errResp, err := client.DeleteMapping(id)
				if err != nil {
					if errResp != nil {
						return fmt.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
					}
					return fmt.Errorf("error deleting mapping %s: %s", id, err)
				}
			}
		}
	}

	return nil
}

func resourceCreateMapping(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)
	name := d.Get("name").(string)

	// Check for duplicate names before creating
	if err := checkDuplicateName(client, name, ""); err != nil {
		return diag.FromErr(err)
	}

	mappingFilePath := d.Get("mapping_file_path").(string)
	normalizedPath := filepath.Clean(mappingFilePath)
	d.Set("mapping_file_path", normalizedPath)

	// read file from mappingFilePath it should be a file path and csv file

	fInfo, err := os.Stat(normalizedPath)
	if err != nil {
		return diag.Errorf("mapping file not found: %s", mappingFilePath)
	} else if fInfo.IsDir() {
		return diag.Errorf("mapping file is a directory: %s", mappingFilePath)
	}

	file, err := os.OpenFile(normalizedPath, os.O_RDONLY, 0644)
	if err != nil {
		return diag.Errorf("cannot open file: %s", mappingFilePath)
	}
	defer file.Close()

	hasher := &FileHasher{
		FilePath:  normalizedPath,
		HashField: "csv_content_hash",
	}
	if err := hasher.SetFileHash(d); err != nil {
		return diag.FromErr(err)
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()

	if err != nil {
		return diag.Errorf("Error reading CSV file: %s", err)
	}

	headers := records[0]
	records = records[1:]

	rows := make([]map[string]string, len(records))
	for i, record := range records {
		row := make(map[string]string)
		for j, cell := range record {
			row[headers[j]] = cell
		}
		rows[i] = row
	}

	matchersSet := d.Get("matchers").(*schema.Set)
	matcherStrings := make([]string, len(matchersSet.List()))
	for i, matcher := range matchersSet.List() {
		matcherStrings[i] = matcher.(string)
	}

	// Validate matchers against CSV content
	if err := validateMatchersAgainstCSV(matcherStrings, rows); err != nil {
		return diag.Errorf("Invalid matchers: %s", err)
	}

	// Format matchers as arrays for the API
	formattedMatchers := formatMatchers(matcherStrings)

	body := map[string]interface{}{
		"name":        d.Get("name").(string),
		"description": d.Get("description").(string),
		"matchers":    formattedMatchers,
		"priority":    d.Get("priority").(int),
		"rows":        rows,
		"file_name":   fInfo.Name(),
	}

	response, errResp, err := client.CreateMapping(body)
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error creating mapping: %s", err)
	}

	// Get the hash value and set composite ID
	contentHash := d.Get("csv_content_hash").(string)
	compositeID := fmt.Sprintf("%v:%s", response["id"], contentHash)
	d.SetId(compositeID)

	d.Set("name", response["name"])
	d.Set("description", response["description"])
	d.Set("priority", response["priority"])

	// Convert matcher arrays back to strings for state if needed
	if matcherArrays, ok := response["matchers"].([]interface{}); ok {
		d.Set("matchers", formatMatchersStringForState(matcherArrays))
	} else {
		d.Set("matchers", matcherStrings)
	}

	// After successful creation, clean up any duplicates
	if err := cleanupDuplicateMappings(client, fmt.Sprintf("%v", response["id"]), response["name"].(string)); err != nil {
		return diag.FromErr(err)
	}

	return nil

}

func resourceReadMapping(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)
	id := d.Id()

	// Handle both composite and simple IDs
	var mappingID string
	if strings.Contains(id, ":") {
		parts := strings.Split(id, ":")
		if len(parts) != 2 {
			return diag.Errorf("invalid resource ID format")
		}
		mappingID = parts[0]
	} else {
		mappingID = id
	}

	mappings, errResp, err := client.GetMappings()
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error getting mappings: %s", err)
	}

	idInt := cast.ToInt(mappingID)
	for _, m := range mappings {
		mapping := m.(map[string]interface{})
		if cast.ToInt(mapping["id"]) == idInt {
			currentDir, _ := os.Getwd()
			filePath := filepath.Join(currentDir, mapping["file_name"].(string))

			// Only set csv_content_hash if we have access to the file
			if path := d.Get("mapping_file_path").(string); path != "" {
				if hash, err := calculateFileHash(path); err == nil {
					d.Set("csv_content_hash", hash)
				}
			}

			d.Set("name", mapping["name"])
			d.Set("description", mapping["description"])
			d.Set("priority", mapping["priority"])
			d.Set("mapping_file_path", filePath)

			// Handle matchers conversion
			var matcherSet *schema.Set
			if matchers, ok := mapping["matchers"].([]interface{}); ok {
				matcherStrings := make([]interface{}, len(matchers))
				for i, matcher := range matchers {
					switch m := matcher.(type) {
					case []interface{}:
						parts := make([]string, len(m))
						for j, part := range m {
							if str, ok := part.(string); ok {
								parts[j] = str
							}
						}
						matcherStrings[i] = strings.Join(parts, " && ")
					case string:
						matcherStrings[i] = m
					}
				}
				matcherSet = schema.NewSet(schema.HashString, matcherStrings)
				d.Set("matchers", matcherSet)
			}

			return nil
		}
	}

	// If we reach here, the resource was not found
	d.SetId("")
	return nil
}

func resourceUpdateMapping(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)
	id := d.Id()

	// Only check for duplicates if name is being changed
	if d.HasChange("name") {
		name := d.Get("name").(string)
		if err := checkDuplicateName(client, name, id); err != nil {
			return diag.FromErr(err)
		}
	}

	// Extract mapping ID from composite ID if present
	var mappingID string
	if strings.Contains(id, ":") {
		parts := strings.Split(id, ":")
		if len(parts) != 2 {
			return diag.Errorf("invalid resource ID format")
		}
		mappingID = parts[0]
	} else {
		mappingID = id
	}

	// If this is a ForceNew update (CSV content changed), ensure old mapping is deleted
	if d.HasChange("csv_content_hash") {
		ruleID, err := strconv.Atoi(mappingID)
		if err != nil {
			return diag.Errorf("invalid rule ID format: %s", err)
		}

		// Delete the old mapping
		deleteReq, err := http.NewRequest("DELETE", fmt.Sprintf("%s/mapping/%d", client.HostURL, ruleID), nil)
		if err != nil {
			return diag.Errorf("cannot create delete request: %s", err)
		}

		_, errResp, err := client.doReq(deleteReq)
		if err != nil {
			if errResp != nil {
				return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
			}
			return diag.Errorf("error deleting resource: %s", err)
		}
	}

	mappingFilePath := d.Get("mapping_file_path").(string)
	normalizedPath := filepath.Clean(mappingFilePath)

	// Rest of the update logic
	fInfo, err := os.Stat(normalizedPath)
	if err != nil {
		return diag.Errorf("mapping file not found: %s", mappingFilePath)
	} else if fInfo.IsDir() {
		return diag.Errorf("mapping file is a directory: %s", mappingFilePath)
	}

	file, err := os.OpenFile(normalizedPath, os.O_RDONLY, 0644)
	if err != nil {
		return diag.Errorf("cannot open file: %s", mappingFilePath)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return diag.Errorf("Error reading CSV file: %s", err)
	}

	headers := records[0]
	records = records[1:]

	rows := make([]map[string]string, len(records))
	for i, record := range records {
		row := make(map[string]string)
		for j, cell := range record {
			row[headers[j]] = cell
		}
		rows[i] = row
	}

	matchersSet := d.Get("matchers").(*schema.Set)
	matcherStrings := make([]string, len(matchersSet.List()))
	for i, matcher := range matchersSet.List() {
		matcherStrings[i] = matcher.(string)
	}

	// Validate matchers against CSV content
	if err := validateMatchersAgainstCSV(matcherStrings, rows); err != nil {
		return diag.Errorf("Invalid matchers: %s", err)
	}

	// Format matchers as arrays for the API
	formattedMatchers := formatMatchers(matcherStrings)

	reqBody := map[string]interface{}{
		"name":        d.Get("name").(string),
		"description": d.Get("description").(string),
		"matchers":    formattedMatchers,
		"priority":    d.Get("priority").(int),
		"rows":        rows,
		"file_name":   fInfo.Name(),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return diag.Errorf("cannot marshal request body: %s", err)
	}

	updateReq, err := http.NewRequest("POST", client.HostURL+"/mapping", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return diag.Errorf("cannot create request: %s", err)
	}

	respBody, errResp, err := client.doReq(updateReq)
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("cannot send request: %s", err)
	}

	var mappingResponse struct {
		ID          int      `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Priority    int      `json:"priority"`
		Matchers    []string `json:"matchers"`
	}

	err = json.Unmarshal(respBody, &mappingResponse)
	if err != nil {
		return diag.Errorf("cannot unmarshal response: %s", err)
	}

	hasher := &FileHasher{
		FilePath:  normalizedPath,
		HashField: "csv_content_hash",
	}
	if err := hasher.SetFileHash(d); err != nil {
		return diag.FromErr(err)
	}

	// Get the hash value after setting it
	contentHash := d.Get("csv_content_hash").(string)
	compositeID := fmt.Sprintf("%d:%s", mappingResponse.ID, contentHash)
	d.SetId(compositeID)
	d.Set("csv_content_hash", contentHash)
	d.Set("name", mappingResponse.Name)
	d.Set("description", mappingResponse.Description)
	d.Set("priority", mappingResponse.Priority)

	// Convert matcher arrays back to strings for state
	d.Set("matchers", formatMatchersStringForState(mappingResponse.Matchers))

	// After successful update, clean up any duplicates
	if err := cleanupDuplicateMappings(client, cast.ToString(mappingResponse.ID), mappingResponse.Name); err != nil {
		return diag.FromErr(err)
	}

	return nil

}

func resourceDeleteMapping(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*Client)
	id := d.Id()

	// Extract mapping ID from composite ID if present
	var mappingID string
	if strings.Contains(id, ":") {
		parts := strings.Split(id, ":")
		if len(parts) != 2 {
			return diag.Errorf("invalid resource ID format")
		}
		mappingID = parts[0]
	} else {
		mappingID = id
	}

	// Convert ID to integer to ensure valid format
	errResp, err := client.DeleteMapping(mappingID)
	if err != nil {
		if errResp != nil {
			return diag.Errorf("API Error: %s. Details: %s", errResp.Error, errResp.Details)
		}
		return diag.Errorf("error deleting mapping: %s", err)
	}

	return nil
}
