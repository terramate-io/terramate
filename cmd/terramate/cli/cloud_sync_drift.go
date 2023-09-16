// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
)

func (c *cli) cloudSyncDriftStatus(st *config.Stack, exitCode int, err error) {
	logger := log.With().
		Str("action", "cloudSyncDriftStatus").
		Stringer("stack", st.Dir).
		Int("exit_code", exitCode).
		Err(err).
		Logger()

	var status stack.Status
	switch {
	case exitCode == 0:
		status = stack.OK
	case exitCode == 2:
		status = stack.Drifted
	case exitCode == 1 || exitCode > 2 || errors.IsAnyKind(err, ErrRunCommandNotFound, ErrRunFailed):
		status = stack.Failed
	default:
		// ignore exit codes < 0
		logger.Debug().Msg("skipping drift sync")
		return
	}

	logger = logger.With().
		Stringer("drift_status", status).
		Logger()

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()

	_, err = c.cloud.client.CreateStackDrift(ctx, c.cloud.run.orgUUID, cloud.DriftStackPayloadRequest{
		Stack: cloud.Stack{
			Repository:      c.prj.prettyRepo(),
			Path:            st.Dir.String(),
			MetaID:          st.ID,
			MetaName:        st.Name,
			MetaDescription: st.Description,
			MetaTags:        st.Tags,
		},
		Status: status,
	})

	if err != nil {
		logger.Error().Err(err).Msg("failed to sync the drift status")
	} else {
		logger.Debug().Msg("synced drift_status successfully")
	}
}
