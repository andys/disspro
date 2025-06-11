package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
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
html, body {
    height: 100%;
    width: 100%;
}
.container {
    display: flex;
    flex-direction: row;
    justify-content: center;
    align-items: flex-start;
    min-height: 100vh;
    gap: 4vw;
    padding: 3vw 2vw 3vw 2vw;
    margin: 0 auto;
    box-sizing: border-box;
    width: 100%;
    max-width: 1200px;
}
@media (max-width: 850px) {
    body { 
        background: #444 !important; 
    }
    .container {
        flex-direction: column;
        align-items: stretch;
    }
    .container .label {
        font-size: 1.2em !important;
    }
    .container .value {
        font-size: 1.5em !important;
    }
    .container .arrow {
        font-size: 1.3em !important;
        margin-left: -1.0em !important;
        margin-right: 0 !important;
        position: static !important;
        display: inline-block !important;
        vertical-align: middle !important;
    }
    .container .emoji {
        font-size: 1.1em !important;
    }
}
.card {
    background: #333;
    border-radius: 1.5em;
    padding: 3em 2em;
    margin: 2em 1em;
    flex: 1 1 0;
    display: flex;
    flex-direction: column;
    align-items: stretch;
    box-shadow: 0 0 18px #000a;
}
.rightcard {
    background: #292929;
    border-radius: 1.5em;
    padding: 2.2em 1.5em;
    margin: 2em 1em;
    flex: 1 1 0;
    display: flex;
    flex-direction: column;
    align-items: stretch;
    box-shadow: 0 0 18px #000a;
}
.rightcard .label {
    font-size: 1.2em;
}
.rightcard .value {
    font-size: 1.6em;
}
.row {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    margin: 0.8em 0;
    gap: 2em;
    padding: 0.5em 0.5em;
}

.rightcard .row {
    margin: 0.7em 0;
    padding: 0.3em 0.5em;
}

.label {
    font-size: 2.0em;
    color: #aaa;
    display: flex;
    align-items: center;
    gap: 0.7em;
    padding: 0.3em 0.3em 0.3em 0.3em;
    margin-right: 0.7em;
}
.value {
    font-size: 2.5em;
    font-weight: bold;
    min-width: 4em;
    text-align: right;
    margin-bottom: 0;
    padding: 0.3em 0.6em 0.3em 0.3em;
}
.arrow {
    font-size: 2.2em;
    font-weight: bold;
		margin-left: -4.0em;
}
.emoji {
    font-size: 1.7em;
    margin-bottom: 0;
    margin-right: 0.5em;
}
.battery { color: #00e676; }
.pv { color: #ffd600; }
.load { color: #ff1744; }
</style>
</head>
<body>
<div class="container">
    <div class="card">
        <div class="row">
            <div class="label">
                <span class="emoji battery">🔋</span>
                Battery
            </div>
            <div class="value" id="soc">--%</div><span class="arrow" id="battery-arrow"></span>
        </div>
        <div class="row">
            <div class="label"><span class="emoji"> </span><div id="battery-time-label">--</div></div>
            <div class="value" id="battery-time">-- h</div>
        </div>
        <div class="row">
            <div class="label">
                <span class="emoji pv">⚡</span> Generation
            </div>
            <div class="value" id="pv">-- kW</div>
        </div>
        <div class="row">
            <div class="label">
                <span class="emoji load">🔌</span> Consumption
            </div>
            <div class="value" id="load">-- kW</div>
        </div>
    </div>
    <div class="card rightcard" id="second-col">
        <div class="row">
            <div class="label">Temperature</div>
            <div class="value" id="temp">-- °C</div>
        </div>
        <div class="row">
            <div class="label">Consumed Today</div>
            <div class="value" id="kwh-consumed">-- kWh</div>
        </div>
        <div class="row">
            <div class="label">🔋 Battery kW</div>
            <div class="value" id="battery-kw">-- kW</div>
        </div>
        <div class="row">
            <div class="label">Generator Today</div>
            <div class="value" id="gen-made">-- kWh</div>
        </div>
        <div class="row">
            <div class="label">🚜 Generator</div>
            <div class="value" id="gen-status">--</div>
        </div>
        <div class="row">
            <div class="label">🚨 Fault</div>
            <div class="value" id="fault-status">--</div>
        </div>
    </div>
</div>
<script>
function updateData() {
    fetch('/data').then(r => r.json()).then(data => {
        let socVal = (data.items.battery_soc || 0).toFixed(1) + ' %';
        document.getElementById('soc').textContent = socVal;
        let batteryW = data.items.battery_w || 0;
        let arrow = '';
        if (batteryW < -5) arrow = '▲';      // Charging when battery is negative (power flowing in)
        else if (batteryW > 5) arrow = '▼'; // Discharging (power flowing out)
        else arrow = '';                      // Idle/neutral
        document.getElementById('battery-arrow').textContent = arrow;

        // Calculate and display "Full in"/"Empty in"
        let avgBatteryW = data.avg_battery_w || 0;
        let hours = 0;
        let label = '--';				
        if (avgBatteryW > 5 && data.hours_until_empty < 24) {
            // Discharging
            hours = data.hours_until_empty || 0;
            label = '🚫 Empty in';
        } else if (avgBatteryW < -5 && data.hours_until_full < 24) {
            // Charging
            hours = data.hours_until_full || 0;
            label = '✅ Full in';
        } else {
            hours = 0;
            label = '--';
        }
        document.getElementById('battery-time-label').textContent = label;
        document.getElementById('battery-time').textContent = hours > 0 ? hours.toFixed(1) + ' h' : '-- h';

        let totalGen = ((data.items.solarinverter_w || 0) + (data.items.shunt_w || 0) + Math.abs(data.items.grid_w || 0)) / 1000;
        document.getElementById('pv').textContent = (Math.floor(totalGen * 10) / 10).toFixed(1) + ' kW';
        document.getElementById('load').textContent = ((data.items.load_w || 0) / 1000).toFixed(1) + ' kW';
        
        // Right card updates
        document.getElementById('temp').textContent = (data.temperature !== undefined ? data.temperature.toFixed(1) : '--') + ' °C';
        document.getElementById('kwh-consumed').textContent = (data.items.load_wh_today || 0).toFixed(1) + ' kWh';
        document.getElementById('gen-made').textContent = (data.items.grid_in_wh_today || 0).toFixed(1) + ' kWh';
        document.getElementById('battery-kw').textContent = ((data.items.battery_w || 0) / 1000).toFixed(1) + ' kW';
        document.getElementById('gen-status').textContent = (data.items.gen_status && data.items.gen_status !== 0) ? 'On' : 'Off';
        document.getElementById('fault-status').textContent = (data.items.fault_code && data.items.fault_code !== 0) ? 'YES' : 'No';
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
	itemsHistory      []SelectronicItems
	itemsHistoryMutex sync.Mutex

	prevGeneratorKwhToday float64
)

const BatterykWh = 24
const BatteryEmptyAtSoc = 30

// AverageBatteryW returns the average BatteryW from itemsHistory.
func AverageBatteryW() float64 {
	itemsHistoryMutex.Lock()
	defer itemsHistoryMutex.Unlock()
	if len(itemsHistory) == 0 {
		return 0
	}
	var sum float64
	for _, item := range itemsHistory {
		sum += item.BatteryW
	}
	return sum / float64(len(itemsHistory))
}

// AverageTotalGeneration returns the average of (GridW + ShuntW + SolarInverterW) from itemsHistory.
func AverageTotalGeneration() float64 {
	itemsHistoryMutex.Lock()
	defer itemsHistoryMutex.Unlock()
	if len(itemsHistory) == 0 {
		return 0
	}
	var sum float64
	for _, item := range itemsHistory {
		sum += math.Abs(item.GridW) + item.ShuntW + item.SolarInverterW
	}
	return sum / float64(len(itemsHistory))
}

// AverageLoadW returns the average LoadW from itemsHistory.
func AverageLoadW() float64 {
	itemsHistoryMutex.Lock()
	defer itemsHistoryMutex.Unlock()
	if len(itemsHistory) == 0 {
		return 0
	}
	var sum float64
	for _, item := range itemsHistory {
		sum += item.LoadW
	}
	return sum / float64(len(itemsHistory))
}

// HoursUntilFull returns the estimated hours until the battery is full (100% SoC),
// only if AverageBatteryW is negative (charging). Returns 0 if not charging or data unavailable.
func HoursUntilFull() float64 {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	if latestData == nil {
		return 0
	}
	currentSoc := latestData.Items.BatterySoc
	// Calculate current kWh in battery
	currentKWh := (currentSoc / 100.0) * BatterykWh
	// kWh needed to reach full
	kWhNeeded := BatterykWh - currentKWh

	avgBatteryW := AverageBatteryW()
	if avgBatteryW >= 0 {
		return 0 // Not charging
	}
	// avgBatteryW is negative when charging (power flowing into battery)
	chargingKW := -avgBatteryW / 1000.0
	if chargingKW <= 0 {
		return 0
	}
	return kWhNeeded / chargingKW
}

// HoursUntilEmpty returns the estimated hours until the battery is empty (reaches BatteryEmptyAtSoc% SoC),
// only if AverageBatteryW is positive (discharging). Returns 0 if not discharging or data unavailable.
func HoursUntilEmpty() float64 {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	if latestData == nil {
		return 0
	}
	currentSoc := latestData.Items.BatterySoc
	// Calculate current kWh in battery
	currentKWh := (currentSoc / 100.0) * BatterykWh
	// kWh available until empty threshold
	emptyKWh := (BatteryEmptyAtSoc / 100.0) * BatterykWh
	kWhAvailable := currentKWh - emptyKWh

	avgBatteryW := AverageBatteryW()
	if avgBatteryW <= 0 {
		return 0 // Not discharging
	}
	// avgBatteryW is positive when discharging (power flowing out of battery)
	dischargeKW := avgBatteryW / 1000.0
	if dischargeKW <= 0 {
		return 0
	}
	return kWhAvailable / dischargeKW
}

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
	tempCmd := exec.Command("./temper/temper", "-c")
	tempOut, tempErr := tempCmd.Output()
	var tempVal float64
	if tempErr == nil {
		tempStr := strings.TrimSpace(string(tempOut))
		tempStr = strings.TrimSuffix(tempStr, "C")
		tempStr = strings.TrimSpace(tempStr)
		if val, err := strconv.ParseFloat(tempStr, 64); err == nil {
			tempVal = val
		}
	} else {
		fmt.Printf("Error getting temp: %v\n", tempErr)
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
	totalPVW := solarInverterW + shuntW + int(math.Abs(float64(data.Items.GridW)))
	generatorKwhToday := data.Items.GridInWhToday / 1000 // Convert Wh to kWh
	loadKwhToday := data.Items.LoadWhToday               // Already in kWh

	// Set GenStatus = 1 if generatorKwhToday has increased since last fetch
	if generatorKwhToday > prevGeneratorKwhToday {
		data.Items.GenStatus = 1
	} else {
		data.Items.GenStatus = 0
	}

	globalMutex.RLock()
	tempC := latestTemperature
	globalMutex.RUnlock()

	fmt.Printf("%-3.1f%%   %-10d %-8d %-8d %-10d %-12d %-14.2f %-14.2f %-8.2f\n",
		soc, batteryW, loadW, shuntW, solarInverterW, totalPVW, generatorKwhToday, loadKwhToday, tempC)

	// Maintain a rolling history of the last 30 SelectronicItems
	itemsHistoryMutex.Lock()
	itemsHistory = append(itemsHistory, data.Items)
	if len(itemsHistory) > 50 {
		itemsHistory = itemsHistory[1:]
	}
	itemsHistoryMutex.Unlock()

	prevGeneratorKwhToday = generatorKwhToday

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
		type OutData struct {
			*SelectronicData
			Temperature     float64 `json:"temperature"`
			AvgBatteryW     float64 `json:"avg_battery_w"`
			HoursUntilFull  float64 `json:"hours_until_full"`
			HoursUntilEmpty float64 `json:"hours_until_empty"`
		}
		out := OutData{
			SelectronicData: data,
			Temperature:     latestTemperature,
			AvgBatteryW:     AverageBatteryW(),
			HoursUntilFull:  HoursUntilFull(),
			HoursUntilEmpty: HoursUntilEmpty(),
		}
		json.NewEncoder(w).Encode(out)
	})

	http.ListenAndServe(":80", nil)
}
