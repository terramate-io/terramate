// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package execgrpc executes plugin commands over gRPC.
package execgrpc

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/exit"
	plugingrpc "github.com/terramate-io/terramate/plugin/grpc"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Spec executes a plugin command over gRPC.
type Spec struct {
	PluginName string
	BinaryPath string
	Command    string
	Args       map[string]string
	Flags      map[string]string
}

// Name returns the command name.
func (*Spec) Name() string { return "plugin exec grpc" }

// Requirements returns command requirements.
func (*Spec) Requirements(context.Context, commands.CLI) any {
	return &commands.EngineRequirement{}
}

// Exec executes the plugin command.
func (s *Spec) Exec(ctx context.Context, cli commands.CLI) error {
	if s.BinaryPath == "" {
		return errors.E("plugin binary path is required")
	}
	if s.Command == "" {
		return errors.E("plugin command is required")
	}

	hostAddr, stopHost, err := startHostService(cli)
	if err != nil {
		return err
	}
	defer stopHost()

	var extraEnv []string
	if hostAddr != "" {
		extraEnv = append(extraEnv, plugingrpc.HostAddrEnv+"="+hostAddr)
	}

	client, err := plugingrpc.NewHostClientWithEnv(s.BinaryPath, extraEnv)
	if err != nil {
		return err
	}
	defer client.Kill()

	rootDir := ""
	if cli.Engine() != nil {
		rootDir = cli.Engine().Config().HostDir()
	}

	req := &pb.CommandRequest{
		Command:    s.Command,
		Args:       s.Args,
		Flags:      s.Flags,
		WorkingDir: cli.WorkingDir(),
		RootDir:    rootDir,
	}

	mode := commandInputNone
	if s.Command == "scaffold" {
		mode = commandInputForm
	}
	exitCode, err := executeCommand(ctx, cli, client.Client().CommandService, req, mode)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		return errors.E(exit.Status(exitCode))
	}
	return nil
}

type commandOutputStream interface {
	Recv() (*pb.CommandOutput, error)
}

type commandInputSender interface {
	Send(*pb.CommandInput) error
}

func consumeCommandStream(stream commandOutputStream, sender commandInputSender, cli commands.CLI) (int32, error) {
	var exitCode int32
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return exitCode, err
		}

		switch out := msg.Output.(type) {
		case *pb.CommandOutput_Stdout:
			if _, err := cli.Stdout().Write(out.Stdout); err != nil {
				return exitCode, err
			}
		case *pb.CommandOutput_Stderr:
			if _, err := cli.Stderr().Write(out.Stderr); err != nil {
				return exitCode, err
			}
		case *pb.CommandOutput_FileWrite:
			if err := writeFile(cli.WorkingDir(), out.FileWrite); err != nil {
				return exitCode, err
			}
		case *pb.CommandOutput_FormRequest:
			if sender == nil {
				return exitCode, errors.E("form request received without input stream")
			}
			resp, err := renderForm(cli, out.FormRequest)
			if err != nil {
				return exitCode, err
			}
			if err := sender.Send(&pb.CommandInput{
				Input: &pb.CommandInput_FormResponse{FormResponse: resp},
			}); err != nil {
				return exitCode, err
			}
		case *pb.CommandOutput_ExitCode:
			exitCode = out.ExitCode
		default:
		}
	}
	return exitCode, nil
}

type commandInputMode int

const (
	commandInputNone commandInputMode = iota
	commandInputStdin
	commandInputForm
)

func executeCommand(ctx context.Context, cli commands.CLI, client pb.CommandServiceClient, req *pb.CommandRequest, mode commandInputMode) (int32, error) {
	if mode == commandInputNone {
		stream, err := client.ExecuteCommand(ctx, req)
		if err != nil {
			return 0, err
		}
		return consumeCommandStream(stream, nil, cli)
	}

	stream, err := client.ExecuteCommandWithInput(ctx)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return 0, errors.E("plugin does not support interactive commands over gRPC")
		}
		return 0, err
	}

	if err := stream.Send(&pb.CommandInput{
		Input: &pb.CommandInput_Request{Request: req},
	}); err != nil {
		return 0, err
	}

	if mode == commandInputForm {
		defer func() {
			_ = stream.CloseSend()
		}()
		exitCode, err := consumeCommandStream(stream, stream, cli)
		if err != nil {
			return exitCode, err
		}
		return exitCode, nil
	}

	sendErr := make(chan error, 1)
	waitForStdin := !stdinIsTerminal(cli.Stdin())
	go func() {
		sendErr <- forwardStdin(stream, cli.Stdin())
	}()

	exitCode, err := consumeCommandStream(stream, nil, cli)
	if err != nil {
		return exitCode, err
	}
	if waitForStdin {
		if err := <-sendErr; err != nil {
			return exitCode, err
		}
	} else {
		go func() {
			<-sendErr
		}()
	}
	return exitCode, nil
}

func closeStdin(stream pb.CommandService_ExecuteCommandWithInputClient) error {
	if err := stream.Send(&pb.CommandInput{
		Input: &pb.CommandInput_StdinClose{StdinClose: true},
	}); err != nil {
		return err
	}
	return stream.CloseSend()
}

func forwardStdin(stream pb.CommandService_ExecuteCommandWithInputClient, stdin io.Reader) error {
	if stdin == nil {
		return closeStdin(stream)
	}
	buf := make([]byte, 4096)
	for {
		n, err := stdin.Read(buf)
		if n > 0 {
			if err := stream.Send(&pb.CommandInput{
				Input: &pb.CommandInput_Stdin{Stdin: append([]byte{}, buf[:n]...)},
			}); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return closeStdin(stream)
}

func stdinIsTerminal(stdin io.Reader) bool {
	f, ok := stdin.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func writeFile(baseDir string, fw *pb.FileWrite) error {
	if fw == nil {
		return nil
	}
	path := fw.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	path = filepath.Clean(path)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(fw.Mode)
	if mode == 0 {
		mode = 0o644
	}
	return os.WriteFile(path, fw.Content, mode)
}

func startHostService(cli commands.CLI) (string, func(), error) {
	if cli == nil || cli.Engine() == nil {
		return "", func() {}, nil
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	server := grpc.NewServer()
	pb.RegisterHostServiceServer(server, plugingrpc.NewHostService(cli.Engine().Config(), cli.Config().UserTerramateDir))
	go func() {
		_ = server.Serve(lis)
	}()
	return lis.Addr().String(), func() {
		server.Stop()
		_ = lis.Close()
	}, nil
}
