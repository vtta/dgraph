/*
 * Copyright 2022 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package common

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vtta/dgraph/chunker"
	"github.com/vtta/dgraph/testutil"
)

// JSON output can be hundreds of lines and diffs can scroll off the terminal before you
// can look at them. This option allows saving the JSON to a specified directory instead
// for easier reviewing after the test completes.
//var savedir = flag.String("savedir", "",
//	"directory to save json from test failures in")
//var quiet = flag.Bool("quiet", false,
//	"just output whether json differs, not a diff")

func TestQueriesFor21Million(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	queryDir := filepath.Join(filepath.Dir(thisFile), "../queries")

	// For this test we DON'T want to start with an empty database.
	dg, err := testutil.DgraphClient(testutil.ContainerAddr("alpha1", 9080))
	if err != nil {
		t.Fatalf("Error while getting a dgraph client: %v", err)
	}

	files, err := ioutil.ReadDir(queryDir)
	if err != nil {
		t.Fatalf("Error reading directory: %s", err.Error())
	}

	//savepath := ""
	//diffs := 0
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "query-") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			filename := filepath.Join(queryDir, file.Name())
			reader, cleanup := chunker.FileReader(filename, nil)
			bytes, err := ioutil.ReadAll(reader)
			if err != nil {
				t.Fatalf("Error reading file: %s", err.Error())
			}
			contents := string(bytes[:])
			cleanup()

			// The test query and expected result are separated by a delimiter.
			bodies := strings.SplitN(contents, "\n---\n", 2)
			// Dgraph can get into unhealthy state sometime. So, add retry for every query.
			for retry := 0; retry < 3; retry++ {
				// If a query takes too long to run, it probably means dgraph is stuck and there's
				// no point in waiting longer or trying more tests.
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				resp, err := dg.NewTxn().Query(ctx, bodies[0])
				cancel()

				if retry < 2 && (err != nil || ctx.Err() == context.DeadlineExceeded) {
					continue
				}

				if ctx.Err() == context.DeadlineExceeded {
					t.Fatal("aborting test due to query timeout")
				}

				t.Logf("running %s", file.Name())
				//if *savedir != "" {
				//	savepath = filepath.Join(*savedir, file.Name())
				//}

				testutil.CompareJSON(t, bodies[1], string(resp.GetJson()))
			}
		})
	}
	//
	//if *savedir != "" && diffs > 0 {
	//	t.Logf("test json saved in directory: %s", *savedir)
	//}
}
