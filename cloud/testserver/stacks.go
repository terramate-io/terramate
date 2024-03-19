// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/strconv"
)

func stateTable() map[drift.Status]map[deployment.Status]stack.Status {
	return map[drift.Status]map[deployment.Status]stack.Status{
		drift.Unknown: {
			deployment.OK:       stack.OK,
			deployment.Failed:   stack.Failed,
			deployment.Canceled: stack.Failed,
		},
		drift.OK: {
			deployment.OK:       stack.OK,
			deployment.Failed:   stack.OK,
			deployment.Canceled: stack.OK,
		},
		drift.Drifted: {
			deployment.OK:       stack.Drifted,
			deployment.Failed:   stack.Failed,
			deployment.Canceled: stack.Failed,
		},
		drift.Failed: {
			deployment.OK:      stack.OK,
			deployment.Pending: stack.OK,
			deployment.Running: stack.OK,
		},
	}
}

// GetStacks is the GET /stacks handler.
func GetStacks(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	orguuid := cloud.UUID(params.ByName("orguuid"))
	filterStatusStr := r.FormValue("status")
	repoStr := r.FormValue("repository")
	metaID := r.FormValue("meta_id")
	perPageStr := r.FormValue("per_page")
	pageStr := r.FormValue("page")
	filterStatus := stack.NoFilter

	org, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeString(w, "organization not found")
		return
	}

	if filterStatusStr != "" {
		filterStatus = stack.NewStatusFilter(filterStatusStr)
		if filterStatus.Is(stack.Unrecognized) {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, errors.E("invalid status: %s", filterStatusStr))
			return
		}
	}

	var andFilters []func(st cloudstore.Stack) bool

	if filterStatus != stack.NoFilter {
		andFilters = append(andFilters,
			func(st cloudstore.Stack) bool {
				return stack.FilterStatus(st.State.Status)&filterStatus != 0
			},
		)
	}

	if repoStr != "" {
		andFilters = append(andFilters,
			func(st cloudstore.Stack) bool {
				return st.Stack.Repository == repoStr
			},
		)
	}

	if metaID != "" {
		andFilters = append(andFilters,
			func(st cloudstore.Stack) bool {
				return st.Stack.MetaID == metaID
			},
		)
	}

	filter := func(st cloudstore.Stack) bool {
		for _, f := range andFilters {
			if !f(st) {
				return false
			}
		}
		return true
	}

	var err error
	var page, perPage int64
	if perPageStr == "" {
		perPage = 10
	} else {
		perPage, err = strconv.Atoi64(perPageStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, errors.E(err, "invalid per_page parameter"))
			return
		}
	}

	if pageStr == "" {
		page = 1
	} else {
		page, err = strconv.Atoi64(pageStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, errors.E(err, "invalid page parameter"))
			return
		}
	}

	start := (page - 1) * perPage

	stacks := org.Stacks
	if start >= int64(len(stacks)) {
		w.Header().Add("Content-Type", "application/json")
		marshalWrite(w, cloud.StacksResponse{
			Pagination: cloud.PaginatedResult{
				Total:   int64(len(stacks)),
				Page:    page,
				PerPage: 0,
			},
		})
		return
	}

	end := start + perPage
	if end > int64(len(stacks)) {
		end = int64(len(stacks))
	}

	var resp cloud.StacksResponse
	for id, st := range stacks[start:end] {
		if !validateStackStatus(st) {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, invalidStackStateError(st))
			return
		}

		if filter(st) {
			resp.Stacks = append(resp.Stacks, cloud.StackObject{
				ID:               int64(id),
				Stack:            st.Stack,
				Status:           st.State.Status,
				DeploymentStatus: st.State.DeploymentStatus,
				DriftStatus:      st.State.DriftStatus,
				CreatedAt:        st.State.CreatedAt,
				UpdatedAt:        st.State.UpdatedAt,
				SeenAt:           st.State.SeenAt,
			})
		}
	}
	sort.Slice(resp.Stacks, func(i, j int) bool {
		return resp.Stacks[i].ID < resp.Stacks[j].ID
	})
	resp.Pagination.Page = page
	resp.Pagination.PerPage = int64(len(resp.Stacks))
	resp.Pagination.Total = int64(len(stacks))
	w.Header().Add("Content-Type", "application/json")
	marshalWrite(w, resp)
}

// PutStack is the PUT /stacks handler.
func PutStack(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	bodyData, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	justClose(r.Body)

	var st cloud.StackObject
	err = json.Unmarshal(bodyData, &st)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	orguuid := cloud.UUID(p.ByName("orguuid"))
	_, err = store.UpsertStack(orguuid, cloudstore.Stack{
		Stack: st.Stack,
		State: cloudstore.StackState{
			Status:    st.Status,
			CreatedAt: st.CreatedAt,
			UpdatedAt: st.UpdatedAt,
			SeenAt:    st.SeenAt,
		},
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetDeploymentLogs is the GET /deployments/.../logs handler.
func GetDeploymentLogs(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, p httprouter.Params) {
	stackIDStr := p.ByName("stackid")
	if stackIDStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	stackid, err := strconv.Atoi64(stackIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
	}
	orguuid := cloud.UUID(p.ByName("orguuid"))
	org, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeString(w, "organization not found")
		return
	}
	stacks := org.Stacks
	if stackid < 0 || stackid >= int64(len(stacks)) {
		w.WriteHeader(http.StatusNotFound)
		writeErr(w, errors.E("stack not found"))
		return
	}
	stack := stacks[stackid]
	deploymentUUID := cloud.UUID(p.ByName("deployment_uuid"))

	logs, err := store.GetDeploymentLogs(orguuid, stack.Stack.MetaID, deploymentUUID, 0)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	data, err := json.MarshalIndent(logs, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	write(w, data)
}

// GetDeploymentLogsEvents is the SSE GET /deployments/.../logs handler.
func GetDeploymentLogsEvents(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, p httprouter.Params) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", "text/event-stream")

	orguuid := cloud.UUID(p.ByName("orguuid"))
	org, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeString(w, "organization not found")
		return
	}

	stackIDStr := p.ByName("stackid")
	if stackIDStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	stackid, err := strconv.Atoi64(stackIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
	}
	stacks := org.Stacks
	if stackid < 0 || stackid >= int64(len(stacks)) {
		w.WriteHeader(http.StatusNotFound)
		writeErr(w, errors.E("stack not found"))
		return
	}
	stack := stacks[stackid]
	deploymentUUID := cloud.UUID(p.ByName("deployment_uuid"))

	line := 0

	// send a ping every 1s
	for {
		logs, err := store.GetDeploymentLogs(orguuid, stack.Stack.MetaID, deploymentUUID, line)
		if err != nil {
			writeErr(w, err)
			return
		}

		for _, l := range logs {
			fmt.Fprintf(w, "%d [%s] %s %s\n", l.Line, l.Channel, l.Timestamp, l.Message)
			w.(http.Flusher).Flush()
			line++
		}
		if len(logs) == 0 {
			fmt.Fprintf(w, ".\n")
			w.(http.Flusher).Flush()
		}
		time.Sleep(1 * time.Second)
	}
}

// PostDeploymentLogs is the POST /deployments/.../logs handler.
func PostDeploymentLogs(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	stackIDStr := p.ByName("stackid")
	if stackIDStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	stackid, err := strconv.Atoi64(stackIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}
	orguuid := cloud.UUID(p.ByName("orguuid"))
	org, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeString(w, "organization not found")
		return
	}

	stacks := org.Stacks
	if stackid < 0 || stackid >= int64(len(stacks)) {
		w.WriteHeader(http.StatusNotFound)
		writeErr(w, errors.E("stack not found"))
		return
	}
	stack := stacks[stackid]
	deploymentUUID := cloud.UUID(p.ByName("deployment_uuid"))

	bodyData, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	justClose(r.Body)

	var logs cloud.CommandLogs
	err = json.Unmarshal(bodyData, &logs)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = store.InsertDeploymentLogs(orguuid, stack.Stack.MetaID, deploymentUUID, logs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetStackDrifts implements the /v1/stacks/:orguuid/:stackid/drifts endpoint.
func GetStackDrifts(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	orguuid := cloud.UUID(params.ByName("orguuid"))
	perPageStr := r.FormValue("per_page")
	pageStr := r.FormValue("page")
	stackid, err := strconv.Atoi64(params.ByName("stackid"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, errors.E(err, "invalid stackid"))
		return
	}

	drifts, err := store.GetStackDrifts(orguuid, stackid)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	var page, perPage int64
	if perPageStr == "" {
		perPage = 10
	} else {
		perPage, err = strconv.Atoi64(perPageStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, errors.E(err, "invalid per_page parameter"))
			return
		}
	}

	if pageStr == "" {
		page = 1
	} else {
		page, err = strconv.Atoi64(pageStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, errors.E(err, "invalid page parameter"))
			return
		}
	}

	start := (page - 1) * perPage

	if start >= int64(len(drifts)) {
		w.Header().Add("Content-Type", "application/json")
		marshalWrite(w, cloud.DriftsStackPayloadResponse{
			Pagination: cloud.PaginatedResult{
				Total:   int64(len(drifts)),
				Page:    page,
				PerPage: 0,
			},
		})
		return
	}

	end := start + perPage
	if end > int64(len(drifts)) {
		end = int64(len(drifts))
	}

	var res cloud.DriftsStackPayloadResponse
	for _, drift := range drifts[start:end] {
		res.Drifts = append(res.Drifts, cloud.Drift{
			ID:       drift.ID,
			Status:   drift.Status,
			Details:  drift.Details,
			Metadata: drift.Metadata,
		})
	}
	res.Pagination = cloud.PaginatedResult{
		Total:   int64(len(drifts)),
		Page:    page,
		PerPage: int64(len(res.Drifts)),
	}
	// return most recent drifts first.
	sort.Slice(res.Drifts, func(i, j int) bool {
		return res.Drifts[i].ID > res.Drifts[j].ID
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	marshalWrite(w, res)
}

func validateStackStatus(s cloudstore.Stack) bool {
	_, ok := stateTable()[s.State.DriftStatus][s.State.DeploymentStatus]
	return ok
}

func invalidStackStateError(st cloudstore.Stack) error {
	return errors.E(
		"stack has invalid state: (drift:%s, deployment:%s, status:%s)",
		st.State.DriftStatus,
		st.State.DeploymentStatus,
		st.State.Status,
	)
}
