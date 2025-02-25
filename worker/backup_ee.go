// +build !oss

/*
 * Copyright 2022 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Dgraph Community License (the "License"); you
 * may not use this file except in compliance with the License. You
 * may obtain a copy of the License at
 *
 *     https://github.com/vtta/dgraph/blob/master/licenses/DCL.txt
 */

package worker

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/vtta/dgraph/posting"
	"github.com/vtta/dgraph/protos/pb"
	"github.com/vtta/dgraph/x"
	ostats "go.opencensus.io/stats"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

// Backup handles a request coming from another node.
func (w *grpcWorker) Backup(ctx context.Context, req *pb.BackupRequest) (*pb.BackupResponse, error) {
	glog.V(2).Infof("Received backup request via Grpc: %+v", req)
	return backupCurrentGroup(ctx, req)
}

func backupCurrentGroup(ctx context.Context, req *pb.BackupRequest) (*pb.BackupResponse, error) {
	glog.Infof("Backup request: group %d at %d", req.GroupId, req.ReadTs)
	if err := ctx.Err(); err != nil {
		glog.Errorf("Context error during backup: %v\n", err)
		return nil, err
	}

	g := groups()
	if g.groupId() != req.GroupId {
		return nil, errors.Errorf("Backup request group mismatch. Mine: %d. Requested: %d\n",
			g.groupId(), req.GroupId)
	}

	if err := posting.Oracle().WaitForTs(ctx, req.ReadTs); err != nil {
		return nil, err
	}

	closer, err := g.Node.startTaskAtTs(opBackup, req.ReadTs)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot start backup operation")
	}
	defer closer.Done()

	bp := NewBackupProcessor(pstore, req)
	defer bp.Close()

	return bp.WriteBackup(closer.Ctx())
}

// BackupGroup backs up the group specified in the backup request.
func BackupGroup(ctx context.Context, in *pb.BackupRequest) (*pb.BackupResponse, error) {
	glog.V(2).Infof("Sending backup request: %+v\n", in)
	if groups().groupId() == in.GroupId {
		return backupCurrentGroup(ctx, in)
	}

	// This node is not part of the requested group, send the request over the network.
	pl := groups().AnyServer(in.GroupId)
	if pl == nil {
		return nil, errors.Errorf("Couldn't find a server in group %d", in.GroupId)
	}
	res, err := pb.NewWorkerClient(pl.Get()).Backup(ctx, in)
	if err != nil {
		glog.Errorf("Backup error group %d: %s", in.GroupId, err)
		return nil, err
	}

	return res, nil
}

// backupLock is used to synchronize backups to avoid more than one backup request
// to be processed at the same time. Multiple requests could lead to multiple
// backups with the same backupNum in their manifest.
var backupLock sync.Mutex

// BackupRes is used to represent the response and error of the Backup gRPC call together to be
// transported via a channel.
type BackupRes struct {
	res *pb.BackupResponse
	err error
}

func ProcessBackupRequest(ctx context.Context, req *pb.BackupRequest) error {
	if err := x.HealthCheck(); err != nil {
		glog.Errorf("Backup canceled, not ready to accept requests: %s", err)
		return err
	}

	// Grab the lock here to avoid more than one request to be processed at the same time.
	backupLock.Lock()
	defer backupLock.Unlock()

	backupSuccessful := false
	ostats.Record(ctx, x.NumBackups.M(1), x.PendingBackups.M(1))
	defer func() {
		if backupSuccessful {
			ostats.Record(ctx, x.NumBackupsSuccess.M(1), x.PendingBackups.M(-1))
		} else {
			ostats.Record(ctx, x.NumBackupsFailed.M(1), x.PendingBackups.M(-1))
		}
	}()

	ts, err := Timestamps(ctx, &pb.Num{ReadOnly: true})
	if err != nil {
		glog.Errorf("Unable to retrieve readonly timestamp for backup: %s", err)
		return err
	}

	req.ReadTs = ts.ReadOnly
	req.UnixTs = time.Now().UTC().Format("20060102.150405.000")

	// Read the manifests to get the right timestamp from which to start the backup.
	uri, err := url.Parse(req.Destination)
	if err != nil {
		return err
	}
	handler, err := NewUriHandler(uri, GetCredentialsFromRequest(req))
	if err != nil {
		return err
	}
	latestManifest, err := handler.GetLatestManifest(uri)
	if err != nil {
		return err
	}

	req.SinceTs = latestManifest.ValidReadTs()

	// To force a full backup we'll set the sinceTs to zero.
	if req.ForceFull {
		req.SinceTs = 0
	} else {
		if x.WorkerConfig.EncryptionKey != nil {
			// If encryption key given, latest backup should be encrypted.
			if latestManifest.Type != "" && !latestManifest.Encrypted {
				err = errors.Errorf("latest manifest indicates the last backup was not encrypted " +
					"but this instance has encryption turned on. Try \"forceFull\" flag.")
				return err
			}
		} else {
			// If encryption turned off, latest backup should be unencrypted.
			if latestManifest.Type != "" && latestManifest.Encrypted {
				err = errors.Errorf("latest manifest indicates the last backup was encrypted " +
					"but this instance has encryption turned off. Try \"forceFull\" flag.")
				return err
			}
		}
	}

	// Update the membership state to get the latest mapping of groups to predicates.
	if err := UpdateMembershipState(ctx); err != nil {
		return err
	}

	// Get the current membership state and parse it for easier processing.
	state := GetMembershipState()
	var groups []uint32
	predMap := make(map[uint32][]string)
	for gid, group := range state.Groups {
		groups = append(groups, gid)
		predMap[gid] = make([]string, 0)
		for pred := range group.Tablets {
			predMap[gid] = append(predMap[gid], pred)
		}
	}

	glog.Infof(
		"Created backup request: read_ts:%d since_ts:%d unix_ts:\"%s\" destination:\"%s\" . Groups=%v\n",
		req.ReadTs,
		req.SinceTs,
		req.UnixTs,
		req.Destination,
		groups,
	)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var dropOperations []*pb.DropOperation
	{ // This is the code which sends out Backup requests and waits for them to finish.
		resCh := make(chan BackupRes, len(state.Groups))
		for _, gid := range groups {
			br := proto.Clone(req).(*pb.BackupRequest)
			br.GroupId = gid
			br.Predicates = predMap[gid]
			go func(req *pb.BackupRequest) {
				res, err := BackupGroup(ctx, req)
				resCh <- BackupRes{res: res, err: err}
			}(br)
		}

		for range groups {
			if backupRes := <-resCh; backupRes.err != nil {
				glog.Errorf("Error received during backup: %v", backupRes.err)
				return backupRes.err
			} else {
				dropOperations = append(dropOperations, backupRes.res.GetDropOperations()...)
			}
		}
	}

	dir := fmt.Sprintf(backupPathFmt, req.UnixTs)
	m := Manifest{
		ReadTs:         req.ReadTs,
		Groups:         predMap,
		Version:        x.DgraphVersion,
		DropOperations: dropOperations,
		Path:           dir,
		Compression:    "snappy",
	}
	if req.SinceTs == 0 {
		m.Type = "full"
		m.BackupId = x.GetRandomName(1)
		m.BackupNum = 1
	} else {
		m.Type = "incremental"
		m.BackupId = latestManifest.BackupId
		m.BackupNum = latestManifest.BackupNum + 1
	}
	m.Encrypted = (x.WorkerConfig.EncryptionKey != nil)

	bp := NewBackupProcessor(nil, req)
	defer bp.Close()
	err = bp.CompleteBackup(ctx, &m)

	if err != nil {
		return err
	}

	backupSuccessful = true
	return nil
}

func ProcessListBackups(ctx context.Context, location string, creds *x.MinioCredentials) (
	[]*Manifest, error) {

	manifests, err := ListBackupManifests(location, creds)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read manifests at location %s", location)
	}

	res := make([]*Manifest, 0)
	for _, m := range manifests {
		res = append(res, m)
	}
	return res, nil
}
