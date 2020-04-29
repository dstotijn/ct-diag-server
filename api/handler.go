package api

import (
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/dstotijn/ct-diag-server/diag"
)

type handler struct {
	diagSvc diag.Service
}

// NewHandler returns a new Handler.
func NewHandler(repo diag.Repository) http.Handler {
	h := handler{diagSvc: diag.NewService(repo)}

	mux := http.NewServeMux()

	mux.HandleFunc("/diagnosis-keys", h.diagnosisKeys)
	mux.HandleFunc("/health", h.health)

	return mux
}

// diagnosisKeys handles both GET and POST requests.
func (h *handler) diagnosisKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
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
	diagKeys, err := h.diagSvc.FindAllDiagnosisKeys(r.Context())
	if err != nil {
		log.Printf("api: error finding all diagnosis keys: %v", err)
		writeInternalErrorResp(w, err)
		return
	}

	if len(diagKeys) == 0 {
		w.Header().Set("Content-Length", "0")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	w.Header().Set("Content-Length", strconv.Itoa(len(diagKeys)*diag.DiagnosisKeySize))

	// Write binary data for the diagnosis keys. Per diagnosis key, 16 bytes are
	// written with the diagnosis key itself, and 4 bytes for `ENIntervalNumber`
	// (uint32, big endian). Because both parts have a fixed length, there is no
	// delimiter.
	for i := range diagKeys {
		_, err := w.Write(diagKeys[i].TemporaryExposureKey[:])
		if err != nil {
			return
		}
		enin := make([]byte, 4)
		binary.BigEndian.PutUint32(enin, diagKeys[i].ENIntervalNumber)
		_, err = w.Write(enin)
		if err != nil {
			return
		}
	}
}

// postDiagnosisKeys reads POST data from an HTTP request and stores it.
func (h *handler) postDiagnosisKeys(w http.ResponseWriter, r *http.Request) {
	maxBytesReader := http.MaxBytesReader(w, r.Body, diag.UploadLimit)
	diagKeys, err := h.diagSvc.ParseDiagnosisKeys(maxBytesReader)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid body: %v", err), http.StatusBadRequest)
		return
	}

	err = h.diagSvc.StoreDiagnosisKeys(r.Context(), diagKeys)
	if err != nil {
		log.Printf("api: error storing diagnosis keys: %v", err)
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
