// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
)

type stackHandler struct {
	stacks   map[string]map[int]cloud.StackResponse
	statuses map[string]map[int]stack.Status
	logs     map[string]map[int]map[string]cloud.DeploymentLogs
	mu       sync.RWMutex
}

func newStackEndpoint() *stackHandler {
	return &stackHandler{
		stacks:   make(map[string]map[int]cloud.StackResponse),
		statuses: make(map[string]map[int]stack.Status),
		logs:     make(map[string]map[int]map[string]cloud.DeploymentLogs),
	}
}

func (handler *stackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	orguuid := params.ByName("orguuid")
	filterStatusStr := r.FormValue("status")
	filterStatus := stack.AllFilter

	if filterStatusStr != "" && filterStatusStr != "unhealthy" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if filterStatusStr == "unhealthy" {
		filterStatus = stack.UnhealthyFilter
	}

	w.Header().Add("Content-Type", "application/json")

	// GET /v1/stacks/{orguuid}
	if r.Method == "GET" && r.URL.Path == "/v1/stacks/"+orguuid {
		var resp cloud.StacksResponse
		var stacks []cloud.StackResponse
		stacksMap, ok := handler.stacks[orguuid]
		if !ok {
			w.WriteHeader(http.StatusOK)
			data, _ := json.Marshal(resp)
			_, _ = w.Write(data)
			return
		}
		for _, st := range stacksMap {
			if stack.FilterStatus(st.Status)&filterStatus != 0 {
				stacks = append(stacks, st)
			}
		}

		sort.Slice(stacks, func(i, j int) bool {
			return stacks[i].ID < stacks[j].ID
		})

		resp.Stacks = stacks
		data, err := json.Marshal(resp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeString(w, "marshaling error")
		}
		write(w, data)
		return
	}

	// POST /v1/stacks/{org_uuid}/{stack_id}/deployments/{deployment_uuid}/logs
	if r.Method == "POST" {
		// lazy, weak check
		if !strings.HasSuffix(r.URL.Path, "/logs") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		stackIDStr := params.ByName("stackid")
		if stackIDStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		stackid, err := strconv.Atoi(stackIDStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, err)
		}
		deploymentUUID := params.ByName("deployment_uuid")

		bodyData, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		justClose(r.Body)

		var logs cloud.DeploymentLogs
		err = json.Unmarshal(bodyData, &logs)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		handler.mu.Lock()
		defer handler.mu.Unlock()

		if _, ok := handler.logs[orguuid]; !ok {
			handler.logs[orguuid] = make(map[int]map[string]cloud.DeploymentLogs)
		}
		if _, ok := handler.logs[orguuid][stackid]; !ok {
			handler.logs[orguuid][stackid] = make(map[string]cloud.DeploymentLogs)
		}
		oldLogs, ok := handler.logs[orguuid][stackid][deploymentUUID]
		if !ok {
			oldLogs = cloud.DeploymentLogs{}
		}
		oldLogs = append(oldLogs, logs...)
		handler.logs[orguuid][stackid][deploymentUUID] = oldLogs

		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/logs") {
		stackIDStr := params.ByName("stackid")
		if stackIDStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		stackid, err := strconv.Atoi(stackIDStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, err)
		}
		deploymentUUID := params.ByName("deployment_uuid")

		handler.mu.Lock()
		if _, ok := handler.logs[orguuid]; !ok {
			handler.logs[orguuid] = make(map[int]map[string]cloud.DeploymentLogs)
		}
		if _, ok := handler.logs[orguuid][stackid]; !ok {
			handler.logs[orguuid][stackid] = make(map[string]cloud.DeploymentLogs)
		}
		logs, ok := handler.logs[orguuid][stackid][deploymentUUID]
		if !ok {
			logs = cloud.DeploymentLogs{}
		}
		handler.logs[orguuid][stackid][deploymentUUID] = logs
		handler.mu.Unlock()

		data, err := json.MarshalIndent(logs, "", "    ")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		write(w, data)
		return
	}

	if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/logs/events") {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Content-Type", "text/event-stream")

		stackIDStr := params.ByName("stackid")
		if stackIDStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		stackid, err := strconv.Atoi(stackIDStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, err)
		}
		deploymentUUID := params.ByName("deployment_uuid")

		handler.mu.Lock()
		if _, ok := handler.logs[orguuid]; !ok {
			handler.logs[orguuid] = make(map[int]map[string]cloud.DeploymentLogs)
		}
		if _, ok := handler.logs[orguuid][stackid]; !ok {
			handler.logs[orguuid][stackid] = make(map[string]cloud.DeploymentLogs)
		}
		oldLogs, ok := handler.logs[orguuid][stackid][deploymentUUID]
		if !ok {
			oldLogs = cloud.DeploymentLogs{}
		}
		handler.logs[orguuid][stackid][deploymentUUID] = oldLogs
		handler.mu.Unlock()

		line := 0

		// send a ping every 1s
		for {
			handler.mu.RLock()
			logs := handler.logs[orguuid][stackid][deploymentUUID]
			newLogs := logs[line:]
			handler.mu.RUnlock()

			for _, l := range newLogs {
				fmt.Fprintf(w, "%d [%s] %s %s\n", l.Line, l.Channel, l.Timestamp, l.Message)
				w.(http.Flusher).Flush()
				line++
			}
			if len(newLogs) == 0 {
				fmt.Fprintf(w, ".\n")
				w.(http.Flusher).Flush()
			}
			time.Sleep(1 * time.Second)
		}
	}

	// fake endpoint
	// PUT /v1/stacks/{orguuid}/{stackid}
	if r.Method == "PUT" {
		stackIDStr := params.ByName("stackid")
		if stackIDStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		stackid, err := strconv.Atoi(stackIDStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, err)
		}
		bodyData, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		justClose(r.Body)

		var st cloud.StackResponse
		err = json.Unmarshal(bodyData, &st)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, err)
			return
		}

		if stackid != st.ID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		handler.mu.Lock()
		defer handler.mu.Unlock()

		if _, ok := handler.stacks[orguuid]; !ok {
			handler.stacks[orguuid] = make(map[int]cloud.StackResponse)
		}
		if _, ok := handler.statuses[orguuid]; !ok {
			handler.statuses[orguuid] = make(map[int]stack.Status)
		}

		handler.stacks[orguuid][stackid] = st
		handler.statuses[orguuid][stackid] = st.Status
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}
