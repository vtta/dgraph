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
	"encoding/json"
	"math"

	"github.com/vtta/dgraph/graphql/resolve"
	"github.com/vtta/dgraph/graphql/schema"
	"github.com/vtta/dgraph/protos/pb"
	"github.com/vtta/dgraph/worker"
	"github.com/vtta/dgraph/x"
	"github.com/golang/glog"
	"github.com/pkg/errors"
)

const notSet = math.MaxInt64

type exportInput struct {
	Format    string
	Namespace int64
	DestinationFields
}

func resolveExport(ctx context.Context, m schema.Mutation) (*resolve.Resolved, bool) {
	glog.Info("Got export request through GraphQL admin API")

	input, err := getExportInput(m)
	if err != nil {
		return resolve.EmptyResult(m, err), false
	}

	format := worker.DefaultExportFormat
	if input.Format != "" {
		format = worker.NormalizeExportFormat(input.Format)
		if format == "" {
			return resolve.EmptyResult(m, errors.Errorf("invalid export format: %v", input.Format)), false
		}
	}

	validateAndGetNs := func(inputNs int64) (uint64, error) {
		ns, err := x.ExtractNamespace(ctx)
		if err != nil {
			return 0, err
		}
		if input.Namespace == notSet {
			// If namespace parameter is not set, use the namespace from the context.
			return ns, nil
		}
		switch ns {
		case x.GalaxyNamespace:
			if input.Namespace < 0 { // export all namespaces.
				return math.MaxUint64, nil
			}
			return uint64(inputNs), nil
		default:
			if input.Namespace != notSet && uint64(input.Namespace) != ns {
				return 0, errors.Errorf("not allowed to export namespace %#x", input.Namespace)
			}
		}
		return ns, nil
	}

	var exportNs uint64
	if exportNs, err = validateAndGetNs(input.Namespace); err != nil {
		return resolve.EmptyResult(m, err), false
	}

	req := &pb.ExportRequest{
		Format:       format,
		Namespace:    exportNs,
		Destination:  input.Destination,
		AccessKey:    input.AccessKey,
		SecretKey:    input.SecretKey,
		SessionToken: input.SessionToken,
		Anonymous:    input.Anonymous,
	}

	files, err := worker.ExportOverNetwork(context.Background(), req)
	if err != nil {
		return resolve.EmptyResult(m, err), false
	}

	data := response("Success", "Export completed.")
	data["exportedFiles"] = toInterfaceSlice(files)
	return resolve.DataResult(
		m,
		map[string]interface{}{m.Name(): data},
		nil,
	), true
}

// toInterfaceSlice converts []string to []interface{}
func toInterfaceSlice(in []string) []interface{} {
	out := make([]interface{}, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}

func getExportInput(m schema.Mutation) (*exportInput, error) {
	inputArg := m.ArgValue(schema.InputArgName)
	inputByts, err := json.Marshal(inputArg)
	if err != nil {
		return nil, schema.GQLWrapf(err, "couldn't get input argument")
	}

	var input exportInput
	err = json.Unmarshal(inputByts, &input)

	// Export everything if namespace is not specified.
	if v, ok := inputArg.(map[string]interface{}); ok {
		if _, ok := v["namespace"]; !ok {
			input.Namespace = notSet
		}
	}
	return &input, schema.GQLWrapf(err, "couldn't get input argument")
}
