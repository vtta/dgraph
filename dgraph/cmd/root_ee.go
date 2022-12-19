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

package cmd

import (
	acl "github.com/vtta/dgraph/ee/acl"
	"github.com/vtta/dgraph/ee/audit"
	"github.com/vtta/dgraph/ee/backup"
)

func init() {
	// subcommands already has the default subcommands, we append to EE ones to that.
	subcommands = append(subcommands,
		&backup.Restore,
		&backup.LsBackup,
		&backup.ExportBackup,
		&acl.CmdAcl,
		&audit.CmdAudit,
	)
}
