package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestWeatherHandler_InvalidCEP(t *testing.T) {
	req := httptest.NewRequest("GET", "/1234567", nil)
	rr := httptest.NewRecorder()
	handler := weatherHandler(http.DefaultClient, "", "")
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("esperado status 422, obteve %d", rr.Code)
	}
	if rr.Body.String() != "invalid zipcode\n" {
		t.Errorf("esperado mensagem 'invalid zipcode', obteve %s", rr.Body.String())
	}
}

func TestWeatherHandler_NonNumericCEP(t *testing.T) {
	req := httptest.NewRequest("GET", "/abcde123", nil)
	rr := httptest.NewRecorder()
	handler := weatherHandler(http.DefaultClient, "", "")
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("esperado status 422, obteve %d", rr.Code)
	}
	if rr.Body.String() != "invalid zipcode\n" {
		t.Errorf("esperado mensagem 'invalid zipcode', obteve %s", rr.Body.String())
	}
}

func TestWeatherHandler_CEPNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"erro": "true"}`)
	}))
	defer server.Close()

	req := httptest.NewRequest("GET", "/00000000", nil)
	rr := httptest.NewRecorder()
	handler := weatherHandler(server.Client(), server.URL+"/%s", "")
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("esperado status 404, obteve %d", rr.Code)
	}
	if rr.Body.String() != "can not find zipcode\n" {
		t.Errorf("esperado mensagem 'can not find zipcode', obteve %s", rr.Body.String())
	}
}

func TestWeatherHandler_Success(t *testing.T) {
	viaCEPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"localidade": "São Paulo"}`)
	}))
	defer viaCEPServer.Close()

	weatherAPIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"current": {"temp_c": 25.0}}`)
	}))
	defer weatherAPIServer.Close()

	os.Setenv("WEATHER_API_KEY", "fake_key")
	defer os.Unsetenv("WEATHER_API_KEY")

	req := httptest.NewRequest("GET", "/01001000", nil)
	rr := httptest.NewRecorder()

	viaCEPURL := viaCEPServer.URL + "/%s/json/"
	weatherAPIURL := weatherAPIServer.URL + "/v1/current.json?key=%s&q=%s&aqi=no"

	handler := weatherHandler(http.DefaultClient, viaCEPURL, weatherAPIURL)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("esperado status 200, obteve %d", rr.Code)
	}

	var response map[string]float64
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("erro ao decodificar resposta: %v", err)
	}

	if tempC, ok := response["temp_C"]; !ok || tempC != 25.0 {
		t.Errorf("esperado temp_C 25.0, obteve %f", tempC)
	}
}
