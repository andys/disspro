package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type SelectronicDevice struct {
	Name string `json:"name"`
}

type SelectronicItems struct {
	BatteryInWhToday  float64 `json:"battery_in_wh_today"`
	BatteryInWhTotal  float64 `json:"battery_in_wh_total"`
	BatteryOutWhToday float64 `json:"battery_out_wh_today"`
	BatteryOutWhTotal float64 `json:"battery_out_wh_total"`
	BatterySoc        float64 `json:"battery_soc"`
	BatteryW          float64 `json:"battery_w"`
	FaultCode         int     `json:"fault_code"`
	FaultTs           int     `json:"fault_ts"`
	GenStatus         int     `json:"gen_status"`
	GridInWhToday     float64 `json:"grid_in_wh_today"`
	GridInWhTotal     float64 `json:"grid_in_wh_total"`
	GridOutWhToday    float64 `json:"grid_out_wh_today"`
	GridOutWhTotal    float64 `json:"grid_out_wh_total"`
	GridW             float64 `json:"grid_w"`
	LoadW             float64 `json:"load_w"`
	LoadWhToday       float64 `json:"load_wh_today"`
	LoadWhTotal       float64 `json:"load_wh_total"`
	ShuntW            float64 `json:"shunt_w"`
	SolarWhToday      float64 `json:"solar_wh_today"`
	SolarWhTotal      float64 `json:"solar_wh_total"`
	SolarInverterW    float64 `json:"solarinverter_w"`
	Timestamp         int     `json:"timestamp"`
}

type SelectronicData struct {
	Device    SelectronicDevice `json:"device"`
	ItemCount int               `json:"item_count"`
	Items     SelectronicItems  `json:"items"`
	Now       int               `json:"now"`
}

var (
	latestData      *SelectronicData
	latestDataErr   error
	latestDataMutex sync.RWMutex
	headerPrinted   bool
	headerMutex     sync.Mutex
	printedLines    int
)

// FetchSelectronicData retrieves SelectronicData from the specified URL, using the current Unix timestamp.
func FetchSelectronicData() (*SelectronicData, error) {
	// Get current Unix timestamp using shell date command
	cmd := exec.Command("date", "+%s")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	timestamp := string(out)
	timestamp = strings.TrimSpace(timestamp)

	url := "http://192.168.1.45/cgi-bin/solarmonweb/devices/024ACEDAE30B42800C8C982AA952369F/point?_=" + timestamp

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data SelectronicData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	headerMutex.Lock()
	if !headerPrinted || printedLines%10 == 0 {
		fmt.Printf("%-6s %-10s %-8s %-8s %-10s %-12s %-14s %-14s\n",
			"soc", "battery_w", "load_w", "shunt_w", "solar_w", "total_pv_w", "gen_kwh_today", "load_kwh_today")
		headerPrinted = true
	}
	printedLines++
	headerMutex.Unlock()

	soc := data.Items.BatterySoc
	batteryW := int(data.Items.BatteryW)
	loadW := int(data.Items.LoadW)
	shuntW := int(data.Items.ShuntW)
	solarInverterW := int(data.Items.SolarInverterW)
	totalPVW := solarInverterW + shuntW
	generatorKwhToday := data.Items.GridInWhToday / 1000 // Convert Wh to kWh
	loadKwhToday := data.Items.LoadWhToday / 1000        // Convert Wh to kWh

	fmt.Printf("%-6.1f %-10d %-8d %-8d %-10d %-12d %-14.2f %-14.2f\n",
		soc, batteryW, loadW, shuntW, solarInverterW, totalPVW, generatorKwhToday, loadKwhToday)

	return &data, nil
}

func main() {
	go func() {
		for {
			data, err := FetchSelectronicData()
			latestDataMutex.Lock()
			latestData = data
			latestDataErr = err
			latestDataMutex.Unlock()
			time.Sleep(10 * time.Second)
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		latestDataMutex.RLock()
		data := latestData
		err := latestDataErr
		latestDataMutex.RUnlock()

		if err != nil {
			http.Error(w, "Failed to fetch data: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if data == nil {
			http.Error(w, "No data available yet", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	http.ListenAndServe(":8080", nil)
}
