package main

import (
	// "bufio"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Stop struct {
	trips map[string]bool
}

// id to map checking for trip id
// only use this for fast lookup
// trip_id = 'platform' if this is an 'exchange' stop
var network map[string]map[string]map[string]bool = make(map[string]map[string]map[string]bool)

// neighbours[stop_id] = [(id1, trip_id), (id2, trip_id), (id3, trip_id)]
var neighbours map[string][][2]string = make(map[string][][2]string)

var trips_ []string

func ensureNodeExists(node string, tripID string) {
	if network[node] == nil {
		network[node] = make(map[string]map[string]bool)
	}
	if network[node][tripID] == nil {
		network[node][tripID] = make(map[string]bool)
	}
}

func addEdge(from, to, tripID string) {
	ensureNodeExists(from, tripID)
	ensureNodeExists(to, tripID)

	if !network[from][tripID][to] {
		neighbours[from] = append(neighbours[from], [2]string{to, tripID})
		neighbours[to] = append(neighbours[to], [2]string{from, tripID})
	}

	network[from][tripID][to] = true
	network[to][tripID][from] = true
}

// func getStop() {
// 	// sql query
// }

type tripRow struct {
	sequence int
	trip_id  string
}

type AdjStop struct {
	id       string
	sequence int
}

func terminate() {
	err := os.RemoveAll("trainData")
	if err != nil {
		log.Fatal(err)
	}
}

func handleError(err error) {
	if err != nil {
		terminate()
		log.Fatal(err)
	}
}

func connectNetwork() {

	db, err := sql.Open("sqlite3", "database/data.db")
	handleError(err)

	defer db.Close()

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_trip_id ON stop_time(trip_id);`)
	handleError(err)

	qry := `SELECT id FROM stop_time WHERE trip_id = ? ORDER BY sequence;`
	stmt, err := db.Prepare(qry)
	handleError(err)

	for _, trip_id := range trips_ {
		rows, err := stmt.Query(trip_id)
		handleError(err)
		if err == sql.ErrNoRows {
			fmt.Println("No rows were returned!")
			continue
		}
		defer rows.Close()

		prev := ""
		curr := ""
		// Iterate through the result set
		for rows.Next() {
			var stopID string
			if err := rows.Scan(&stopID); err != nil {
				fmt.Println("Error scanning row:", err)
				continue
			}

			curr = stopID
			if prev != "" {
				addEdge(prev, curr, trip_id)
			}
			prev = curr
		}

	}

	fmt.Println(neighbours)
}

func createDb(db *sql.DB, qry string) {

	createTableSQL := qry
	_, err := db.Exec(createTableSQL)
	if err != nil {
		log.Fatal(err)
	}

}

// proper error handling for query execution
func getStops() {
	file, err := os.Open("trainData/stops.txt")
	if err != nil {
		return
	}
	db, err := sql.Open("sqlite3", "database/data.db")
	if err != nil {
		fmt.Println("crazy")
		log.Fatal(err)
	}

	db_qry := `
	CREATE TABLE IF NOT EXISTS stop (
		id TEXT PRIMARY KEY,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		parent_id TEXT,
		FOREIGN KEY (parent_id) REFERENCES stop(id)
	);
`
	createDb(db, db_qry)

	reader := csv.NewReader(file)
	lines, err := reader.ReadAll()
	if err != nil {
		return
	}

	insert_limit := 1000
	var stops []string

	for i, line := range lines[1:] {
		if len(stops) == insert_limit || (len(stops) > 0 && i == len(lines)) {
			qry := strings.Join(stops, ",")
			full_qry := "INSERT INTO stop (id, code, name, parent_id) VALUES " + qry
			_, err := db.Exec(full_qry)
			if err != nil {
				log.Fatal(err)
			}
			stops = []string{}
		}

		id := line[0]
		code := line[1]
		name := line[2]
		parent_station := line[9]
		// add stop to graph
		if network[id] == nil {
			network[id] = make(map[string]map[string]bool)
			if parent_station != "" {
				addEdge(id, parent_station, "platform")
			}
		}

		parent := "'" + line[9] + "'"
		// is this valid?
		if line[9] == "" {
			parent = "NULL"
		}

		stops = append(stops, fmt.Sprintf("('%s','%s','%s',%s)", id, code, name, parent))
	}

}

func getCalendar() map[string]bool {
	file, err := os.Open("trainData/calendar.txt")
	if err != nil {
		log.Fatal("couldn't open calendar.txt")
		return nil
	}
	db, err := sql.Open("sqlite3", "database/data.db")
	if err != nil {
		log.Fatal(err)
	}

	db_qry := `
	CREATE TABLE IF NOT EXISTS calendar (
		id TEXT NOT NULL,
		mon INT NOT NULL,
		tue INT NOT NULL,
		wed INT NOT NULL,
		thu INT NOT NULL,
		fri INT NOT NULL,
		sat INT NOT NULL,
		sun INT NOT NULL,
		start_date TEXT NOT NULL,
		end_date TEXT NOT NULL
	);	
	`

	createDb(db, db_qry)

	reader := csv.NewReader(file)
	lines, err := reader.ReadAll()
	if err != nil {
		return nil
	}

	insert_limit := 1000
	var services []string
	// should respond to user day query
	today := (int(time.Now().Weekday()) + 6) % 7

	serviceMap := make(map[string]bool)
	insertionQry := "INSERT INTO calendar (id, mon, tue, wed, thu, fri, sat, sun, start_date, end_date) VALUES "
	curr := time.Now().Format("20060102")

	// reduce repetition
	for i, line := range lines[1:] {
		if len(services) == insert_limit || (len(services) > 0 && i == len(services)) {
			qry := strings.Join(services, ",")
			// get mon-sunday
			full_qry := insertionQry + qry
			_, err := db.Exec(full_qry)
			if err != nil {
				log.Fatal(err)
			}
			services = []string{}
		}

		start := line[8]
		end := line[9]
		id := line[0]
		line[0], line[8], line[9] = fmt.Sprintf("'%s'", line[0]), fmt.Sprintf("'%s'", line[8]), fmt.Sprintf("'%s'", line[9])
		// fmt.Printf("%s, %s, %d\n", curr, start, today)

		dayData := line[1:8]
		// consider including services for the next day as well
		if dayData[today] == "1" && curr >= start && curr <= end {
			services = append(services, "("+strings.Join(line, ",")+")")
			serviceMap[id] = true
		}
	}

	return serviceMap

}

func getTrips(validService map[string]bool) map[string]bool {
	file, err := os.Open("trainData/trips.txt")
	if err != nil {
		log.Fatal(err)
		return nil
	}
	db, err := sql.Open("sqlite3", "database/data.db")
	if err != nil {
		log.Fatal(err)
	}

	// get trips returning true with serviceCalenar dictionary. then at stop times, use selected trips to reduce data space to only relevant fields.

	db_qry := `
	CREATE TABLE IF NOT EXISTS trip (
		route_id TEXT NOT NULL,
		service_id TEXT NOT NULL,
		id TEXT NOT NULL,
		headsign TEXT,
		short_name TEXT NOT NULL,
		direction_id INT NOT NULL,
		block_id TEXT,
		shape_id TEXT,
		wheelchair INT NOT NULL,
		FOREIGN KEY (service_id) REFERENCES calendar(id)
	);
	`
	createDb(db, db_qry)

	reader := csv.NewReader(file)
	lines, err := reader.ReadAll()
	if err != nil {
		return nil
	}

	insert_limit := 1000
	var trips []string
	tripReady := make(map[string]bool)
	// reduce repetition
	for i, line := range lines[1:] {
		if len(trips) == insert_limit || (len(trips) > 0 && i == len(lines)) {
			qry := strings.Join(trips, ",")
			full_qry := "INSERT INTO trip (route_id, service_id, id, headsign, short_name, direction_id, block_id, shape_id, wheelchair) VALUES " + qry
			_, err := db.Exec(full_qry)
			if err != nil {
				log.Fatal(err)
			}
			trips = []string{}
		}
		if validService[line[1]] == true {
			trips_ = append(trips_, line[2])
			tripReady[line[2]] = true
			var quotes []int = []int{0, 1, 2, 3, 4, 6, 7}
			for _, i := range quotes {
				line[i] = "'" + line[i] + "'"
			}
			trips = append(trips, "("+strings.Join(line, ",")+")")
		}
	}

	return tripReady
}

func getStopTimes(trips map[string]bool) {
	var keys []string
	for key := range trips {
		keys = append(keys, key)
	}
	print(keys)

	file, err := os.Open("trainData/stop_times.txt")
	if err != nil {
		return
	}
	db, err := sql.Open("sqlite3", "database/data.db")
	if err != nil {
		log.Fatal(err)
	}

	db_qry := `
	CREATE TABLE IF NOT EXISTS stop_time (
		trip_id TEXT NOT NULL,
		arrival_time TEXT NOT NULL,
		departure_time TEXT NOT NULL,
		id TEXT NOT NULL,
		sequence INT NOT NULL,
		headsign TEXT,
		FOREIGN KEY (trip_id) REFERENCES trip(id),
		FOREIGN KEY (id) REFERENCES stop(id)
	);
`

	createDb(db, db_qry)

	reader := csv.NewReader(file)
	lines, err := reader.ReadAll()
	if err != nil {
		return
	}

	insert_limit := 1000

	var stop_times []string
	for i, line := range lines[1:] {
		if len(stop_times) == insert_limit || (len(stop_times) > 0 && i == len(lines)) {
			qry := strings.Join(stop_times, ",")
			// print(qry)
			full_qry := "INSERT INTO stop_time (trip_id, arrival_time, departure_time, id, sequence, headsign) VALUES " + qry
			_, err := db.Exec(full_qry)
			if err != nil {
				log.Fatal(err)
			}
			stop_times = []string{}
		}

		if trips[line[0]] == true {
			var text []int = []int{0, 1, 2, 3, 5}
			for _, i := range text {
				line[i] = "'" + line[i] + "'"
			}

			stop_times = append(stop_times, "("+strings.Join(line[:len(line)-3], ",")+")")
		}
	}

	// WE DON'T WANT ALL THE DATA, JUST THE ONES AFTER CURERNT
	// DIFF SERVICE IDS WILL SHOW WHEN CERTAIN TRIPS ARE AVAILABLE

	// IDEA 1:
	// LOOK AT SERVICE IDS OVER THE NEXT 24-48 HOURS, BASED ON THAT:
	// // NARROW DOWN TRIPS THAT RUN IN CURRENT DAY + TOMORROW
	// // THEN CONSIDER CURRENT TIME AND BEYOND (NOT INTERESTED IN PAST)
	// // HOPEFULLY THIS REDUCES THE SIZE OF THE SQL TABLE BY A LOT

	// HOW TO REDUCE REPETITION OF SQL DB CREATION??

	// NEED TO ADD calendar.txt before trips.txt, trips.txt before stop_times.txt

	// THIS SHOULD GIVE ME THE NECESSARY DATA TO BUILD THE EDGES TO COMMENCE ROUTING

}

func main() {
	// err := os.RemoveAll("data")
	// if err != nil {
	// 	log.Fatal("shit")
	// }
	token := APIKey
	url := "https://api.transport.nsw.gov.au/v1/gtfs/schedule/sydneytrains"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error creating request: ", err)
		return
	}

	req.Header.Add("accept", "application/octet-stream")
	req.Header.Add("Authorization", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error making request: ", err)
		return
	}
	defer resp.Body.Close()

	problem := os.MkdirAll("trainData", 0755)
	if problem != nil {
		log.Fatal(problem)
	}

	out, err := os.Create("trainData/sydneytrains_GTFS_20231215140100.zip")
	if err != nil {
		fmt.Println("uh oh! you fucked up, badly. 00")
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println("uh oh! you fucked up, badly.")
	}

	err = os.MkdirAll("database", 0755)
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command("/bin/sh", "unpack.sh")
	_, err = cmd.Output()
	if err != nil {
		fmt.Println("Error running timetable unpacking shell script: ", err)
		return
	}

	getStops()
	serviceCalendar := getCalendar()
	if serviceCalendar == nil {
		log.Fatal("something fucked with getCalendar fn")
		return
	}
	var keys []string
	for key := range serviceCalendar {
		keys = append(keys, key)
	}

	tripSubset := getTrips(serviceCalendar)
	if tripSubset == nil {
		log.Fatal("something fucked with getTrips ngl mate")
	}
	getStopTimes(tripSubset)

	connectNetwork()

	// fmt.Println(network)
	// add stops to indexed graph
	// connect stops based on stop times
	//

	// for {
	// 	var from string
	// 	var trip_id string
	// 	var to string
	// 	fmt.Print("<from> <trip id> <to>:")
	// 	_, err := fmt.Scanf("%s %s %s", &from, &trip_id, &to)
	// 	if err != nil {
	// 		fmt.Printf("err: %s\n", err)
	// 		continue
	// 	}

	// 	fmt.Printf("%t", network[from][trip_id][to])

	// }

	terminate()

}
