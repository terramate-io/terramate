// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
)

type (
	// LogSyncer is the log syncer controller type.
	LogSyncer struct {
		pending      DeploymentLogs
		fds          []io.Closer
		in           chan *DeploymentLog
		lastEnqueued time.Time
		syncfn       Syncer
		wg           sync.WaitGroup
		shutdown     chan struct{}

		batchSize    int
		idleDuration time.Duration
	}

	// Syncer is the actual synchronizer callback.
	Syncer func(l DeploymentLogs)
)

// DefaultLogBatchSize is the default batch size.
const DefaultLogBatchSize = 256

// DefaultLogIdleDuration is the maximum idle duration before a sync could happen.
const DefaultLogIdleDuration = 1 * time.Second

// NewLogSyncer creates a new log syncer.
func NewLogSyncer(syncfn Syncer) *LogSyncer {
	return NewLogSyncerWith(syncfn, DefaultLogBatchSize, DefaultLogIdleDuration)
}

// NewLogSyncerWith creates a new customizable syncer.
func NewLogSyncerWith(
	syncfn Syncer,
	batchSize int,
	idleDuration time.Duration,
) *LogSyncer {
	l := &LogSyncer{
		in:       make(chan *DeploymentLog, batchSize),
		syncfn:   syncfn,
		shutdown: make(chan struct{}),

		batchSize:    batchSize,
		idleDuration: idleDuration,
	}
	l.start()
	return l
}

// NewBuffer creates a new synchronized buffer.
func (s *LogSyncer) NewBuffer(channel LogChannel, out io.Writer) io.Writer {
	r, w := io.Pipe()
	s.fds = append(s.fds, w)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		linenum := int64(1)

		var pending []byte
		errs := errors.L()
		for {
			lines, rest, readErr := readLines(r, pending)
			if readErr != nil && readErr != io.EOF {
				errs.Append(readErr)
				break
			}
			if readErr == io.EOF && len(rest) > 0 {
				lines = [][]byte{rest}
			}
			for _, line := range lines {
				_, err := out.Write(line)
				if err != nil {
					errs.Append(errors.E(err, "writing to terminal"))
				}

				t := time.Now().UTC()
				s.in <- &DeploymentLog{
					Channel:   channel,
					Line:      linenum,
					Message:   string(dropCRLN([]byte(line))),
					Timestamp: &t,
				}
				linenum++
			}
			if readErr == io.EOF {
				break
			}
			pending = rest
		}

		errs.Append(r.Close())
		errs.Append(w.Close())
		if err := errs.AsError(); err != nil {
			log.Error().Err(err).Msg("synchroning command output")
		}
	}()
	return w
}

// Wait waits for the processing of all log messages.
// After calling this method, it's not safe to call any other method, as it
// closes the internal channels and shutdown all goroutines.
func (s *LogSyncer) Wait() {
	for _, writerFD := range s.fds {
		// only return an error when readerFD.CloseWithError(err) is called but
		// but this is not the case.
		_ = writerFD.Close()
	}
	s.wg.Wait()
	close(s.in)
	<-s.shutdown
}

func (s *LogSyncer) start() {
	go func() {
		s.lastEnqueued = time.Now()
		for e := range s.in {
			s.enqueue(e)
		}
		for len(s.pending) > 0 {
			rest := min(s.batchSize, len(s.pending))
			s.syncfn(s.pending[:rest])
			s.pending = s.pending[rest:]
		}
		s.shutdown <- struct{}{}
	}()
}

func (s *LogSyncer) enqueue(l *DeploymentLog) {
	s.pending = append(s.pending, l)
	if len(s.pending) >= s.batchSize ||
		(len(s.pending) > 0 && time.Since(s.lastEnqueued) > s.idleDuration) {
		rest := min(s.batchSize, len(s.pending))
		s.syncfn(s.pending[:rest])
		s.pending = s.pending[rest:]
	}
	s.lastEnqueued = time.Now()
}

func readLines(r io.Reader, pending []byte) (line [][]byte, rest []byte, err error) {
	const readSize = 1024

	var buf [readSize]byte
	rest = pending
	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			rest = append(rest, buf[:n]...)
			var lines [][]byte

			var nlpos int
			for nlpos != -1 {
				nlpos = bytes.IndexByte(rest, '\n')
				if nlpos >= 0 {
					lines = append(lines, rest[:nlpos+1]) // line includes ln
					rest = rest[nlpos+1:]
				}
			}
			if len(lines) > 0 {
				return lines, rest, err
			}
		} else if err == nil {
			// misbehaving reader
			return nil, nil, io.EOF
		}
		if err != nil {
			return nil, rest, err
		}

		// line ending not found, continue reading.
	}
}

func readLine(r io.Reader) (line []byte, err error) {
	var buf [1]byte
	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			b := buf[0]
			line = append(line, b)
			if b == '\n' {
				return line, err
			}
		} else if err == nil {
			// misbehaving reader
			return nil, io.EOF
		}
		if err != nil {
			return line, err
		}
	}
}

// dropCRLN drops a terminating \n and \r from the data.
func dropCRLN(data []byte) []byte {
	data = dropByte(data, '\n')
	data = dropByte(data, '\r')
	return data
}

func dropByte(data []byte, b byte) []byte {
	if len(data) == 0 {
		return data
	}
	if data[len(data)-1] == b {
		data = data[0 : len(data)-1]
	}
	return data
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
