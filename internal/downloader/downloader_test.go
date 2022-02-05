// Copyright 2022 Dhi Aurrahman
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

package downloader_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bazelbuild/bazelisk/httputil"
	"github.com/stretchr/testify/require"

	"github.com/dio/pc/internal/archives"
	"github.com/dio/pc/internal/downloader"
)

var (
	seed = time.Now()
)

type fakeClock struct {
	now          time.Time
	SleepPeriods []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: seed}
}

func (fc *fakeClock) Sleep(d time.Duration) {
	fc.now = fc.now.Add(d)
	fc.SleepPeriods = append(fc.SleepPeriods, d)
}

func (fc *fakeClock) Now() time.Time {
	return fc.now
}

func (fc *fakeClock) TimesSlept() int {
	return len(fc.SleepPeriods)
}

func setUp() (*httputil.FakeTransport, *fakeClock) {
	transport := httputil.NewFakeTransport()
	httputil.DefaultTransport = transport

	clock := newFakeClock()
	httputil.RetryClock = clock
	return transport, clock
}

// TestDownloadVersionedBinarySuccessOnFirstTry tests if we can download the archive and get the
// extracted file.
func TestDownloadVersionedBinarySuccessOnFirstTry(t *testing.T) {
	tests := []struct {
		name    string
		archive archives.Archive
	}{
		{
			name:    "auth_server.tar.gz",
			archive: &archives.ExtAuthz{},
		},
		{
			name:    "envoy-v1.12.2-linux-amd64.tar.xz",
			archive: &archives.Proxy{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport, _ := setUp()
			data, err := os.ReadFile(filepath.Join("testdata", test.name))
			require.NoError(t, err)
			transport.AddResponse(downloader.GetArchiveURL(test.archive), 200, string(data), nil)
			downloaded, err := downloader.DownloadVersionedBinary(context.Background(), test.archive, t.TempDir())
			require.NoError(t, err)
			require.FileExists(t, downloaded)
		})
	}
}
