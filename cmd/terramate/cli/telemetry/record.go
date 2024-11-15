// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package telemetry

import (
	"errors"
	"runtime"
	"slices"
	"sync"
)

// Record is used to aggregate telemetry data over the runtime of the application.
type Record struct {
	sync.Mutex
	msg  *Message
	done <-chan error
}

// NewRecord creates an empty record.
func NewRecord() *Record {
	return &Record{
		msg: &Message{},
	}
}

// Set updates the telemetry data in the record.
// It is safe to be called concurrently.
func (r *Record) Set(opts ...MessageOpt) {
	r.Lock()
	defer r.Unlock()

	// Silently discard any updates if the record is already being sent.
	if r.done != nil {
		return
	}

	for _, opt := range opts {
		opt(r.msg)
	}
}

// MessageOpt is the option type for Set.
type MessageOpt func(msg *Message)

// Command sets the command name.
func Command(cmd string) MessageOpt {
	return func(msg *Message) {
		msg.Command = cmd
	}
}

// Command sets the organization name.
func OrgName(orgName string) MessageOpt {
	return func(msg *Message) {
		msg.OrgName = orgName
	}
}

// DetectFromEnv detects platform, auth type, signature, architecture and OS from the environment.
func DetectFromEnv(credfile, cpsigfile, anasigfile string) MessageOpt {
	return func(msg *Message) {
		msg.Platform = DetectPlatformFromEnv()

		msg.Auth = DetectAuthTypeFromEnv(credfile)
		msg.Signature, _ = GenerateOrReadSignature(cpsigfile, anasigfile)

		msg.Arch = runtime.GOARCH
		msg.OS = runtime.GOOS
	}
}

// BoolFlag sets the given detail name if flag is true.
// If ifCmds is not empty, the current command of the record must also match any of the given values.
func BoolFlag(name string, flag bool, ifCmds ...string) MessageOpt {
	return func(msg *Message) {
		if flag {
			if len(ifCmds) != 0 && !slices.Contains(ifCmds, msg.Command) {
				return
			}
			if slices.Contains(msg.Details, name) {
				return
			}
			msg.Details = append(msg.Details, name)
		}
	}
}

// StringFlag calls BoolFlag with flag = (stringFlag != "").
func StringFlag(name string, flag string, ifCmds ...string) MessageOpt {
	return BoolFlag(name, flag != "", ifCmds...)
}

// Send sends a message for the current record state asynchronously.
// A record can only be sent once, subsequent calls will be ignored.
// The function is non-blocking, the result can be checked with WaitForSend().
func (r *Record) Send(params SendMessageParams) {
	r.Lock()
	defer r.Unlock()

	if r.done == nil {
		r.done = SendMessage(r.msg, params)
	}
}

// WaitForSend waits until Send is done, either successfully, or with error/timeout.
func (r *Record) WaitForSend() error {
	if r.done == nil {
		return errors.New("message was not sent")
	}

	return <-r.done
}

// DefaultRecord is the global default record.
var DefaultRecord = NewRecord()
