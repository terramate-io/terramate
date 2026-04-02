// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "ui" {
  content = <<-EOT
package ui // import "github.com/terramate-io/terramate/commands/ui"

Package ui implements the terramate ui command.

func FormatDisplayValue(val cty.Value, typ typeschema.Type) string
type BoolWidget struct{ ... }
    func NewBoolWidget(wctx *WidgetContext) *BoolWidget
type BundleOption struct{ ... }
type BundleRefWidget struct{ ... }
    func NewBundleRefWidget(wctx *WidgetContext, classID string) *BundleRefWidget
type BundleSelectPage int
    const BundleSelectCollection BundleSelectPage = iota ...
type Change struct{ ... }
    func NewChangeFromExisting(est *EngineState, oldChange Change, schemactx typeschema.EvalContext, ...) (Change, error)
    func NewCreateChange(est *EngineState, activeEnv *config.Environment, ...) (Change, error)
    func NewPromoteChange(est *EngineState, env *config.Environment, bundle *config.Bundle, ...) (Change, error)
    func NewReconfigChange(est *EngineState, bundle *config.Bundle, bde *config.BundleDefinitionEntry, ...) (Change, error)
type ChangeKind string
    const ChangeCreate ChangeKind = "change_create" ...
type CreateFrame struct{ ... }
type EngineState struct{ ... }
type FocusArea int
    const FocusCommands FocusArea = iota ...
type InlineListWidget struct{ ... }
    func NewInlineListWidget(wctx *WidgetContext, valueType typeschema.Type) *InlineListWidget
type InlineMapWidget struct{ ... }
    func NewInlineMapWidget(wctx *WidgetContext, valueType typeschema.Type) *InlineMapWidget
type InputFocusArea int
    const InputFocusActive InputFocusArea = iota ...
type InputOption struct{ ... }
type InputWidget interface{ ... }
    func NewListWidget(wctx *WidgetContext, valTyp typeschema.Type) InputWidget
    func NewMapWidget(wctx *WidgetContext, valTyp typeschema.Type) InputWidget
    func NewSubFormListWidget(wctx *WidgetContext, valueType typeschema.Type) InputWidget
    func NewSubFormMapWidget(wctx *WidgetContext, valueType typeschema.Type) InputWidget
    func NewWidget(wctx *WidgetContext, typ typeschema.Type) InputWidget
type InputsForm struct{ ... }
    func NewInputsForm(inputDefs []*config.InputDefinition, schemactx typeschema.EvalContext, ...) InputsForm
    func NewInputsFormWithValues(inputDefs []*config.InputDefinition, schemactx typeschema.EvalContext, ...) InputsForm
type InputsFormState int
    const InputsFormActive InputsFormState = iota ...
type Model struct{ ... }
    func NewModel(est *EngineState) Model
type MultiSelectWidget struct{ ... }
    func NewMultiSelectWidget(wctx *WidgetContext) *MultiSelectWidget
type MultilineWidget struct{ ... }
    func NewMultilineWidget(wctx *WidgetContext) *MultilineWidget
type ObjectEditFrame struct{ ... }
type ObjectWidget struct{ ... }
    func NewObjectWidget(wctx *WidgetContext, objType *typeschema.ObjectType) *ObjectWidget
type Registry struct{ ... }
type SavedChange struct{ ... }
type SelectWidget struct{ ... }
    func NewSelectWidget(wctx *WidgetContext) *SelectWidget
type SharedWidgetContext struct{ ... }
type Spec struct{ ... }
type SubFormListWidget struct{ ... }
type SubFormMapWidget struct{ ... }
type SubFormRequest struct{ ... }
type SubFormResult struct{ ... }
type TextWidget struct{ ... }
    func NewTextWidget(wctx *WidgetContext, valueType typeschema.Type) *TextWidget
type ViewState int
    const ViewCloudLogin ViewState = iota ...
type WidgetContext struct{ ... }
type WidgetSignal int
    const WidgetContinue WidgetSignal = iota ...
EOT

  filename = "${path.module}/mock-ui.ignore"
}
