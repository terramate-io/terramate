// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/resources"
)

type (
	write struct {
		channel resources.LogChannel
		data    []byte
		idle    time.Duration
	}
	want struct {
		output  map[resources.LogChannel][]byte
		batches []resources.CommandLogs
	}
	testcase struct {
		name         string
		writes       []write
		batchSize    int
		syncInterval time.Duration
		want         want
	}
)

func TestCloudLogSyncer(t *testing.T) {
	t.Parallel()
	hugeLine := bytes.Repeat([]byte{'A'}, 1*1024*1024) // 1 mib line, no line ending

	for _, tc := range []testcase{
		{
			name: "no output",
		},
		{
			name: "unlimited line length",
			writes: []write{
				{
					channel: resources.StdoutLogChannel,
					data:    hugeLine,
				},
			},
			want: want{
				output: map[resources.LogChannel][]byte{
					resources.StdoutLogChannel: hugeLine,
				},
				batches: []resources.CommandLogs{
					{
						{
							Line:    1,
							Channel: resources.StdoutLogChannel,
							Message: string(hugeLine),
						},
					},
				},
			},
		},
		{
			name: "multiple writes with no newline",
			writes: []write{
				{channel: resources.StdoutLogChannel, data: []byte("terramate ")},
				{channel: resources.StdoutLogChannel, data: []byte("cloud ")},
				{channel: resources.StdoutLogChannel, data: []byte("is ")},
				{channel: resources.StdoutLogChannel, data: []byte("amazing")},
			},
			want: want{
				output: map[resources.LogChannel][]byte{
					resources.StdoutLogChannel: []byte("terramate cloud is amazing"),
				},
				batches: []resources.CommandLogs{
					{
						{
							Line:    1,
							Channel: resources.StdoutLogChannel,
							Message: "terramate cloud is amazing",
						},
					},
				},
			},
		},
		{
			name: "multiple writes with newlines",
			writes: []write{
				{channel: resources.StdoutLogChannel, data: []byte("terramate\ncloud\n")},
				{channel: resources.StdoutLogChannel, data: []byte("is\namazing")},
			},
			want: want{
				output: map[resources.LogChannel][]byte{
					resources.StdoutLogChannel: []byte("terramate\ncloud\nis\namazing"),
				},
				batches: []resources.CommandLogs{
					{
						{
							Line:    1,
							Channel: resources.StdoutLogChannel,
							Message: "terramate",
						},
						{
							Line:    2,
							Channel: resources.StdoutLogChannel,
							Message: "cloud",
						},
						{
							Line:    3,
							Channel: resources.StdoutLogChannel,
							Message: "is",
						},
						{
							Line:    4,
							Channel: resources.StdoutLogChannel,
							Message: "amazing",
						},
					},
				},
			},
		},
		{
			name: "empty line -- regression check",
			writes: []write{
				{channel: resources.StdoutLogChannel, data: []byte("\n")},
			},
			want: want{
				output: map[resources.LogChannel][]byte{
					resources.StdoutLogChannel: []byte("\n"),
				},
				batches: []resources.CommandLogs{
					{
						{
							Line:    1,
							Channel: resources.StdoutLogChannel,
							Message: "",
						},
					},
				},
			},
		},
		{
			name: "multiple writes with CRLN",
			writes: []write{
				{channel: resources.StdoutLogChannel, data: []byte("terramate\r\ncloud\r\n")},
				{channel: resources.StdoutLogChannel, data: []byte("is\r\namazing")},
			},
			want: want{
				output: map[resources.LogChannel][]byte{
					resources.StdoutLogChannel: []byte("terramate\r\ncloud\r\nis\r\namazing"),
				},
				batches: []resources.CommandLogs{
					{
						{
							Line:    1,
							Channel: resources.StdoutLogChannel,
							Message: "terramate",
						},
						{
							Line:    2,
							Channel: resources.StdoutLogChannel,
							Message: "cloud",
						},
						{
							Line:    3,
							Channel: resources.StdoutLogChannel,
							Message: "is",
						},
						{
							Line:    4,
							Channel: resources.StdoutLogChannel,
							Message: "amazing",
						},
					},
				},
			},
		},
		{
			name: "mix of stdout and stderr writes",
			writes: []write{
				{channel: resources.StdoutLogChannel, data: []byte("A\nB\n")},
				{channel: resources.StderrLogChannel, data: []byte("C\nD\n")},
				{channel: resources.StdoutLogChannel, data: []byte("E\nF")},
				{channel: resources.StderrLogChannel, data: []byte("G\nH")},
			},
			want: want{
				output: map[resources.LogChannel][]byte{
					resources.StdoutLogChannel: []byte("A\nB\nE\nF"),
					resources.StderrLogChannel: []byte("C\nD\nG\nH"),
				},
				batches: []resources.CommandLogs{
					{
						{
							Line:    1,
							Channel: resources.StdoutLogChannel,
							Message: "A",
						},
						{
							Line:    2,
							Channel: resources.StdoutLogChannel,
							Message: "B",
						},
						{
							Line:    1,
							Channel: resources.StderrLogChannel,
							Message: "C",
						},
						{
							Line:    2,
							Channel: resources.StderrLogChannel,
							Message: "D",
						},
						{
							Line:    3,
							Channel: resources.StdoutLogChannel,
							Message: "E",
						},
						{
							Line:    4,
							Channel: resources.StdoutLogChannel,
							Message: "F",
						},
						{
							Line:    3,
							Channel: resources.StderrLogChannel,
							Message: "G",
						},
						{
							Line:    4,
							Channel: resources.StderrLogChannel,
							Message: "H",
						},
					},
				},
			},
		},
		{
			name:         "batch size is respected",
			batchSize:    1,
			syncInterval: 10 * time.Second, // just to ensure it's not used in slow envs
			writes: []write{
				{channel: resources.StdoutLogChannel, data: []byte("A\n")},
				{channel: resources.StdoutLogChannel, data: []byte("B\nC\n")},
				{channel: resources.StdoutLogChannel, data: []byte("D\nE\nF\nG\n")},
			},
			want: want{
				output: map[resources.LogChannel][]byte{
					resources.StdoutLogChannel: []byte("A\nB\nC\nD\nE\nF\nG\n"),
				},
				batches: []resources.CommandLogs{
					{
						{
							Line:    1,
							Channel: resources.StdoutLogChannel,
							Message: "A",
						},
					},
					{
						{
							Line:    2,
							Channel: resources.StdoutLogChannel,
							Message: "B",
						},
					},
					{
						{
							Line:    3,
							Channel: resources.StdoutLogChannel,
							Message: "C",
						},
					},
					{
						{
							Line:    4,
							Channel: resources.StdoutLogChannel,
							Message: "D",
						},
					},
					{
						{
							Line:    5,
							Channel: resources.StdoutLogChannel,
							Message: "E",
						},
					},
					{
						{
							Line:    6,
							Channel: resources.StdoutLogChannel,
							Message: "F",
						},
					},
					{
						{
							Line:    7,
							Channel: resources.StdoutLogChannel,
							Message: "G",
						},
					},
				},
			},
		},
		{
			name:         "if no write happens after configured idle duration then pending data is synced",
			batchSize:    6,
			syncInterval: 100 * time.Millisecond,
			writes: []write{
				{channel: resources.StdoutLogChannel, data: []byte("first write\n")},
				{
					// make this lower than tc.idleDuration to fail the test.
					idle:    300 * time.Millisecond,
					channel: resources.StdoutLogChannel,
					data:    []byte("write after idle time\n"),
				},
				{channel: resources.StdoutLogChannel, data: []byte("another\nmultiline\nwrite\nhere")},
			},
			want: want{
				output: map[resources.LogChannel][]byte{
					resources.StdoutLogChannel: []byte("first write\nwrite after idle time\nanother\nmultiline\nwrite\nhere"),
				},
				batches: []resources.CommandLogs{
					// first batch is due to sync interval trigger.
					{
						{
							Line:    1,
							Channel: resources.StdoutLogChannel,
							Message: "first write",
						},
					},
					{
						{
							Line:    2,
							Channel: resources.StdoutLogChannel,
							Message: "write after idle time",
						},
						{
							Line:    3,
							Channel: resources.StdoutLogChannel,
							Message: "another",
						},
						{
							Line:    4,
							Channel: resources.StdoutLogChannel,
							Message: "multiline",
						},
						{
							Line:    5,
							Channel: resources.StdoutLogChannel,
							Message: "write",
						},
						{
							Line:    6,
							Channel: resources.StdoutLogChannel,
							Message: "here",
						},
					},
				},
			},
		},
		{
			name: "disable sync in case non-utf8 output is detected",
			writes: []write{
				{channel: resources.StdoutLogChannel, data: []byte("valid\n")},
				{channel: resources.StdoutLogChannel, data: []byte{0x80, 1, 2, 3, 4, '\n'}},
				{channel: resources.StdoutLogChannel, data: []byte("another valid")},
			},
			want: want{
				output: map[resources.LogChannel][]byte{
					resources.StdoutLogChannel: {
						'v', 'a', 'l', 'i', 'd', '\n',
						0x80, 1, 2, 3, 4, '\n',
						'a', 'n', 'o', 't', 'h', 'e', 'r', ' ', 'v', 'a', 'l', 'i', 'd',
					},
				},
				batches: []resources.CommandLogs{
					{
						// sync is disabled at first non-utf8 sequence.
						{
							Line:    1,
							Channel: resources.StdoutLogChannel,
							Message: "valid",
						},
					},
				},
			},
		},
	} {
		tc := tc
		tc.validate(t)
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotBatches []resources.CommandLogs
			s := cloud.NewLogSyncerWith(func(logs resources.CommandLogs) {
				gotBatches = append(gotBatches, logs)
			}, tc.batchSize, tc.syncInterval)
			var stdoutBuf, stderrBuf bytes.Buffer
			stdoutProxy := s.NewBuffer(resources.StdoutLogChannel, &stdoutBuf)
			stderrProxy := s.NewBuffer(resources.StderrLogChannel, &stderrBuf)

			writeFinished := make(chan struct{})
			go func() {
				fdmap := map[resources.LogChannel]io.Writer{
					resources.StdoutLogChannel: stdoutProxy,
					resources.StderrLogChannel: stderrProxy,
				}
				for _, write := range tc.writes {
					time.Sleep(write.idle)
					n, err := fdmap[write.channel].Write(write.data)
					assert.NoError(t, err)
					assert.EqualInts(t, len(write.data), n)
				}
				writeFinished <- struct{}{}
			}()

			<-writeFinished
			s.Wait()

			var gotOutputs map[resources.LogChannel][]byte
			stdoutBytes := stdoutBuf.Bytes()
			stderrBytes := stderrBuf.Bytes()
			if len(stdoutBytes) > 0 || len(stderrBytes) > 0 {
				gotOutputs = map[resources.LogChannel][]byte{}
			}
			if len(stdoutBytes) > 0 {
				gotOutputs[resources.StdoutLogChannel] = stdoutBytes
			}
			if len(stderrBytes) > 0 {
				gotOutputs[resources.StderrLogChannel] = stderrBytes
			}
			if diff := cmp.Diff(gotOutputs, tc.want.output); diff != "" {
				t.Logf("want stdout:%s", tc.want.output[resources.StdoutLogChannel])
				t.Logf("got stdout:%s", gotOutputs[resources.StdoutLogChannel])
				t.Logf("want stderr:%s", tc.want.output[resources.StderrLogChannel])
				t.Logf("got stderr:%s", gotOutputs[resources.StderrLogChannel])

				t.Fatal(diff)
			}

			compareBatches(t, gotBatches, tc.want.batches)
		})
	}
}

func (tc *testcase) validate(t *testing.T) {
	if tc.name == "" {
		t.Fatalf("testcase without name: %+v", tc)
	}
	if tc.batchSize == 0 {
		tc.batchSize = cloud.DefaultLogBatchSize
	}
	if tc.syncInterval == 0 {
		tc.syncInterval = 1 * time.Second
	}
}

func compareBatches(t *testing.T, got, want []resources.CommandLogs) {
	assert.EqualInts(t, len(want), len(got), "number of batches mismatch")

	wantStdoutLogs, wantStderrLogs := divideBatches(want)
	gotStdoutLogs, gotStderrLogs := divideBatches(got)
	if diff := cmp.Diff(wantStdoutLogs, gotStdoutLogs, cmpopts.IgnoreFields(resources.CommandLog{}, "Timestamp")); diff != "" {
		t.Fatalf("log stdout mismatch: %s", diff)
	}
	if diff := cmp.Diff(wantStderrLogs, gotStderrLogs, cmpopts.IgnoreFields(resources.CommandLog{}, "Timestamp")); diff != "" {
		t.Fatalf("log stderr mismatch: %s", diff)
	}
}

func divideBatches(batches []resources.CommandLogs) (stdoutLogs, stderrLogs resources.CommandLogs) {
	for _, batch := range batches {
		for _, log := range batch {
			if log.Channel == resources.StdoutLogChannel {
				stdoutLogs = append(stdoutLogs, log)
			} else {
				stderrLogs = append(stderrLogs, log)
			}
		}
	}
	return stdoutLogs, stderrLogs
}
