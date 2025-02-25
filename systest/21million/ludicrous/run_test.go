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

package bulk

import (
	"os"
	"path/filepath"

	"github.com/vtta/dgraph/testutil"

	"testing"
)

//
//func TestQueries(t *testing.T) {
//	t.Run("Run queries", common.TestQueriesFor21Million)
//}

func TestMain(m *testing.M) {
	schemaFile := filepath.Join(testutil.TestDataDirectory, "21million.schema")
	rdfFile := filepath.Join(testutil.TestDataDirectory, "21million.rdf.gz")

	if err := testutil.LiveLoad(testutil.LiveOpts{
		Alpha:      testutil.ContainerAddr("alpha1", 9080),
		Zero:       testutil.SockAddrZero,
		RdfFile:    rdfFile,
		SchemaFile: schemaFile,
		Ludicrous:  true,
	}); err != nil {
		cleanupAndExit(1)
	}

	// dont run queries
	//fmt.Print("waiting for the indexes to be completed \n")
	//time.Sleep(10 * time.Minute)
	//exitCode := m.Run()
	cleanupAndExit(0)
}

func cleanupAndExit(exitCode int) {
	_ = os.RemoveAll("./t")
	os.Exit(exitCode)
}
