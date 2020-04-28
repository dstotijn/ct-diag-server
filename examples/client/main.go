package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dstotijn/ct-diag-server/diag"
)

const (
	actionList = "list"
	actionPost = "post"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

func main() {
	var (
		action    string
		baseURL   string
		batchSize int
	)

	flag.StringVar(&baseURL, "baseURL", "http://localhost", "Base URL of cg-diag-server")
	flag.StringVar(&action, "action", actionList, "Action (default: `list`, allowed values: `list`, `post`)")
	flag.IntVar(&batchSize, "batchSize", 14, "Diagnosis Key batch size, used when posting keys")
	flag.Parse()

	switch action {
	case actionList:
		listDiagnosisKeys(baseURL)
	case actionPost:
		postDiagnosisKeys(baseURL, batchSize)
	default:
		log.Fatalf("Unsupported action (%v)", action)
	}

}

func listDiagnosisKeys(baseURL string) {
	req, err := http.NewRequest("GET", baseURL+"/diagnosis-keys", nil)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	contentLength, _ := strconv.Atoi(resp.Header.Get("Content-Length"))

	log.Printf("Received HTTP response with %v key(s): %v %v: [% #x]",
		contentLength/diag.DiagnosisKeySize,
		resp.StatusCode,
		http.StatusText(resp.StatusCode),
		body,
	)
}

func postDiagnosisKeys(baseURL string, batchSize int) {
	diagKeys := diagnosisKeys(batchSize)

	buf := &bytes.Buffer{}
	for _, diagKey := range diagKeys {
		_, err := buf.Write(diagKey.TemporaryExposureKey[:])
		if err != nil {
			log.Fatal(err)
		}
		err = binary.Write(buf, binary.BigEndian, diagKey.ENIntervalNumber)
		if err != nil {
			log.Fatal(err)
		}
	}

	req, err := http.NewRequest("POST", baseURL+"/diagnosis-keys", buf)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Received HTTP response: %v %v: `%s`",
		resp.StatusCode,
		http.StatusText(resp.StatusCode),
		strings.TrimSpace(string(body)),
	)

}

func diagnosisKeys(n int) (keys []diag.DiagnosisKey) {
	for i := 0; i < n; i++ {
		// startRollingNumber is the ENIntervalNumber that denotes the start
		// validity time of a TemporaryExposureKey.
		startRollingNumber := time.Now().AddDate(0, 0, -(i+1)).Unix() / (60 * 10) / 144 * 144
		buf := make([]byte, 16)
		_, err := rand.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		var key [16]byte
		copy(key[:], buf)
		keys = append(keys, diag.DiagnosisKey{
			TemporaryExposureKey: key,
			ENIntervalNumber:     uint32(startRollingNumber),
		})
	}
	return
}
