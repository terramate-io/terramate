// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "v1" {
  content = <<-EOT
package v1 // import "github.com/terramate-io/terramate/plugin/proto/v1"

const PluginService_GetPluginInfo_FullMethodName = "/terramate.plugin.v1.PluginService/GetPluginInfo" ...
const CommandService_GetCommands_FullMethodName = "/terramate.plugin.v1.CommandService/GetCommands" ...
const HCLSchemaService_GetHCLSchema_FullMethodName = "/terramate.plugin.v1.HCLSchemaService/GetHCLSchema" ...
const HostService_ReadFile_FullMethodName = "/terramate.plugin.v1.HostService/ReadFile" ...
const GenerateService_Generate_FullMethodName = "/terramate.plugin.v1.GenerateService/Generate"
const LifecycleService_PostInit_FullMethodName = "/terramate.plugin.v1.LifecycleService/PostInit"
var BlockKind_name = map[int32]string{ ... } ...
var Diagnostic_Severity_name = map[int32]string{ ... } ...
var CommandService_ServiceDesc = grpc.ServiceDesc{ ... }
var File_plugin_proto protoreflect.FileDescriptor
var GenerateService_ServiceDesc = grpc.ServiceDesc{ ... }
var HCLSchemaService_ServiceDesc = grpc.ServiceDesc{ ... }
var HostService_ServiceDesc = grpc.ServiceDesc{ ... }
var LifecycleService_ServiceDesc = grpc.ServiceDesc{ ... }
var PluginService_ServiceDesc = grpc.ServiceDesc{ ... }
func RegisterCommandServiceServer(s grpc.ServiceRegistrar, srv CommandServiceServer)
func RegisterGenerateServiceServer(s grpc.ServiceRegistrar, srv GenerateServiceServer)
func RegisterHCLSchemaServiceServer(s grpc.ServiceRegistrar, srv HCLSchemaServiceServer)
func RegisterHostServiceServer(s grpc.ServiceRegistrar, srv HostServiceServer)
func RegisterLifecycleServiceServer(s grpc.ServiceRegistrar, srv LifecycleServiceServer)
func RegisterPluginServiceServer(s grpc.ServiceRegistrar, srv PluginServiceServer)
type AttributeValue struct{ ... }
type AttributeValue_BoolValue struct{ ... }
type AttributeValue_ExpressionText struct{ ... }
type AttributeValue_FloatValue struct{ ... }
type AttributeValue_IntValue struct{ ... }
type AttributeValue_JsonValue struct{ ... }
type AttributeValue_StringValue struct{ ... }
type BlockKind int32
    const BlockKind_BLOCK_UNMERGED BlockKind = 0 ...
type Capabilities struct{ ... }
type CommandArg struct{ ... }
type CommandFlag struct{ ... }
type CommandInput struct{ ... }
type CommandInput_FormResponse struct{ ... }
type CommandInput_Request struct{ ... }
type CommandInput_Stdin struct{ ... }
type CommandInput_StdinClose struct{ ... }
type CommandList struct{ ... }
type CommandOutput struct{ ... }
type CommandOutput_ExitCode struct{ ... }
type CommandOutput_FileWrite struct{ ... }
type CommandOutput_FormRequest struct{ ... }
type CommandOutput_Stderr struct{ ... }
type CommandOutput_Stdout struct{ ... }
type CommandRequest struct{ ... }
type CommandServiceClient interface{ ... }
    func NewCommandServiceClient(cc grpc.ClientConnInterface) CommandServiceClient
type CommandServiceServer interface{ ... }
type CommandService_ExecuteCommandClient = grpc.ServerStreamingClient[CommandOutput]
type CommandService_ExecuteCommandServer = grpc.ServerStreamingServer[CommandOutput]
type CommandService_ExecuteCommandWithInputClient = grpc.BidiStreamingClient[CommandInput, CommandOutput]
type CommandService_ExecuteCommandWithInputServer = grpc.BidiStreamingServer[CommandInput, CommandOutput]
type CommandSpec struct{ ... }
type ConfigPatch struct{ ... }
type ConfigTreeNode struct{ ... }
type ConfirmFormField struct{ ... }
type Diagnostic struct{ ... }
type Diagnostic_Severity int32
    const Diagnostic_ERROR Diagnostic_Severity = 0 ...
type DirEntry struct{ ... }
type Empty struct{ ... }
type FileWrite struct{ ... }
type FormField struct{ ... }
type FormField_Confirm struct{ ... }
type FormField_MultiSelect struct{ ... }
type FormField_Select struct{ ... }
type FormField_TextArea struct{ ... }
type FormField_TextInput struct{ ... }
type FormOption struct{ ... }
type FormRequest struct{ ... }
type FormResponse struct{ ... }
type GenerateOutput struct{ ... }
type GenerateOutput_ExitCode struct{ ... }
type GenerateOutput_FileWrite struct{ ... }
type GenerateOutput_Stderr struct{ ... }
type GenerateOutput_Stdout struct{ ... }
type GenerateRequest struct{ ... }
type GenerateServiceClient interface{ ... }
    func NewGenerateServiceClient(cc grpc.ClientConnInterface) GenerateServiceClient
type GenerateServiceServer interface{ ... }
type GenerateService_GenerateClient = grpc.ServerStreamingClient[GenerateOutput]
type GenerateService_GenerateServer = grpc.ServerStreamingServer[GenerateOutput]
type GetConfigTreeRequest struct{ ... }
type GetStackRequest struct{ ... }
type HCLAttributeSchema struct{ ... }
type HCLBlockSchema struct{ ... }
type HCLSchemaList struct{ ... }
type HCLSchemaServiceClient interface{ ... }
    func NewHCLSchemaServiceClient(cc grpc.ClientConnInterface) HCLSchemaServiceClient
type HCLSchemaServiceServer interface{ ... }
type HostServiceClient interface{ ... }
    func NewHostServiceClient(cc grpc.ClientConnInterface) HostServiceClient
type HostServiceServer interface{ ... }
type HostService_WalkDirClient = grpc.ServerStreamingClient[DirEntry]
type HostService_WalkDirServer = grpc.ServerStreamingServer[DirEntry]
type LifecycleServiceClient interface{ ... }
    func NewLifecycleServiceClient(cc grpc.ClientConnInterface) LifecycleServiceClient
type LifecycleServiceServer interface{ ... }
type MultiSelectFormField struct{ ... }
type ParsedBlock struct{ ... }
type ParsedBlocksRequest struct{ ... }
type ParsedBlocksResponse struct{ ... }
type PluginInfo struct{ ... }
type PluginServiceClient interface{ ... }
    func NewPluginServiceClient(cc grpc.ClientConnInterface) PluginServiceClient
type PluginServiceServer interface{ ... }
type PostInitRequest struct{ ... }
type PostInitResponse struct{ ... }
type ReadFileRequest struct{ ... }
type ReadFileResponse struct{ ... }
type SelectFormField struct{ ... }
type SetStackRequest struct{ ... }
type StackMetadata struct{ ... }
type StackMetadataUpdate struct{ ... }
type StringValue struct{ ... }
type TextAreaFormField struct{ ... }
type TextInputFormField struct{ ... }
type UnimplementedCommandServiceServer struct{}
type UnimplementedGenerateServiceServer struct{}
type UnimplementedHCLSchemaServiceServer struct{}
type UnimplementedHostServiceServer struct{}
type UnimplementedLifecycleServiceServer struct{}
type UnimplementedPluginServiceServer struct{}
type UnsafeCommandServiceServer interface{ ... }
type UnsafeGenerateServiceServer interface{ ... }
type UnsafeHCLSchemaServiceServer interface{ ... }
type UnsafeHostServiceServer interface{ ... }
type UnsafeLifecycleServiceServer interface{ ... }
type UnsafePluginServiceServer interface{ ... }
type WalkDirRequest struct{ ... }
type WriteFileRequest struct{ ... }
EOT

  filename = "${path.module}/mock-v1.ignore"
}
