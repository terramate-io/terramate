// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/agent"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/typeschema"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

// cloudAuthCache caches loaded cloud credentials to avoid repeated network
// calls (MemberOrganizations, Users) on every chat message. It is safe for
// concurrent use and reloads automatically when the credential is absent
// (e.g. after the user completes the in-TUI cloud login flow).
type cloudAuthCache struct {
	mu      sync.Mutex
	cred    cliauth.Credential
	orgUUID string
}

// auth returns a valid Bearer token and the active organisation UUID.
// The credential is loaded lazily on the first call; subsequent calls reuse
// the cached credential (Token() handles its own refresh internally).
func (c *cloudAuthCache) auth(cloudBaseURL string, cliCfg cliconfig.Config) (token, orgUUID string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cred == nil {
		if err := c.loadLocked(cloudBaseURL, cliCfg); err != nil {
			return "", "", err
		}
	}

	token, err = c.cred.Token()
	return token, c.orgUUID, err
}

// reset clears the cached credential so the next auth() call reloads from
// disk (called after the user completes cloud login in the TUI).
func (c *cloudAuthCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cred = nil
	c.orgUUID = ""
}

// loadLocked is called with c.mu held. It probes for a stored credential,
// fetches the user's organisations, and caches the first active one.
func (c *cloudAuthCache) loadLocked(cloudBaseURL string, cliCfg cliconfig.Config) error {
	client := cloud.NewClient(cloud.WithBaseURL(cloudBaseURL))
	pp := discardPrinters()
	for _, probe := range cliauth.ProbingPrecedence(pp, 0, client, cliCfg) {
		found, err := probe.Load()
		if err != nil {
			// Bit of a hack, but it's fine.
			if strings.Contains(err.Error(), "status: 401") {
				return errors.E("To use the AI assistant, you need to be logged in to Terramate Cloud.")
			}
			return errors.E(err, "cloud authentication failed")
		}
		if found {
			c.cred = probe
			break
		}
	}
	if c.cred == nil {
		return errors.E("Not logged in to Terramate Cloud. Please login first.")
	}
	orgs := c.cred.Organizations()
	if len(orgs) == 0 {
		c.cred = nil
		return errors.E("No Terramate Cloud organisations found. Please complete onboarding at https://cloud.terramate.io")
	}
	c.orgUUID = string(orgs[0].UUID)
	return nil
}

func discardPrinters() printer.Printers {
	return printer.Printers{
		Stdout: printer.NewPrinter(io.Discard),
		Stderr: printer.NewPrinter(io.Discard),
	}
}

// cloudChatRequest is the JSON payload sent to POST /v1/agent/{org_uuid}/chat.
// Field names match the cloud API spec exactly.
type cloudChatRequest struct {
	Message           string                      `json:"message"`
	History           []cloudChatMessage          `json:"history,omitempty"`
	PendingProposals  []int                       `json:"pending_proposals,omitempty"`
	BundleDefinitions []agent.BundleDefinitionDTO `json:"bundle_definitions,omitempty"`
	BundleInstances   []agent.BundleInstanceDTO   `json:"bundle_instances,omitempty"`
}

// cloudChatMessage is a single conversation turn in cloudChatRequest.
type cloudChatMessage struct {
	Role        string                 `json:"role"`
	Content     string                 `json:"content"`
	ToolCalls   []cloudToolCall        `json:"tool_calls,omitempty"`
	ToolResults []agent.ToolCallResult `json:"tool_results,omitempty"`
}

// cloudToolCall carries tool invocations. Arguments is a plain JSON string
// (the cloud API serialises json.RawMessage as a string).
type cloudToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// cloudChatResponse is decoded from the cloud API response body.
type cloudChatResponse struct {
	Text      string          `json:"text"`
	ToolCalls []cloudToolCall `json:"tool_calls,omitempty"`
}

// buildBundleContext returns the bundle definition and instance DTOs for the
// current project state, suitable for inclusion in a chat request.
func (m *Model) buildBundleContext() ([]agent.BundleDefinitionDTO, []agent.BundleInstanceDTO) {
	defs := bundleDefsToDTO(m.EngineState.LocalBundleDefs)
	instances := bundleInstancesToDTO(m.EngineState.Registry.Bundles, m.EngineState.LocalBundleDefs, m.selectedEnv)
	return defs, instances
}

// bundleDefsToDTO converts local bundle definition entries into the JSON-friendly
// DTOs included in each chat request. It re-tokenizes HCL expressions from the
// parsed AST rather than evaluating them, preserving the original syntax.
func bundleDefsToDTO(entries []config.BundleDefinitionEntry) []agent.BundleDefinitionDTO {
	dtos := make([]agent.BundleDefinitionDTO, 0, len(entries))
	for _, entry := range entries {
		dto := agent.BundleDefinitionDTO{
			Source:      entry.Source,
			Name:        entry.Metadata.Name,
			Class:       entry.Metadata.Class,
			Version:     entry.Metadata.Version,
			Description: entry.Metadata.Description,
		}
		if entry.Define != nil {
			dto.Inputs = inputsToDTO(entry.Define.Inputs)

			if entry.Define.Scaffolding.Name == nil {
				dto.Inputs = append(dto.Inputs, agent.InputDTO{
					Name:        pseudoKeyOutputName,
					Type:        "string",
					Description: "Name of the created bundle instance.",
				})
			}
			if entry.Define.Scaffolding.Path == nil {
				dto.Inputs = append(dto.Inputs, agent.InputDTO{
					Name:        pseudoKeyOutputPath,
					Type:        "string",
					Description: "Output file path (must end in .tm.yml). Paths starting with / are relative to the project root; otherwise relative to the current directory. (Important) This path must be unique, do not use the same path for multiple bundles.",
				})
			}
		}
		dtos = append(dtos, dto)
	}
	return dtos
}

// bundleInstancesToDTO converts existing bundle instances into the JSON-friendly
// DTOs included in each chat request. Input values are serialized back to HCL
// expression strings via ast.TokensForValue.
// Only prompted inputs are included; non-prompted inputs are hidden from the AI.
func bundleInstancesToDTO(bundles []*config.Bundle, defs []config.BundleDefinitionEntry, curEnv *config.Environment) []agent.BundleInstanceDTO {
	promptedInputs := buildPromptedInputsIndex(defs)

	dtos := make([]agent.BundleInstanceDTO, 0, len(bundles))
	for _, b := range bundles {
		var envID string
		if b.Environment != nil {
			envID = b.Environment.ID
			if curEnv != nil && curEnv.ID != envID {
				continue
			}
		}

		allowed := promptedInputs[b.DefinitionMetadata.Name]
		inputs := make(map[string]string, len(b.Inputs))
		for k, wrapped := range b.Inputs {
			if allowed != nil && !allowed[k] {
				continue
			}
			v := wrapped.GetAttr("value")
			tokens := ast.TokensForValue(v)
			inputs[k] = strings.TrimSpace(string(tokens.Bytes()))
		}

		location := fmt.Sprintf("%s:%s", b.Workdir.String(), b.Name)
		dtos = append(dtos, agent.BundleInstanceDTO{
			Location:         location,
			DefinitionSource: b.Source,
			Name:             b.Name,
			EnvID:            envID,
			Inputs:           inputs,
		})
	}
	return dtos
}

// buildPromptedInputsIndex returns a map from definition name to the set of
// input names that have a prompt block. Definitions without a define block
// are omitted (all inputs allowed).
func buildPromptedInputsIndex(defs []config.BundleDefinitionEntry) map[string]map[string]bool {
	idx := make(map[string]map[string]bool, len(defs))
	for _, entry := range defs {
		if entry.Define == nil {
			continue
		}
		allowed := make(map[string]bool, len(entry.Define.Inputs))
		for _, inp := range entry.Define.Inputs {
			if inp.Prompt != nil {
				allowed[inp.Name] = true
			}
		}
		idx[entry.Metadata.Name] = allowed
	}
	return idx
}

func inputsToDTO(inputs map[string]*hcl.DefineInput) []agent.InputDTO {
	if len(inputs) == 0 {
		return nil
	}
	dtos := make([]agent.InputDTO, 0, len(inputs))
	for _, input := range inputs {
		if input.Prompt == nil {
			continue
		}
		dto := agent.InputDTO{
			Name:        input.Name,
			Type:        exprToString(input.Type),
			Description: unquote(exprToString(input.Description)),
			Default:     exprToString(input.Default),
			Prompt:      promptToDTO(input.Prompt),
		}
		dtos = append(dtos, dto)
	}
	return dtos
}

func promptToDTO(p *hcl.DefineInputPrompt) *agent.PromptDTO {
	if p == nil {
		return nil
	}
	dto := &agent.PromptDTO{
		Text:        unquote(exprToString(p.Text)),
		Options:     exprToString(p.Options),
		Multiline:   exprToString(p.Multiline),
		Multiselect: exprToString(p.Multiselect),
	}
	if dto.Text == "" && dto.Options == "" && dto.Multiline == "" && dto.Multiselect == "" {
		return nil
	}
	return dto
}

// exprToString re-tokenizes an HCL attribute expression back to its source text.
func exprToString(attr *ast.Attribute) string {
	if attr == nil {
		return ""
	}
	return hclExprString(attr.Expr)
}

func hclExprString(expr hhcl.Expression) string {
	if expr == nil {
		return ""
	}
	return strings.TrimSpace(string(ast.TokensForExpression(expr).Bytes()))
}

// unquote strips surrounding double-quotes from simple HCL string literals.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// llmConfig holds the LLM provider configuration passed to tmagent via HTTP headers.
type llmConfig struct {
	enabled  bool   // true when --experimental-ai-prompt is set
	Provider string // "openai" or "anthropic"
	APIKey   string
	Model    string // optional override; empty = use server default
}

// sendChatMessage sends a user message to the Terramate Cloud agent endpoint
// POST /v1/agent/{org_uuid}/chat. Bundle context is included with every
// request so the server stays stateless. LLM provider configuration is passed
// via X-Tmagent-* headers; a Bearer token from the stored cloud credential is
// used for authentication.
func sendChatMessage(
	cloudBaseURL string,
	cloudAuth *cloudAuthCache,
	cliCfg cliconfig.Config,
	llm llmConfig,
	message string,
	history []agent.ChatMessageDTO,
	pendingProposals []int,
	bundleDefs []agent.BundleDefinitionDTO,
	bundleInstances []agent.BundleInstanceDTO,
) (*agent.ChatResponse, error) {
	if llm.Provider == "" || llm.APIKey == "" {
		return nil, errors.E("No AI provider configured. " +
			"Set ANTHROPIC_API_KEY or OPENAI_API_KEY environment variables to use the agent. " +
			"You can optionally override the model with ANTHROPIC_MODEL or OPENAI_MODEL.")
	}

	token, orgUUID, err := cloudAuth.auth(cloudBaseURL, cliCfg)
	if err != nil {
		return nil, err
	}

	// Build the cloud-API request, converting tmagent ToolCall.Arguments
	// (json.RawMessage) to the string format the cloud API expects.
	req := cloudChatRequest{
		Message:           message,
		PendingProposals:  pendingProposals,
		BundleDefinitions: bundleDefs,
		BundleInstances:   bundleInstances,
	}
	for _, h := range history {
		msg := cloudChatMessage{
			Role:        h.Role,
			Content:     h.Content,
			ToolResults: h.ToolResults,
		}
		for _, tc := range h.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, cloudToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: string(tc.Arguments),
			})
		}
		req.History = append(req.History, msg)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, errors.E(err, "marshaling chat request")
	}

	url := fmt.Sprintf("%s/v1/agent/%s/chat", cloudBaseURL, orgUUID)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.E(err, "creating request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set(agent.HeaderProvider, llm.Provider)
	httpReq.Header.Set(agent.HeaderAPIKey, llm.APIKey)
	if llm.Model != "" {
		httpReq.Header.Set(agent.HeaderModel, llm.Model)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, errors.E(err, "calling cloud agent")
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, errors.E("cloud agent error %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var cloudResp cloudChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cloudResp); err != nil {
		return nil, errors.E(err, "decoding response")
	}

	// Convert cloud response back to tmagent types, restoring Arguments from
	// the JSON string the cloud API sends to raw json.RawMessage.
	result := &agent.ChatResponse{Text: cloudResp.Text}
	for _, tc := range cloudResp.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, agent.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: json.RawMessage(tc.Arguments),
		})
	}
	return result, nil
}

// buildChatHistory converts the model's chat messages into tmagent DTOs
// for the conversation history, including tool calls and tool results.
func (m *Model) buildChatHistory() []agent.ChatMessageDTO {
	var history []agent.ChatMessageDTO
	for _, msg := range m.chatMessages {
		if msg.LocalOnly {
			continue
		}
		switch msg.Role {
		case "user":
			if len(msg.ToolResults) > 0 {
				history = append(history, agent.ChatMessageDTO{
					Role:        "user",
					ToolResults: msg.ToolResults,
				})
			} else {
				history = append(history, agent.ChatMessageDTO{
					Role:    "user",
					Content: msg.Content,
				})
			}
		case "ai":
			history = append(history, agent.ChatMessageDTO{
				Role:      "assistant",
				Content:   msg.Content,
				ToolCalls: msg.ToolCalls,
			})
		}
	}
	return history
}

// pendingProposalIDs returns the ProposalIDs of all current proposed changes.
func (m *Model) pendingProposalIDs() []int {
	ids := make([]int, len(m.ProposedChanges()))
	for i, c := range m.ProposedChanges() {
		ids[i] = c.ProposalID
	}
	return ids
}

// toolCallOutcome wraps a protocol-level ToolCallResult with an optional
// user-facing message. UserMessage is rendered in the chat UI but never
// sent back to the LLM, keeping the protocol DTO clean.
type toolCallOutcome struct {
	Result      agent.ToolCallResult
	UserMessage string
}

func errorOutcome(toolCallID, content string) *toolCallOutcome {
	return &toolCallOutcome{
		Result: agent.ToolCallResult{
			ToolCallID: toolCallID,
			Content:    content,
			IsError:    true,
		},
	}
}

// processAgentToolCalls converts LLM tool calls into fully resolved Change
// proposals. Each tool call goes through input evaluation (filling defaults)
// and validation. If any step fails the Change is created with an Error field
// so the failure can be reported back to the LLM as a tool result.
func (m *Model) processAgentToolCalls(toolCalls []agent.ToolCall) ([]toolCallOutcome, []Change) {
	var outcomes []toolCallOutcome
	var changes []Change

	for _, tc := range toolCalls {
		var outcome *toolCallOutcome
		var change *Change
		switch tc.Name {
		case "add_bundle":
			outcome, change = m.processAddBundleToolCall(tc)
		case "reconfig_bundle":
			outcome, change = m.processReconfigBundleToolCall(tc)
		default:
			outcome = &toolCallOutcome{
				Result: agent.ToolCallResult{
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("the tool %q is not supported by this client.", tc.Name),
					IsError:    true,
				},
			}
		}

		if change != nil {
			changes = append(changes, *change)
		}
		if outcome != nil {
			outcomes = append(outcomes, *outcome)
		}
	}
	return outcomes, changes
}

func (m *Model) calcIncrementalInputs(proposalID *int, newInputs map[string]string) (int, map[string]cty.Value) {
	inputValues := map[string]cty.Value{}

	for k, v := range newInputs {
		inputValues[k] = parseHCLValue(v)
	}

	changeIdx := -1
	if proposalID != nil {
		for i, p := range m.ProposedChanges() {
			if p.ProposalID != *proposalID {
				continue
			}
			for k, v := range p.Values {
				if _, exists := inputValues[k]; !exists {
					inputValues[k] = v
				}
			}
			changeIdx = i
			break
		}
	}
	return changeIdx, inputValues
}

func (m *Model) processAddBundleToolCall(tc agent.ToolCall) (*toolCallOutcome, *Change) {
	est := m.EngineState

	var args struct {
		Source     string `json:"source"`
		ProposalID *int   `json:"proposal_id,omitempty"`
	}
	if err := json.Unmarshal(tc.Arguments, &args); err != nil {
		return errorOutcome(tc.ID, fmt.Sprintf("invalid arguments: %v.", err)), nil
	}

	inputs := extractFlatInputs(tc.Arguments)

	bde := m.findLocalBundleDefBySource(args.Source)
	if bde == nil {
		return errorOutcome(tc.ID, fmt.Sprintf("could not find a bundle definition at source %q.", args.Source)), nil
	}

	evalctx := est.Evalctx.ChildContext()
	if err := checkEnvRequired(evalctx, bde.Define, m.EngineState.Registry.Environments); err != nil {
		return errorOutcome(tc.ID, "this bundle requires environments, but the user has no environments configured."), nil
	}

	schemas, err := config.EvalBundleSchemaNamespaces(est.Root, est.ResolveAPI, evalctx, bde.Define, true)
	if err != nil {
		return errorOutcome(tc.ID, "the local bundle definition contains errors and could not be loaded."), nil
	}

	schemactx := typeschema.EvalContext{
		Evalctx: evalctx,
		Schemas: schemas,
	}

	inputDefs, err := config.EvalBundleInputDefinitions(schemactx, bde.Define)
	if err != nil {
		return errorOutcome(tc.ID, "the local bundle definition contains errors and could not be loaded."), nil
	}

	changeIdx, inputValues := m.calcIncrementalInputs(args.ProposalID, inputs)
	if changeIdx != -1 {
		m.ProposedChanges()[changeIdx].MarkedForReplacement = true
	}
	change, err := NewCreateChange(
		est,
		m.selectedEnv,
		bde,
		schemactx,
		inputDefs,
		inputValues,
	)
	if err != nil {
		if changeIdx != -1 {
			m.ProposedChanges()[changeIdx].MarkedForReplacement = false
		}
		return errorOutcome(tc.ID, fmt.Sprintf("the bundle inputs values were invalid. error: %s.", err)), nil
	}

	if changeIdx != -1 {
		change.ProposalID = *args.ProposalID
		m.ProposedChanges()[changeIdx] = change
		return &toolCallOutcome{
			Result: agent.ToolCallResult{
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("Successfully updated Proposal %d. To update this proposal, use proposal_id=%d.", change.ProposalID, change.ProposalID),
			},
			UserMessage: fmt.Sprintf("Updated Proposed Change #%d.", change.ProposalID),
		}, nil
	}

	change.ProposalID = m.nextProposalID
	m.nextProposalID++

	return &toolCallOutcome{
		Result: agent.ToolCallResult{
			ToolCallID: tc.ID,
			Content:    fmt.Sprintf("Successfully created Proposal %d. To update this proposal, use proposal_id=%d.", change.ProposalID, change.ProposalID),
		},
		UserMessage: fmt.Sprintf("Created Proposed Change #%d.", change.ProposalID),
	}, &change
}

func (m *Model) processReconfigBundleToolCall(tc agent.ToolCall) (*toolCallOutcome, *Change) {
	est := m.EngineState

	var args struct {
		Location   string `json:"location"`
		ProposalID *int   `json:"proposal_id,omitempty"`
	}
	if err := json.Unmarshal(tc.Arguments, &args); err != nil {
		return errorOutcome(tc.ID, fmt.Sprintf("invalid arguments: %v.", err)), nil
	}

	inputs := extractFlatInputs(tc.Arguments)

	b := m.findBundleByLocation(args.Location)
	if b == nil {
		return errorOutcome(tc.ID, fmt.Sprintf("could not find a bundle at location %q.", args.Location)), nil
	}

	bde := makeBundleDefinitionEntry(m.EngineState.Root, b)
	if bde == nil {
		return errorOutcome(tc.ID, fmt.Sprintf("the bundle at location %q could not be loaded.", args.Location)), nil
	}

	schemactx, err := m.loadBundleEvalContext(bde)
	if err != nil {
		return errorOutcome(tc.ID, fmt.Sprintf("the bundle at location %q could not be loaded.", args.Location)), nil
	}

	inputDefs, err := config.EvalBundleInputDefinitions(schemactx, bde.Define)
	if err != nil {
		return errorOutcome(tc.ID, "the local bundle definition contains errors and could not be loaded."), nil
	}

	// Start from the existing bundle's input values so that omitted inputs keep their current values.
	inputValues := inputsToValueMap(b.Inputs)

	// When updating a pending proposal, merge from the proposal's values so that
	// previously-reconfigured attributes aren't lost if the LLM only sends a
	// partial update. The proposal's Values are the full resolved set from the
	// last call, so they layer correctly on top of the original bundle inputs.
	changeIdx := -1
	if args.ProposalID != nil {
		for i, p := range m.ProposedChanges() {
			if p.ProposalID == *args.ProposalID {
				for k, v := range p.Values {
					inputValues[k] = v
				}
				changeIdx = i
				break
			}
		}
	}

	for k, v := range inputs {
		if k == pseudoKeyOutputName || k == pseudoKeyOutputPath {
			continue
		}
		inputValues[k] = parseHCLValue(v)
	}

	if changeIdx != -1 {
		m.ProposedChanges()[changeIdx].MarkedForReplacement = true
	}
	change, err := NewReconfigChange(est, b, bde, schemactx, inputDefs, inputValues)
	if err != nil {
		if changeIdx != -1 {
			m.ProposedChanges()[changeIdx].MarkedForReplacement = false
		}
		return errorOutcome(tc.ID, fmt.Sprintf("the bundle inputs values were invalid. error: %s.", err)), nil
	}

	if changeIdx != -1 {
		change.ProposalID = *args.ProposalID
		m.ProposedChanges()[changeIdx] = change
		return &toolCallOutcome{
			Result: agent.ToolCallResult{
				ToolCallID: tc.ID,
				Content:    fmt.Sprintf("Successfully updated Proposal %d. To update this proposal, use proposal_id=%d.", change.ProposalID, change.ProposalID),
			},
			UserMessage: fmt.Sprintf("Updated Proposed Change #%d.", change.ProposalID),
		}, nil
	}

	change.ProposalID = m.nextProposalID
	m.nextProposalID++

	return &toolCallOutcome{
		Result: agent.ToolCallResult{
			ToolCallID: tc.ID,
			Content:    fmt.Sprintf("Successfully created Proposal %d. To update this proposal, use proposal_id=%d.", change.ProposalID, change.ProposalID),
		},
		UserMessage: fmt.Sprintf("Created Proposed Change #%d.", change.ProposalID),
	}, &change
}

// extractFlatInputs extracts bundle input values from flat tool call arguments.
// Reserved keys (source, location, proposal_id) are skipped; everything else
// is treated as an input. Also supports a legacy nested "inputs" object for
// backward compatibility with models that still nest values.
func extractFlatInputs(raw json.RawMessage) map[string]string {
	var allKeys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &allKeys); err != nil {
		return nil
	}

	reserved := map[string]bool{
		"source": true, "location": true, "proposal_id": true,
	}

	inputs := make(map[string]string, len(allKeys))

	// Support legacy nested "inputs" object — values are already HCL expression strings.
	if nested, ok := allKeys["inputs"]; ok {
		var nestedMap map[string]string
		if json.Unmarshal(nested, &nestedMap) == nil {
			for k, v := range nestedMap {
				inputs[k] = v
			}
		}
	}

	for k, v := range allKeys {
		if reserved[k] || k == "inputs" {
			continue
		}
		if _, exists := inputs[k]; exists {
			continue
		}
		inputs[k] = rawJSONToHCL(v)
	}

	return inputs
}

// rawJSONToHCL converts a raw JSON value to an HCL expression string.
// JSON strings are passed through as-is since they already represent HCL
// expressions (or plain values that parseHCLValue handles gracefully).
// Non-string JSON types (bool, number, array, object) are converted to HCL syntax.
func rawJSONToHCL(raw json.RawMessage) string {
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	return jsonValueToHCL(raw)
}

// jsonValueToHCL converts a non-string JSON value to an HCL expression string.
func jsonValueToHCL(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return ""
	}

	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		if b {
			return "true"
		}
		return "false"
	}

	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		elems := make([]string, 0, len(arr))
		for _, elem := range arr {
			elems = append(elems, rawJSONToHCLQuoted(elem))
		}
		return "[" + strings.Join(elems, ", ") + "]"
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		kvs := make([]string, 0, len(obj))
		for k, v := range obj {
			kvs = append(kvs, fmt.Sprintf("%s = %s", k, rawJSONToHCLQuoted(v)))
		}
		return "{" + strings.Join(kvs, ", ") + "}"
	}

	return s
}

// rawJSONToHCLQuoted converts a JSON value to HCL, quoting string values.
// Used for values nested inside arrays and objects where strings must be
// HCL string literals (e.g. ["a", "b"] not [a, b]).
func rawJSONToHCLQuoted(raw json.RawMessage) string {
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return fmt.Sprintf("%q", str)
	}
	return jsonValueToHCL(raw)
}

// parseHCLValue parses a string as an HCL expression and evaluates it to a cty.Value.
// For literal expressions (true, 42, "hello", ["a","b"]) no eval context is needed.
// Falls back to treating the raw string as a cty.StringVal if parsing fails.
func parseHCLValue(s string) cty.Value {
	expr, err := ast.ParseExpression(s, "<agent>")
	if err != nil {
		return cty.StringVal(s)
	}
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return cty.StringVal(s)
	}
	return val
}
