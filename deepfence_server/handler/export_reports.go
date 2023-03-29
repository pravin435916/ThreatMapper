package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/deepfence/ThreatMapper/deepfence_server/ingesters"
	"github.com/deepfence/ThreatMapper/deepfence_server/model"
	"github.com/deepfence/golang_deepfence_sdk/utils/directory"
	"github.com/deepfence/golang_deepfence_sdk/utils/log"
	"github.com/deepfence/golang_deepfence_sdk/utils/utils"
	"github.com/go-chi/chi/v5"
	httpext "github.com/go-playground/pkg/v5/net/http"
	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j/dbtype"
)

func (h *Handler) DeleteReport(w http.ResponseWriter, r *http.Request) {

	var req model.ReportReq
	req.ReportID = chi.URLParam(r, "report_id")
	if err := h.Validator.Struct(req); err != nil {
		respondError(&ValidatorError{err}, w)
		return
	}
	if err := h.Validator.Struct(req); err != nil {
		respondError(&ValidatorError{err}, w)
		return
	}

	driver, err := directory.Neo4jClient(r.Context())
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(directory.ErrNamespaceNotFound, w)
	}

	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
	}
	defer session.Close()

	tx, err := session.BeginTransaction()
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
	}
	defer tx.Close()

	query := `MATCH (n:Report{report_id:$uid}) DELETE n`
	vars := map[string]interface{}{"uid": req.ReportID}
	_, err = tx.Run(query, vars)
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}
	httpext.JSON(w, http.StatusOK, nil)
}

func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {

	var req model.ReportReq
	req.ReportID = chi.URLParam(r, "report_id")
	if err := h.Validator.Struct(req); err != nil {
		respondError(&ValidatorError{err}, w)
		return
	}
	driver, err := directory.Neo4jClient(r.Context())
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(directory.ErrNamespaceNotFound, w)
	}

	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
	}
	defer session.Close()

	tx, err := session.BeginTransaction()
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
	}
	defer tx.Close()

	query := `MATCH (n:Report{report_id:$uid}) RETURN n`
	vars := map[string]interface{}{"uid": req.ReportID}
	result, err := tx.Run(query, vars)
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}

	records, err := result.Single()
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}

	i, ok := records.Get("n")
	if !ok {
		respondError(&ingesters.NodeNotFoundError{NodeId: req.ReportID}, w)
		return
	}
	da, ok := i.(dbtype.Node)
	if !ok {
		respondError(&ingesters.NodeNotFoundError{NodeId: req.ReportID}, w)
		return
	}

	var report model.ExportReport
	utils.FromMap(da.Props, &report)

	httpext.JSON(w, http.StatusOK, report)
}

func (h *Handler) ListReports(w http.ResponseWriter, r *http.Request) {

	driver, err := directory.Neo4jClient(r.Context())
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(directory.ErrNamespaceNotFound, w)
	}

	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
	}
	defer session.Close()

	tx, err := session.BeginTransaction()
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
	}
	defer tx.Close()

	query := `MATCH (n:Report) RETURN n`
	result, err := tx.Run(query, map[string]interface{}{})
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}

	records, err := result.Collect()
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}

	reports := []model.ExportReport{}
	for _, rec := range records {
		i, ok := rec.Get("n")
		if !ok {
			log.Warn().Msgf("Missing neo4j entry")
			continue
		}
		da, ok := i.(dbtype.Node)
		if !ok {
			log.Warn().Msgf("Missing neo4j entry")
			continue
		}
		var report model.ExportReport
		utils.FromMap(da.Props, &report)
		reports = append(reports, report)
	}

	httpext.JSON(w, http.StatusOK, reports)
}

func (h *Handler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req model.GenerateReportReq
	err := httpext.DecodeJSON(r, httpext.NoQueryParams, MaxPostRequestSize, &req)
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(&BadDecoding{err}, w)
		return
	}

	// report task params
	report_id := uuid.New().String()
	params := utils.ReportParams{
		ReportID:   report_id,
		ReportType: req.ReportType,
		Duration:   req.Duration,
		Filters:    req.Filters,
	}

	namespace, err := directory.ExtractNamespace(r.Context())
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}

	driver, err := directory.Neo4jClient(r.Context())
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(directory.ErrNamespaceNotFound, w)
	}

	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
	}
	defer session.Close()

	tx, err := session.BeginTransaction()
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
	}
	defer tx.Close()

	query := `
	CREATE (n:Report{created_at:TIMESTAMP(), type:$type, report_id:$uid, status:$status, filters:$filters, duration:$duration})
	RETURN n`
	vars := map[string]interface{}{
		"type":     req.ReportType,
		"uid":      report_id,
		"status":   utils.SCAN_STATUS_STARTING,
		"filters":  req.Filters.String(),
		"duration": req.Duration,
	}

	_, err = tx.Run(query, vars)
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}

	payload, err := json.Marshal(params)
	if err != nil {
		log.Error().Msg(err.Error())
		respondError(err, w)
		return
	}

	// create a task message
	msg := message.NewMessage(watermill.NewUUID(), payload)
	msg.Metadata = map[string]string{
		directory.NamespaceKey: string(namespace),
		"report_type":          req.ReportType,
	}
	msg.SetContext(directory.NewContextWithNameSpace(namespace))
	middleware.SetCorrelationID(watermill.NewShortUUID(), msg)

	err = h.TasksPublisher.Publish(utils.ReportGeneratorTask, msg)
	if err != nil {
		log.Error().Msgf("failed to publish task: %+v", err)
		respondError(err, w)
		return
	}

	httpext.JSON(w, http.StatusOK, model.GenerateReportResp{ReportID: report_id})
}
