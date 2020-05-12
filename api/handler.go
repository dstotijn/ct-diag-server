// Package api provides an http.Handler with handlers for uploading and retrieving
// Diagnosis Keys.
package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/dstotijn/ct-diag-server/diag"

	"go.uber.org/zap"
)

type handler struct {
	diagSvc diag.Service
	logger  *zap.Logger
}

// NewHandler returns a new Handler.
func NewHandler(ctx context.Context, cfg diag.Config, logger *zap.Logger) (http.Handler, error) {
	diagSvc, err := diag.NewService(ctx, cfg)
	if err != nil {
		return nil, err
	}

	h := handler{
		diagSvc: diagSvc,
		logger:  logger,
	}

	expConfigHandler, err := exposureConfig(cfg.ExposureConfig)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/diagnosis-keys", h.diagnosisKeys)
	mux.HandleFunc("/exposure-config", expConfigHandler)
	mux.HandleFunc("/health", h.health)

	return mux, nil
}

// diagnosisKeys handles both GET and POST requests.
func (h *handler) diagnosisKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		fallthrough
	case http.MethodGet:
		h.listDiagnosisKeys(w, r)
	case http.MethodPost:
		h.postDiagnosisKeys(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// listDiagnosisKeys writes all diagnosis keys as binary data in the HTTP response.
func (h *handler) listDiagnosisKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=600")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	var after [16]byte
	afterParam := r.URL.Query().Get("after")
	if afterParam != "" {
		buf, err := hex.DecodeString(afterParam)
		if err != nil || len(buf) != 16 {
			msg := fmt.Sprintf("Invalid `after` query parameter, must be the hexadecimal encoding of a 16 byte key.")
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		copy(after[:], buf)
	}

	diagKeys, err := h.diagSvc.ListDiagnosisKeys(after)
	if err != nil {
		h.logger.Error("Could not fetch diagnosis keys", zap.Error(err))
		writeInternalErrorResp(w, err)
		return
	}

	if len(diagKeys) == 0 {
		w.Header().Set("Content-Length", "0")
		return
	}

	buf := &bytes.Buffer{}
	n, err := diag.WriteDiagnosisKeyProtobuf(buf, diagKeys...)
	if err != nil {
		h.logger.Error("Could not write diagnosis keys to protobuf", zap.Error(err))
		writeInternalErrorResp(w, err)
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(n))
	io.Copy(w, buf)
}

// postDiagnosisKeys reads POST data from an HTTP request and stores it.
func (h *handler) postDiagnosisKeys(w http.ResponseWriter, r *http.Request) {
	maxBytesReader := http.MaxBytesReader(w, r.Body, diag.MaxUploadSize)
	diagKeys, err := diag.ParseDiagnosisKeyFile(maxBytesReader)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid body: %v", err), http.StatusBadRequest)
		return
	}
	maxBytesReader.Close()

	if len(diagKeys) == 0 {
		http.Error(w, "Missing request body", http.StatusBadRequest)
		return
	}

	err = h.diagSvc.StoreDiagnosisKeys(r.Context(), diagKeys)
	if err != nil {
		h.logger.Error("Could not store diagnosis keys", zap.Error(err))
		writeInternalErrorResp(w, err)
		return
	}

	fmt.Fprint(w, "OK")
}

// health writes OK in the HTTP response.
func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "OK")
}

func writeInternalErrorResp(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	http.Error(w, http.StatusText(code), code)
}

// exposureConfig returns the exposure configuration in JSON.
func exposureConfig(expCfg diag.ExposureConfig) (http.HandlerFunc, error) {
	buf, err := json.Marshal(expCfg)
	if err != nil {
		return nil, err
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buf)
	}, nil
}
