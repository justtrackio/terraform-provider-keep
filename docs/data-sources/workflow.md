---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "keep_workflow Data Source - terraform-provider-keep"
subcategory: ""
description: |-

---

# keep_workflow (Data Source)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) The ID of the workflow.

### Read-Only

- `created_by` (String) The user who created the workflow.
- `creation_time` (String) The time when the workflow was created.
- `description` (String) The description of the workflow.
- `interval` (Number) The interval of the workflow.
- `invalid` (Boolean) The invalid status of the workflow.
- `keep_providers` (String) The providers of the workflow.
- `last_execution_status` (String) The status of the last execution of the workflow.
- `last_execution_time` (String) The time when the workflow was last executed.
- `last_updated` (String) The time when the workflow was last updated.
- `name` (String) The name of the workflow.
- `revision` (Number) The revision of the workflow.
- `triggers` (String) The triggers of the workflow.
- `workflow_raw` (String) The raw workflow.
- `workflow_raw_id` (String) The ID of the raw workflow.
