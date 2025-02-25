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

package admin

import (
	"context"

	"github.com/vtta/dgraph/graphql/resolve"
	"github.com/vtta/dgraph/graphql/schema"
	"github.com/vtta/dgraph/x"
	"github.com/golang/glog"
)

func resolveShutdown(ctx context.Context, m schema.Mutation) (*resolve.Resolved, bool) {
	glog.Info("Got shutdown request through GraphQL admin API")

	x.ServerCloser.Signal()

	return resolve.DataResult(
		m,
		map[string]interface{}{m.Name(): response("Success", "Server is shutting down")},
		nil,
	), true
}
