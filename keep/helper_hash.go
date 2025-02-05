package keep

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// FileHasher provides functionality for file content hash checking
type FileHasher struct {
	FilePath    string
	HashField   string
	Description string
}

// calculateFileHash calculates SHA256 hash of file content
func calculateFileHash(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("cannot read file: %s", err)
	}

	h := sha256.New()
	if _, err := h.Write(content); err != nil {
		return "", fmt.Errorf("cannot calculate hash: %s", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// AddHashFieldToSchema adds a content hash field to a schema
func (h *FileHasher) AddHashFieldToSchema(s map[string]*schema.Schema) {
	s[h.HashField] = &schema.Schema{
		Type:        schema.TypeString,
		Computed:    true,
		ForceNew:    true,
		Description: h.Description,
	}
}

// CustomizeDiff adds hash checking to resource diff
func (h *FileHasher) CustomizeDiff(ctx interface{}, d *schema.ResourceDiff) error {
	if h.FilePath == "" {
		return nil
	}

	hash, err := calculateFileHash(h.FilePath)
	if err != nil {
		return fmt.Errorf("cannot calculate file hash: %s", err)
	}

	oldHash := d.Get(h.HashField).(string)
	if oldHash != hash {
		d.ForceNew(h.HashField)
		d.SetNew(h.HashField, hash)
	}

	return nil
}

// SetFileHash calculates and sets the file hash in ResourceData
func (h *FileHasher) SetFileHash(d *schema.ResourceData) error {
	hash, err := calculateFileHash(h.FilePath)
	if err != nil {
		return fmt.Errorf("cannot calculate file hash: %s", err)
	}
	return d.Set(h.HashField, hash)
}
