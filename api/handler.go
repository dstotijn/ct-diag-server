package api

import (
	"encoding/binary"
	"fmt"
	"io"
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

	// 18 bytes per diagnosis key: 16 bytes (key) + 2 bytes (day number).
	w.Header().Set("Content-Length", strconv.Itoa(len(diagKeys)*18))

	// Write binary data for the diagnosis keys. Per diagnosis key, 16 bytes are
	// written with the diagnosis key itself, and 2 bytes for its day
	// number (uint16, big endian). Because both parts have a fixed length,
	// there is no delimiter.
	for i := range diagKeys {
		_, err := w.Write(diagKeys[i].Key[:])
		if err != nil {
			return
		}
		dayNumber := make([]byte, 2)
		binary.BigEndian.PutUint16(dayNumber, diagKeys[i].DayNumber)
		_, err = w.Write(dayNumber)
		if err != nil {
			return
		}
	}
}

// postDiagnosisKeys reads POST data from an HTTP request and stores it.
func (h *handler) postDiagnosisKeys(w http.ResponseWriter, r *http.Request) {
	diagKeys := make([]diag.DiagnosisKey, 0, diag.MaxUploadBatchSize)

	for {
		// 18 bytes for the key (16 bytes) and the day number (2 bytes).
		var diagKey [18]byte
		_, err := io.ReadFull(r.Body, diagKey[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid diagnosis key: %v", err), http.StatusBadRequest)
			return
		}

		if len(diagKeys) == diag.MaxUploadBatchSize {
			code := http.StatusRequestEntityTooLarge
			http.Error(w, http.StatusText(code), code)
			return
		}

		var key [16]byte
		copy(key[:], diagKey[:16])
		dayNumber := binary.BigEndian.Uint16(diagKey[16:])

		diagKeys = append(diagKeys, diag.DiagnosisKey{Key: key, DayNumber: dayNumber})
	}

	if len(diagKeys) == 0 {
		http.Error(w, "Request body is missing", http.StatusBadRequest)
		return
	}

	err := h.diagSvc.StoreDiagnosisKeys(r.Context(), diagKeys)
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
