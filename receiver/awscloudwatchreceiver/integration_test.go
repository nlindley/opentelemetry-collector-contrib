// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestLoggingIntegration(t *testing.T) {
	mc := &mockClient{}
	mc.On("DescribeLogGroupsWithContext", mock.Anything, mock.Anything, mock.Anything).
		Return(loadLogGroups(t), nil)

	mc.On("FilterLogEventsWithContext", mock.Anything, mock.Anything, mock.Anything).
		Return(loadLogEvents(t), nil)

	sink := &consumertest.LogsSink{}
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-east-2"
	cfg.Logs.PollInterval = time.Second
	cfg.Logs.Groups.AutodiscoverConfig = &AutodiscoverConfig{
		Limit: 1,
	}
	recv, err := NewFactory().CreateLogsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		sink,
	)
	require.NoError(t, err)

	rcvr, ok := recv.(*logsReceiver)
	require.True(t, ok)
	rcvr.client = mc

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 5*time.Second, 10*time.Millisecond)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]

	expectedLogs, err := readLogs(filepath.Join("testdata", "golden", "autodiscovered.json"))
	require.NoError(t, err)
	require.NoError(t, compareLogs(expectedLogs, logs))
}

var (
	logGroupFiles = []string{
		filepath.Join("testdata", "log-groups", "group-1.json"),
	}
	logEventsFiles = []string{
		filepath.Join("testdata", "events", "event-1.json"),
	}
)

func loadLogGroups(t *testing.T) *cloudwatchlogs.DescribeLogGroupsOutput {
	var output []*cloudwatchlogs.LogGroup
	for _, lg := range logGroupFiles {
		bytes, err := os.ReadFile(lg)
		require.NoError(t, err)
		var logGroup cloudwatchlogs.LogGroup
		err = json.Unmarshal(bytes, &logGroup)
		require.NoError(t, err)
		output = append(output, &logGroup)
	}

	return &cloudwatchlogs.DescribeLogGroupsOutput{
		LogGroups: output,
		NextToken: nil,
	}
}

func loadLogEvents(t *testing.T) *cloudwatchlogs.FilterLogEventsOutput {
	var output []*cloudwatchlogs.FilteredLogEvent
	for _, lg := range logEventsFiles {
		bytes, err := os.ReadFile(lg)
		require.NoError(t, err)
		var event cloudwatchlogs.FilteredLogEvent
		err = json.Unmarshal(bytes, &event)
		require.NoError(t, err)
		output = append(output, &event)
	}

	return &cloudwatchlogs.FilterLogEventsOutput{
		Events:    output,
		NextToken: nil,
	}
}
