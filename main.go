package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var htmlPage = `
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Solar Monitor</title>
<style>
body {
    font-family: sans-serif;
    background: #222;
    color: #fff;
    margin: 0;
    padding: 0;
}
.container {
    display: flex;
    flex-direction: row;
    justify-content: center;
    align-items: flex-start;
    min-height: 100vh;
    gap: 2vw;
}
@media (max-width: 700px) {
    .container {
        flex-direction: column;
        align-items: stretch;
    }
}
.card {
    background: #333;
    border-radius: 1em;
    padding: 2em 1em;
    margin: 1em 0;
    flex: 1 1 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    box-shadow: 0 0 10px #0008;
}
.emoji {
    font-size: 3em;
    margin-bottom: 0.5em;
}
.value {
    font-size: 2.5em;
    font-weight: bold;
    margin-bottom: 0.2em;
}
.label {
    font-size: 1.2em;
    color: #aaa;
}
.battery { color: #00e676; }
.pv { color: #ffd600; }
.gen { color: #40c4ff; }
.load { color: #ff1744; }
</style>
</head>
<body>
<div class="container">
    <div class="card">
        <div class="emoji battery">🔋</div>
        <div class="value" id="soc">--%</div>
        <div class="label">Battery SOC</div>
        <div class="emoji pv">☀️</div>
        <div class="value" id="pv">-- kW</div>
        <div class="label">Total PV</div>
        <div class="emoji gen">🏭</div>
        <div class="value" id="gen">-- kW</div>
        <div class="label">Generator</div>
        <div class="emoji load">🔌</div>
        <div class="value" id="load">-- kW</div>
        <div class="label">Load</div>
    </div>
    <div class="card" id="second-col">
        <!-- You can add more info here later -->
    </div>
</div>
<script>
function updateData() {
    fetch('/data').then(r => r.json()).then(data => {
        document.getElementById('soc').textContent = (data.items.battery_soc || 0).toFixed(1) + '%';
        let totalPV = ((data.items.solarinverter_w || 0) + (data.items.shunt_w || 0)) / 1000;
        document.getElementById('pv').textContent = totalPV.toFixed(2) + ' kW';
        document.getElementById('gen').textContent = ((data.items.grid_w || 0) / 1000).toFixed(2) + ' kW';
        document.getElementById('load').textContent = ((data.items.load_w || 0) / 1000).toFixed(2) + ' kW';
    });
}
setInterval(updateData, 5000);
updateData();
</script>
</body>
</html>
`

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
	latestData        *SelectronicData
	latestDataErr     error
	globalMutex       sync.RWMutex
	headerPrinted     bool
	printedLines      int
	latestTemperature float64
)

// FetchSelectronicData retrieves SelectronicData from the specified URL, using the current Unix timestamp.
func FetchSelectronicData() (*SelectronicData, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

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

	// Fetch temperature from temper/temper
	tempCmd := exec.Command("temper/temper")
	tempOut, tempErr := tempCmd.Output()
	var tempVal float64
	if tempErr == nil {
		tempStr := strings.TrimSpace(string(tempOut))
		if val, err := strconv.ParseFloat(tempStr, 64); err == nil {
			tempVal = val
		}
	}

	globalMutex.Lock()
	if !headerPrinted || printedLines%10 == 0 {
		fmt.Printf("%-7s %-10s %-8s %-8s %-10s %-12s %-14s %-14s %-8s\n",
			"soc", "battery_w", "load_w", "shunt_w", "solar_w", "total_pv_w", "gen_kwh_today", "load_kwh_today", "temp_C")
		headerPrinted = true
	}
	printedLines++
	if tempErr == nil {
		latestTemperature = tempVal
	}
	globalMutex.Unlock()

	soc := data.Items.BatterySoc
	batteryW := int(data.Items.BatteryW)
	loadW := int(data.Items.LoadW)
	shuntW := int(data.Items.ShuntW)
	solarInverterW := int(data.Items.SolarInverterW)
	totalPVW := solarInverterW + shuntW
	generatorKwhToday := data.Items.GridInWhToday / 1000 // Convert Wh to kWh
	loadKwhToday := data.Items.LoadWhToday / 1000        // Convert Wh to kWh

	globalMutex.RLock()
	tempC := latestTemperature
	globalMutex.RUnlock()

	fmt.Printf("%-6.1f%% %-10d %-8d %-8d %-10d %-12d %-14.2f %-14.2f %-8.2f\n",
		soc, batteryW, loadW, shuntW, solarInverterW, totalPVW, generatorKwhToday, loadKwhToday, tempC)

	return &data, nil
}

func main() {
	go func() {
		for {
			data, err := FetchSelectronicData()
			globalMutex.Lock()
			latestData = data
			latestDataErr = err
			globalMutex.Unlock()
			time.Sleep(10 * time.Second)
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlPage)
	})

	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		globalMutex.RLock()
		data := latestData
		err := latestDataErr
		globalMutex.RUnlock()

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
