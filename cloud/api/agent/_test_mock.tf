// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "agent" {
  content = <<-EOT
package agent // import "github.com/terramate-io/terramate/cloud/api/agent"

Package agent provides types and helpers for the agent API.

const HeaderProvider = "X-Tmagent-Provider" ...
type BundleDefinitionDTO struct{ ... }
type BundleInstanceDTO struct{ ... }
type ChatMessageDTO struct{ ... }
type ChatResponse struct{ ... }
type InputDTO struct{ ... }
type PromptDTO struct{ ... }
type ToolCall struct{ ... }
type ToolCallResult struct{ ... }
EOT

  filename = "${path.module}/mock-agent.ignore"
}
