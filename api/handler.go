package api

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/dstotijn/ct-diag-server/diag"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

const maxBatchSize = 60

type handler struct {
	diagSvc diag.Service
}

// NewHandler returns a new Handler.
func NewHandler(repo diag.Repository) http.Handler {
	h := handler{diagSvc: diag.NewService(repo)}

	router := httprouter.New()
	router.GET("/diagnosis-keys", h.listDiagnosisKeys)
	router.POST("/diagnosis-keys", h.postDiagnosisKeys)
	router.GET("/health", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Write([]byte("OK"))
	})

	return router
}

// listDiagnosisKeys writes all diagnosis keys as binary data in the HTTP response.
func (h *handler) listDiagnosisKeys(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	diagKeys, err := h.diagSvc.FindAllDiagnosisKeys(r.Context())
	if err != nil {
		log.Printf("api: error finding all diagnosis keys: %v", err)
		writeInternalErrorResp(w, err)
		return
	}

	if len(diagKeys) == 0 {
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// 18 bytes per diagnosis key: 16 bytes (UUID) + 2 bytes (uint16).
	w.Header().Set("Content-Length", strconv.Itoa(len(diagKeys)*18))

	// Write binary data for the diagnosis keys. Per diagnosis key, 16 bytes are
	// written with the diagnosis key itself (UUID), and 2 bytes for its day
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
func (h *handler) postDiagnosisKeys(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	diagKeys := make([]diag.DiagnosisKey, 0, maxBatchSize)

	for {
		keyBuf := make([]byte, 16)
		_, err := r.Body.Read(keyBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		if len(diagKeys) == maxBatchSize {
			code := http.StatusRequestEntityTooLarge
			http.Error(w, http.StatusText(code), code)
			return
		}

		key, err := uuid.FromBytes(keyBuf)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid UUID: %v", err), http.StatusBadRequest)
			return
		}

		var dayNumber uint16
		err = binary.Read(r.Body, binary.BigEndian, &dayNumber)
		if err == io.EOF {
			http.Error(w, "Missing day number", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "Invalid day number", http.StatusBadRequest)
			return
		}

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
}

func writeInternalErrorResp(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	http.Error(w, http.StatusText(code), code)
}
