package httpapi

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type internalContingencyTickRequest struct {
	AdAccountIDs  []string `json:"ad_account_ids"`
	DryRun        bool     `json:"dry_run"`
	RefreshStatus *bool    `json:"refresh_status"`
	MaxCandidates int      `json:"max_candidates"`
	MaxAttempts   int      `json:"max_attempts"`
	TriggerType   string   `json:"trigger_type"`
	DispatchTasks *bool    `json:"dispatch_tasks"`
}

type internalContingencyTickAccountResult struct {
	AdAccountID        string                   `json:"ad_account_id"`
	CandidatesScanned  int                      `json:"candidates_scanned"`
	CandidatesDeduped  int                      `json:"candidates_deduped"`
	IncidentsCreated   int                      `json:"incidents_created"`
	IncidentsExisting  int                      `json:"incidents_existing"`
	IncidentsProcessed int                      `json:"incidents_processed"`
	IncidentsEnqueued  int                      `json:"incidents_enqueued"`
	Items              []monitorContingencyItem `json:"items"`
	EnqueuedTasks      []string                 `json:"enqueued_tasks,omitempty"`
	Failed             []string                 `json:"failed,omitempty"`
	EnqueueFailed      []string                 `json:"enqueue_failed,omitempty"`
	Error              string                   `json:"error,omitempty"`
}

type memoryResponseWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newMemoryResponseWriter() *memoryResponseWriter {
	return &memoryResponseWriter{
		header: make(http.Header),
		status: http.StatusOK,
	}
}

func (m *memoryResponseWriter) Header() http.Header {
	return m.header
}

func (m *memoryResponseWriter) Write(b []byte) (int, error) {
	return m.body.Write(b)
}

func (m *memoryResponseWriter) WriteHeader(statusCode int) {
	m.status = statusCode
}

func normalizeAdAccountIDs(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (h *Handler) contingencyDefaultMaxCandidates() int {
	return parseContingencyLimit(h.ContingencyDefaultMaxCandidates)
}

func (h *Handler) contingencyDefaultMaxAttempts() int {
	return parseContingencyMaxAttempts(h.ContingencyDefaultMaxAttempts)
}

func (h *Handler) authorizeInternalContingencyRequest(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(h.ContingencyInternalToken)
	if expected == "" {
		writeErr(w, http.StatusServiceUnavailable, "contingency_internal_not_configured")
		return false
	}

	provided := strings.TrimSpace(r.Header.Get("X-Contingency-Token"))
	if provided == "" {
		writeErr(w, http.StatusUnauthorized, "invalid_internal_contingency_token")
		return false
	}

	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeErr(w, http.StatusUnauthorized, "invalid_internal_contingency_token")
		return false
	}

	return true
}

func (h *Handler) InternalContingencyTick(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeInternalContingencyRequest(w, r) {
		return
	}

	var req internalContingencyTickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}

	triggerType := normalizeContingencyTriggerType(req.TriggerType)
	if !contingencyTriggerTypeAllowed(triggerType) {
		writeErr(w, http.StatusBadRequest, "invalid_contingency_trigger_type")
		return
	}

	maxCandidates := h.contingencyDefaultMaxCandidates()
	if req.MaxCandidates > 0 {
		maxCandidates = parseContingencyLimit(req.MaxCandidates)
	}

	maxAttempts := h.contingencyDefaultMaxAttempts()
	if req.MaxAttempts > 0 {
		maxAttempts = parseContingencyMaxAttempts(req.MaxAttempts)
	}

	refreshStatus := h.ContingencyDefaultRefreshStatus
	if req.RefreshStatus != nil {
		refreshStatus = *req.RefreshStatus
	}

	dispatchTasks := h.ContingencyDispatchViaTasks
	if req.DispatchTasks != nil {
		dispatchTasks = *req.DispatchTasks
	}
	if dispatchTasks && h.ContingencyTasks == nil {
		writeErr(w, http.StatusServiceUnavailable, "contingency_task_queue_not_configured")
		return
	}

	adAccountIDs := normalizeAdAccountIDs(req.AdAccountIDs)
	if len(adAccountIDs) == 0 {
		adAccountIDs = normalizeAdAccountIDs(h.ContingencyMonitorAdAccounts)
	}
	if len(adAccountIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "missing_contingency_ad_accounts")
		return
	}

	results := make([]internalContingencyTickAccountResult, 0, len(adAccountIDs))
	totalEnqueued := 0
	totalCreated := 0
	totalExisting := 0
	globalFailed := make([]string, 0)

	for _, adAccountID := range adAccountIDs {
		monitorReq := monitorContingencyRequest{
			AdAccountID:   adAccountID,
			DryRun:        req.DryRun,
			RefreshStatus: &refreshStatus,
			MaxCandidates: maxCandidates,
			TriggerType:   triggerType,
		}

		runResult, err := h.runMonitorContingency(r.Context(), monitorReq)
		if err != nil {
			results = append(results, internalContingencyTickAccountResult{
				AdAccountID: adAccountID,
				Error:       err.Error(),
			})
			globalFailed = append(globalFailed, adAccountID+": "+err.Error())
			continue
		}

		entry := internalContingencyTickAccountResult{
			AdAccountID:        adAccountID,
			CandidatesScanned:  runResult.CandidatesScanned,
			CandidatesDeduped:  runResult.CandidatesDeduped,
			IncidentsCreated:   runResult.IncidentsCreated,
			IncidentsExisting:  runResult.IncidentsExisting,
			IncidentsProcessed: runResult.IncidentsTotal,
			Items:              runResult.Items,
			Failed:             runResult.Failed,
		}

		totalCreated += runResult.IncidentsCreated
		totalExisting += runResult.IncidentsExisting

		if dispatchTasks && !req.DryRun {
			for _, item := range runResult.Items {
				if strings.TrimSpace(item.IncidentUUID) == "" {
					continue
				}
				if strings.ToLower(strings.TrimSpace(item.IncidentState)) != "detected" {
					continue
				}
				taskName, enqueueErr := h.ContingencyTasks.EnqueueContingencyExecution(
					r.Context(),
					item.IncidentUUID,
					maxAttempts,
				)
				if enqueueErr != nil {
					entry.EnqueueFailed = append(entry.EnqueueFailed, item.IncidentUUID+": "+enqueueErr.Error())
					globalFailed = append(globalFailed, item.IncidentUUID+": "+enqueueErr.Error())
					continue
				}
				entry.IncidentsEnqueued++
				totalEnqueued++
				if taskName != "" {
					entry.EnqueuedTasks = append(entry.EnqueuedTasks, taskName)
				}
			}
		}

		results = append(results, entry)
	}

	statusCode := http.StatusOK
	if len(globalFailed) > 0 {
		statusCode = http.StatusMultiStatus
	}

	resp := map[string]any{
		"ad_account_ids":     adAccountIDs,
		"accounts_processed": len(results),
		"dry_run":            req.DryRun,
		"dispatch_tasks":     dispatchTasks,
		"refresh_status":     refreshStatus,
		"trigger_type":       triggerType,
		"max_candidates":     maxCandidates,
		"max_attempts":       maxAttempts,
		"incidents_created":  totalCreated,
		"incidents_existing": totalExisting,
		"incidents_enqueued": totalEnqueued,
		"results":            results,
	}
	if len(globalFailed) > 0 {
		resp["failed"] = globalFailed
	}

	writeJSON(w, statusCode, resp)
}

func (h *Handler) InternalContingencyExecute(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeInternalContingencyRequest(w, r) {
		return
	}

	var req executeContingencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}

	req.IncidentUUID = strings.TrimSpace(req.IncidentUUID)
	if req.IncidentUUID == "" {
		writeErr(w, http.StatusBadRequest, "missing_contingency_incident_uuid")
		return
	}

	incident, err := h.Store.GetContingencyIncidentByUUID(r.Context(), req.IncidentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusOK, map[string]any{
				"internal_processed": true,
				"original_status":    http.StatusNotFound,
				"result": map[string]any{
					"error_code": "contingency_incident_not_found",
					"message":    "incident not found; task ignored",
				},
			})
			return
		}
		writeErrCause(w, http.StatusInternalServerError, "failed_to_get_contingency_incident", err)
		return
	}

	recorder := newMemoryResponseWriter()
	h.executeContingencyIncident(recorder, r, incident, parseContingencyMaxAttempts(req.MaxAttempts))

	// Em Cloud Tasks, códigos 4xx causam retry desnecessário para erros de negócio.
	// Apenas falhas 5xx retornam erro HTTP para reprocessamento automático.
	if recorder.status >= http.StatusInternalServerError {
		for key, values := range recorder.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(recorder.status)
		_, _ = w.Write(recorder.body.Bytes())
		return
	}

	var payload any
	if err := json.Unmarshal(recorder.body.Bytes(), &payload); err != nil {
		payload = map[string]any{
			"raw_body": recorder.body.String(),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"internal_processed": true,
		"original_status":    recorder.status,
		"result":             payload,
	})
}
