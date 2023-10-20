// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"bufio"
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

		maxLineSize  int
		batchSize    int
		idleDuration time.Duration
	}

	// Syncer is the actual synchronizer callback.
	Syncer func(l DeploymentLogs)
)

// DefaultLogMaxLineSize is the default maximum line.
// TODO(i4k): to be removed.
const DefaultLogMaxLineSize = 4096

// DefaultLogBatchSize is the default batch size.
const DefaultLogBatchSize = 256

// DefaultLogIdleDuration is the maximum idle duration before a sync could happen.
const DefaultLogIdleDuration = 1 * time.Second

// NewLogSyncer creates a new log syncer.
func NewLogSyncer(syncfn Syncer) *LogSyncer {
	return NewLogSyncerWith(syncfn, DefaultLogMaxLineSize, DefaultLogBatchSize, DefaultLogIdleDuration)
}

// NewLogSyncerWith creates a new customizable syncer.
func NewLogSyncerWith(
	syncfn Syncer,
	maxLineSize int,
	batchSize int,
	idleDuration time.Duration,
) *LogSyncer {
	if maxLineSize == 0 {
		panic("max line size must be set")
	}
	l := &LogSyncer{
		in:       make(chan *DeploymentLog, batchSize),
		syncfn:   syncfn,
		shutdown: make(chan struct{}),

		maxLineSize:  maxLineSize,
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
		line := int64(1)
		scanner := bufio.NewScanner(r)
		maxLineSize := s.maxLineSize
		buf := make([]byte, maxLineSize)
		scanner.Buffer(buf, maxLineSize)
		// no internal allocation
		scanner.Split(scanLines)

		errs := errors.L()
		for scanner.Scan() {
			message := scanner.Text()
			_, err := out.Write([]byte(message))
			if err != nil {
				errs.Append(errors.E(err, "writing to terminal"))
				continue
			}

			t := time.Now().UTC()
			s.in <- &DeploymentLog{
				Channel:   channel,
				Line:      line,
				Message:   string(dropCRLN([]byte(message))),
				Timestamp: &t,
			}
			line++
		}
		if err := scanner.Err(); err != nil {
			errs.Append(errors.E(err, "scanning output lines"))
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

// scanLines is a split function for a [bufio.Scanner] that returns each line of
// text. It's similar to [bufio.ScanLines] but do not remove the trailing newline
// marker and optional carriege return. The returned line may be empty.
// The end-of-line marker is one optional carriage return followed
// by one mandatory newline. In regular expression notation, it is `\r?\n`.
// The last non-empty line of input will be returned even if it has no
// newline.
func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[0 : i+1], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
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
