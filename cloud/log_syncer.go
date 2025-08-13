// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"bytes"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/errors"
)

type (
	// BufferGroup manages a group of synchronized buffers.
	BufferGroup struct {
		fds       []io.Closer
		wg        sync.WaitGroup
		logSyncer *LogSyncer
	}

	// LogSyncer synchronizes log lines, typically from a single buffer group.
	LogSyncer struct {
		pending  resources.CommandLogs
		in       chan *resources.CommandLog
		syncfn   SyncFunc
		shutdown chan struct{}

		batchSize    int
		syncInterval time.Duration
	}

	// SyncFunc is the actual synchronizer callback.
	SyncFunc func(l resources.CommandLogs)
)

// NewBufferGroup creates a new buffer group with an optional LogSyncer.
func NewBufferGroup(logSyncer *LogSyncer) *BufferGroup {
	return &BufferGroup{logSyncer: logSyncer}
}

// Wait waits for the processing all output for the buffers within this group.
// If a LogSyncher is attached, it will also wait for it to finish.
// After calling this method, it's not safe to call any other method, as it
// closes the internal channels and shutdown all goroutines.
func (s *BufferGroup) Wait() {
	for _, writerFD := range s.fds {
		// only return an error when readerFD.CloseWithError(err) is called but
		// but this is not the case.
		_ = writerFD.Close()
	}
	s.wg.Wait()

	if s.logSyncer != nil {
		s.logSyncer.Wait()
	}
}

// NewBuffer creates a new synchronized buffer.
func (s *BufferGroup) NewBuffer(channel resources.LogChannel, out io.Writer) io.Writer {
	r, w := io.Pipe()
	s.fds = append(s.fds, w)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		linenum := int64(0)
		syncDisabled := s.logSyncer == nil

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
				linenum++

				_, err := out.Write(line)
				if err != nil {
					errs.Append(errors.E(err, "writing to terminal"))
				}

				if syncDisabled {
					continue
				}

				if !utf8.Valid(line) {
					syncDisabled = true
					errs.Append(errors.E("skipping sync of non-utf8 (%s) output", channel.String()))
					continue
				}

				s.logSyncer.ProcessLine(channel, linenum, line)

			}
			if readErr == io.EOF {
				break
			}
			pending = rest
		}

		errs.Append(r.Close())
		errs.Append(w.Close())
		if err := errs.AsError(); err != nil {
			log.Error().Err(err).Msg("synchronizing command output")
		}
	}()
	return w
}

// DefaultLogBatchSize is the default batch size.
const DefaultLogBatchSize = 256

// DefaultLogSyncInterval is the maximum idle duration before a sync could happen.
const DefaultLogSyncInterval = 1 * time.Second

// NewLogSyncer creates a new log syncer with default parameters.
func NewLogSyncer(syncfn SyncFunc) *LogSyncer {
	return NewLogSyncerWith(syncfn, DefaultLogBatchSize, DefaultLogSyncInterval)
}

// NewLogSyncerWith creates a new customizable syncer.
func NewLogSyncerWith(
	syncfn SyncFunc,
	batchSize int,
	syncInterval time.Duration,
) *LogSyncer {
	l := &LogSyncer{
		in:       make(chan *resources.CommandLog, batchSize),
		syncfn:   syncfn,
		shutdown: make(chan struct{}),

		batchSize:    batchSize,
		syncInterval: syncInterval,
	}
	l.start()

	return l
}

// ProcessLine creates a new synchronized buffer.
func (s *LogSyncer) ProcessLine(channel resources.LogChannel, linenum int64, line []byte) {
	t := time.Now().UTC()
	s.in <- &resources.CommandLog{
		Channel:   channel,
		Line:      linenum,
		Message:   string(dropCRLN([]byte(line))),
		Timestamp: &t,
	}
}

// Wait waits for the processing of all log messages.
// After calling this method, it's not safe to call any other method, as it
// closes the internal channels and shutdown all goroutines.
func (s *LogSyncer) Wait() {
	close(s.in)
	<-s.shutdown
}

func (s *LogSyncer) start() {
	go func() {
		ticker := time.NewTicker(s.syncInterval)
		defer ticker.Stop()

	outer:
		for {
			select {
			case e, ok := <-s.in:
				if !ok {
					break outer
				}
				s.enqueue(e)
			case <-ticker.C:
				s.syncAll()
			}
		}
		s.syncAll()
		s.shutdown <- struct{}{}
	}()
}

func (s *LogSyncer) syncAll() {
	for len(s.pending) > 0 {
		rest := min(s.batchSize, len(s.pending))
		s.syncfn(s.pending[:rest])
		s.pending = s.pending[rest:]
	}
}

func (s *LogSyncer) enqueue(l *resources.CommandLog) {
	s.pending = append(s.pending, l)
	if len(s.pending) >= s.batchSize {
		s.syncAll()
	}
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
