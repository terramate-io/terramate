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
)

type (
	write struct {
		channel cloud.LogChannel
		data    []byte
		idle    time.Duration
	}
	want struct {
		output  map[cloud.LogChannel][]byte
		batches []cloud.CommandLogs
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
					channel: cloud.StdoutLogChannel,
					data:    hugeLine,
				},
			},
			want: want{
				output: map[cloud.LogChannel][]byte{
					cloud.StdoutLogChannel: hugeLine,
				},
				batches: []cloud.CommandLogs{
					{
						{
							Line:    1,
							Channel: cloud.StdoutLogChannel,
							Message: string(hugeLine),
						},
					},
				},
			},
		},
		{
			name: "multiple writes with no newline",
			writes: []write{
				{channel: cloud.StdoutLogChannel, data: []byte("terramate ")},
				{channel: cloud.StdoutLogChannel, data: []byte("cloud ")},
				{channel: cloud.StdoutLogChannel, data: []byte("is ")},
				{channel: cloud.StdoutLogChannel, data: []byte("amazing")},
			},
			want: want{
				output: map[cloud.LogChannel][]byte{
					cloud.StdoutLogChannel: []byte("terramate cloud is amazing"),
				},
				batches: []cloud.CommandLogs{
					{
						{
							Line:    1,
							Channel: cloud.StdoutLogChannel,
							Message: "terramate cloud is amazing",
						},
					},
				},
			},
		},
		{
			name: "multiple writes with newlines",
			writes: []write{
				{channel: cloud.StdoutLogChannel, data: []byte("terramate\ncloud\n")},
				{channel: cloud.StdoutLogChannel, data: []byte("is\namazing")},
			},
			want: want{
				output: map[cloud.LogChannel][]byte{
					cloud.StdoutLogChannel: []byte("terramate\ncloud\nis\namazing"),
				},
				batches: []cloud.CommandLogs{
					{
						{
							Line:    1,
							Channel: cloud.StdoutLogChannel,
							Message: "terramate",
						},
						{
							Line:    2,
							Channel: cloud.StdoutLogChannel,
							Message: "cloud",
						},
						{
							Line:    3,
							Channel: cloud.StdoutLogChannel,
							Message: "is",
						},
						{
							Line:    4,
							Channel: cloud.StdoutLogChannel,
							Message: "amazing",
						},
					},
				},
			},
		},
		{
			name: "empty line -- regression check",
			writes: []write{
				{channel: cloud.StdoutLogChannel, data: []byte("\n")},
			},
			want: want{
				output: map[cloud.LogChannel][]byte{
					cloud.StdoutLogChannel: []byte("\n"),
				},
				batches: []cloud.CommandLogs{
					{
						{
							Line:    1,
							Channel: cloud.StdoutLogChannel,
							Message: "",
						},
					},
				},
			},
		},
		{
			name: "multiple writes with CRLN",
			writes: []write{
				{channel: cloud.StdoutLogChannel, data: []byte("terramate\r\ncloud\r\n")},
				{channel: cloud.StdoutLogChannel, data: []byte("is\r\namazing")},
			},
			want: want{
				output: map[cloud.LogChannel][]byte{
					cloud.StdoutLogChannel: []byte("terramate\r\ncloud\r\nis\r\namazing"),
				},
				batches: []cloud.CommandLogs{
					{
						{
							Line:    1,
							Channel: cloud.StdoutLogChannel,
							Message: "terramate",
						},
						{
							Line:    2,
							Channel: cloud.StdoutLogChannel,
							Message: "cloud",
						},
						{
							Line:    3,
							Channel: cloud.StdoutLogChannel,
							Message: "is",
						},
						{
							Line:    4,
							Channel: cloud.StdoutLogChannel,
							Message: "amazing",
						},
					},
				},
			},
		},
		{
			name: "mix of stdout and stderr writes",
			writes: []write{
				{channel: cloud.StdoutLogChannel, data: []byte("A\nB\n")},
				{channel: cloud.StderrLogChannel, data: []byte("C\nD\n")},
				{channel: cloud.StdoutLogChannel, data: []byte("E\nF")},
				{channel: cloud.StderrLogChannel, data: []byte("G\nH")},
			},
			want: want{
				output: map[cloud.LogChannel][]byte{
					cloud.StdoutLogChannel: []byte("A\nB\nE\nF"),
					cloud.StderrLogChannel: []byte("C\nD\nG\nH"),
				},
				batches: []cloud.CommandLogs{
					{
						{
							Line:    1,
							Channel: cloud.StdoutLogChannel,
							Message: "A",
						},
						{
							Line:    2,
							Channel: cloud.StdoutLogChannel,
							Message: "B",
						},
						{
							Line:    1,
							Channel: cloud.StderrLogChannel,
							Message: "C",
						},
						{
							Line:    2,
							Channel: cloud.StderrLogChannel,
							Message: "D",
						},
						{
							Line:    3,
							Channel: cloud.StdoutLogChannel,
							Message: "E",
						},
						{
							Line:    4,
							Channel: cloud.StdoutLogChannel,
							Message: "F",
						},
						{
							Line:    3,
							Channel: cloud.StderrLogChannel,
							Message: "G",
						},
						{
							Line:    4,
							Channel: cloud.StderrLogChannel,
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
				{channel: cloud.StdoutLogChannel, data: []byte("A\n")},
				{channel: cloud.StdoutLogChannel, data: []byte("B\nC\n")},
				{channel: cloud.StdoutLogChannel, data: []byte("D\nE\nF\nG\n")},
			},
			want: want{
				output: map[cloud.LogChannel][]byte{
					cloud.StdoutLogChannel: []byte("A\nB\nC\nD\nE\nF\nG\n"),
				},
				batches: []cloud.CommandLogs{
					{
						{
							Line:    1,
							Channel: cloud.StdoutLogChannel,
							Message: "A",
						},
					},
					{
						{
							Line:    2,
							Channel: cloud.StdoutLogChannel,
							Message: "B",
						},
					},
					{
						{
							Line:    3,
							Channel: cloud.StdoutLogChannel,
							Message: "C",
						},
					},
					{
						{
							Line:    4,
							Channel: cloud.StdoutLogChannel,
							Message: "D",
						},
					},
					{
						{
							Line:    5,
							Channel: cloud.StdoutLogChannel,
							Message: "E",
						},
					},
					{
						{
							Line:    6,
							Channel: cloud.StdoutLogChannel,
							Message: "F",
						},
					},
					{
						{
							Line:    7,
							Channel: cloud.StdoutLogChannel,
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
				{channel: cloud.StdoutLogChannel, data: []byte("first write\n")},
				{
					// make this lower than tc.idleDuration to fail the test.
					idle:    300 * time.Millisecond,
					channel: cloud.StdoutLogChannel,
					data:    []byte("write after idle time\n"),
				},
				{channel: cloud.StdoutLogChannel, data: []byte("another\nmultiline\nwrite\nhere")},
			},
			want: want{
				output: map[cloud.LogChannel][]byte{
					cloud.StdoutLogChannel: []byte("first write\nwrite after idle time\nanother\nmultiline\nwrite\nhere"),
				},
				batches: []cloud.CommandLogs{
					// first batch is due to sync interval trigger.
					{
						{
							Line:    1,
							Channel: cloud.StdoutLogChannel,
							Message: "first write",
						},
					},
					{
						{
							Line:    2,
							Channel: cloud.StdoutLogChannel,
							Message: "write after idle time",
						},
						{
							Line:    3,
							Channel: cloud.StdoutLogChannel,
							Message: "another",
						},
						{
							Line:    4,
							Channel: cloud.StdoutLogChannel,
							Message: "multiline",
						},
						{
							Line:    5,
							Channel: cloud.StdoutLogChannel,
							Message: "write",
						},
						{
							Line:    6,
							Channel: cloud.StdoutLogChannel,
							Message: "here",
						},
					},
				},
			},
		},
		{
			name: "disable sync in case non-utf8 output is detected",
			writes: []write{
				{channel: cloud.StdoutLogChannel, data: []byte("valid\n")},
				{channel: cloud.StdoutLogChannel, data: []byte{0x80, 1, 2, 3, 4, '\n'}},
				{channel: cloud.StdoutLogChannel, data: []byte("another valid")},
			},
			want: want{
				output: map[cloud.LogChannel][]byte{
					cloud.StdoutLogChannel: {
						'v', 'a', 'l', 'i', 'd', '\n',
						0x80, 1, 2, 3, 4, '\n',
						'a', 'n', 'o', 't', 'h', 'e', 'r', ' ', 'v', 'a', 'l', 'i', 'd',
					},
				},
				batches: []cloud.CommandLogs{
					{
						// sync is disabled at first non-utf8 sequence.
						{
							Line:    1,
							Channel: cloud.StdoutLogChannel,
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
			var gotBatches []cloud.CommandLogs
			s := cloud.NewLogSyncerWith(func(logs cloud.CommandLogs) {
				gotBatches = append(gotBatches, logs)
			}, tc.batchSize, tc.syncInterval)
			var stdoutBuf, stderrBuf bytes.Buffer
			stdoutProxy := s.NewBuffer(cloud.StdoutLogChannel, &stdoutBuf)
			stderrProxy := s.NewBuffer(cloud.StderrLogChannel, &stderrBuf)

			writeFinished := make(chan struct{})
			go func() {
				fdmap := map[cloud.LogChannel]io.Writer{
					cloud.StdoutLogChannel: stdoutProxy,
					cloud.StderrLogChannel: stderrProxy,
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

			var gotOutputs map[cloud.LogChannel][]byte
			stdoutBytes := stdoutBuf.Bytes()
			stderrBytes := stderrBuf.Bytes()
			if len(stdoutBytes) > 0 || len(stderrBytes) > 0 {
				gotOutputs = map[cloud.LogChannel][]byte{}
			}
			if len(stdoutBytes) > 0 {
				gotOutputs[cloud.StdoutLogChannel] = stdoutBytes
			}
			if len(stderrBytes) > 0 {
				gotOutputs[cloud.StderrLogChannel] = stderrBytes
			}
			if diff := cmp.Diff(gotOutputs, tc.want.output); diff != "" {
				t.Logf("want stdout:%s", tc.want.output[cloud.StdoutLogChannel])
				t.Logf("got stdout:%s", gotOutputs[cloud.StdoutLogChannel])
				t.Logf("want stderr:%s", tc.want.output[cloud.StderrLogChannel])
				t.Logf("got stderr:%s", gotOutputs[cloud.StderrLogChannel])

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

func compareBatches(t *testing.T, got, want []cloud.CommandLogs) {
	assert.EqualInts(t, len(want), len(got), "number of batches mismatch")

	wantStdoutLogs, wantStderrLogs := divideBatches(want)
	gotStdoutLogs, gotStderrLogs := divideBatches(got)
	if diff := cmp.Diff(wantStdoutLogs, gotStdoutLogs, cmpopts.IgnoreFields(cloud.CommandLog{}, "Timestamp")); diff != "" {
		t.Fatalf("log stdout mismatch: %s", diff)
	}
	if diff := cmp.Diff(wantStderrLogs, gotStderrLogs, cmpopts.IgnoreFields(cloud.CommandLog{}, "Timestamp")); diff != "" {
		t.Fatalf("log stderr mismatch: %s", diff)
	}
}

func divideBatches(batches []cloud.CommandLogs) (stdoutLogs, stderrLogs cloud.CommandLogs) {
	for _, batch := range batches {
		for _, log := range batch {
			if log.Channel == cloud.StdoutLogChannel {
				stdoutLogs = append(stdoutLogs, log)
			} else {
				stderrLogs = append(stderrLogs, log)
			}
		}
	}
	return stdoutLogs, stderrLogs
}
