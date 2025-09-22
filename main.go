package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type ViaCEPResponse struct {
	Cep        string `json:"cep"`
	Logradouro string `json:"logradouro"`
	Localidade string `json:"localidade"`
	Uf         string `json:"uf"`
	Erro       string `json:"erro,omitempty"`
}

type WeatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

type ErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func isNumeric(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func weatherHandler(client *http.Client, viaCEPURL, weatherAPIURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cep := strings.TrimPrefix(r.URL.Path, "/")
		if len(cep) != 8 || !isNumeric(cep) {
			http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
			return
		}

		// Consulta ViaCEP
		viaURL := fmt.Sprintf(viaCEPURL, cep)
		viaResp, err := client.Get(viaURL)
		if err != nil {
			fmt.Println("Error fetching from ViaCEP:", err)
			http.Error(w, "can not find zipcode", http.StatusNotFound)
			return
		}
		defer viaResp.Body.Close()

		var via ViaCEPResponse
		if err := json.NewDecoder(viaResp.Body).Decode(&via); err != nil {
			fmt.Println("Error decoding ViaCEP JSON:", err)
			http.Error(w, "can not decode viacep response", http.StatusInternalServerError)
			return
		}

		if via.Erro != "" || via.Localidade == "" {
			http.Error(w, "can not find zipcode", http.StatusNotFound)
			return
		}

		// Consulta WeatherAPI
		apiKey := os.Getenv("WEATHER_API_KEY")

		if apiKey == "" {
			http.Error(w, "internal server error: API key missing", http.StatusInternalServerError)
			return
		}

		fullWeatherURL := fmt.Sprintf(weatherAPIURL, apiKey, url.QueryEscape(via.Localidade))
		weatherResp, err := client.Get(fullWeatherURL)
		if err != nil {
			fmt.Println("Error fetching from WeatherAPI:", err)
			http.Error(w, "internal server error: weather API failure", http.StatusInternalServerError)
			return
		}
		defer weatherResp.Body.Close()

		if weatherResp.StatusCode != http.StatusOK {
			var errResp ErrorResponse
			json.NewDecoder(weatherResp.Body).Decode(&errResp)
			fmt.Printf("WeatherAPI error: %s\n", errResp.Error.Message)
			http.Error(w, "can not find weather for location", http.StatusNotFound)
			return
		}

		var weather WeatherAPIResponse
		if err := json.NewDecoder(weatherResp.Body).Decode(&weather); err != nil {
			http.Error(w, "internal server error: weather API failure", http.StatusInternalServerError)
			return
		}

		c := weather.Current.TempC
		f := c*1.8 + 32
		k := c + 273

		response := map[string]float64{
			"temp_C": c,
			"temp_F": f,
			"temp_K": k,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func main() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	viaCEPURL := "https://viacep.com.br/ws/%s/json/"
	weatherAPIURL := "http://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no"

	http.HandleFunc("/", weatherHandler(client, viaCEPURL, weatherAPIURL))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Servidor rodando na porta %s\n", port)
	http.ListenAndServe(":"+port, nil)
}
