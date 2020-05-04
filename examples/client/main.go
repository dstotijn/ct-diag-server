package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
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

	flag.StringVar(&baseURL, "baseURL", "http://localhost:8080", "Base URL of cg-diag-server")
	flag.StringVar(&action, "action", actionList, "Action (allowed values: `list`, `post`)")
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

	diagKeys, err := diag.ParseDiagnosisKeys(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Received HTTP response with %v key(s): %v %v: %+v",
		len(diagKeys),
		resp.StatusCode,
		http.StatusText(resp.StatusCode),
		diagKeys,
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
		err = binary.Write(buf, binary.BigEndian, diagKey.RollingStartNumber)
		if err != nil {
			log.Fatal(err)
		}
		_, err = buf.Write([]byte{diagKey.TransmissionRiskLevel})
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
		// rollingStartNumber is the RollingStartNumber that denotes the start
		// validity time of a TemporaryExposureKey.
		rollingStartNumber := time.Now().Add(time.Duration(-i+1)*24*time.Hour).Unix() / (60 * 10) / 144 * 144
		buf := make([]byte, 16)
		_, err := rand.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		var key [16]byte
		copy(key[:], buf)
		keys = append(keys, diag.DiagnosisKey{
			TemporaryExposureKey:  key,
			RollingStartNumber:    uint32(rollingStartNumber),
			TransmissionRiskLevel: 50,
		})
	}
	return
}
