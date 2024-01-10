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
	db, err := sql.Open("sqlite3", "data/data.db")
	if err != nil {
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

	// reduce repetition
	for i, line := range lines[1:] {
		if i == insert_limit {
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
		parent := "'" + line[9] + "'"
		// is this valid?
		if line[9] == "" {
			parent = "NULL"
		}

		stops = append(stops, fmt.Sprintf("('%s','%s','%s',%s)", id, code, name, parent))
	}

	if len(stops) > 0 {
		qry := strings.Join(stops, ",")
		full_qry := "INSERT INTO stop (id, code, name, parent_id) VALUES " + qry
		db.Exec(full_qry)
	}

}

func getCalendar() map[string]bool {
	file, err := os.Open("trainData/calendar.txt")
	if err != nil {
		log.Fatal("couldn't open calendar.txt")
		return nil
	}
	db, err := sql.Open("sqlite3", "data/data.db")
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
	// reduce repetition
	for i, line := range lines[1:] {
		if i == insert_limit {
			qry := strings.Join(services, ",")
			// get mon-sunday
			full_qry := insertionQry + qry
			_, err := db.Exec(full_qry)
			if err != nil {
				log.Fatal(err)
			}
			services = []string{}
		}

		curr := time.Now().Format("20060102")
		start := line[8]
		id := line[0]
		line[0], line[8], line[9] = fmt.Sprintf("'%s'", line[0]), fmt.Sprintf("'%s'", line[8]), fmt.Sprintf("'%s'", line[9])
		// fmt.Printf("%s, %s, %d\n", curr, start, today)

		dayData := line[1:8]
		// consider including services for the next day as well
		if dayData[today] == "1" && curr == start {
			fmt.Println("cheesecake shops man")
			services = append(services, "(" + strings.Join(line, ",") + ")")
			serviceMap[id] = true
		}
	}

	if len(services) > 0 {
		qry := strings.Join(services, ",")
		full_qry := insertionQry + qry
		fmt.Println(full_qry)
		_, err := db.Exec(full_qry)
		if err != nil {
			log.Fatal(err)
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
	db, err := sql.Open("sqlite3", "data/data.db")
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
		if i == insert_limit && len(trips) > 0 {
			qry := strings.Join(trips, ",")
			full_qry := "INSERT INTO trip (route_id, service_id, id, headsign, short_name, direction_id, block_id, shape_id, wheelchair) VALUES " + qry
			_, err := db.Exec(full_qry)
			if err != nil {
				log.Fatal(err)
			}
			trips = []string{}
		}
		if validService[line[1]] == true {
			var quotes []int = []int{0, 1, 2, 3, 4, 6, 7}
			for _, i := range quotes {
				line[i] = "'" + line[i] + "'"
			}
			fmt.Println("shit")
			trips = append(trips, "("+ strings.Join(line, ",") +")")
			tripReady[line[0]] = true
		}
	}

	if len(trips) > 0 {
		qry := strings.Join(trips, ",")
		fmt.Println("2")
		full_qry := "INSERT INTO trip (route_id, service_id, id, headsign, short_name, direction_id, block_id, shape_id, wheelchair) VALUES " + qry
		_, err := db.Exec(full_qry)
		if err != nil {
			log.Fatal(err)
		}
	}

	return tripReady
}

func terminate() {
	err := os.RemoveAll("trainData")
	if err != nil {
		log.Fatal(err)
	}
}

// func getStopTimes() {
// 	file, err := os.Open("trainData/stop_times.txt")
// 	if err != nil {
// 		return
// 	}
// 	db, err := sql.Open("sqlite3", "data/data.db")
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	db_qry := `
// 	CREATE TABLE IF NOT EXISTS stop_time (
// 		trip_id TEXT NOT NULL,
// 		stop_id TEXT NOT NULL,
// 		arrival_time TEXT NOT NULL,
// 		departure_time TEXT NOT NULL,
// 		stop_sequence INT NOT NULL,
// 		FOREIGN KEY (trip_id) REFERENCES trip(id)
// 		FOREIGN KEY (stop_id) REFERENCES stop(id)
// 	);
// `
// 	createDb(db, db_qry)

// 	// WE DON'T WANT ALL THE DATA, JUST THE ONES AFTER CURERNT
// 	// DIFF SERVICE IDS WILL SHOW WHEN CERTAIN TRIPS ARE AVAILABLE

// 	// IDEA 1:
// 	// LOOK AT SERVICE IDS OVER THE NEXT 24-48 HOURS, BASED ON THAT:
// 	// // NARROW DOWN TRIPS THAT RUN IN CURRENT DAY + TOMORROW
// 	// // THEN CONSIDER CURRENT TIME AND BEYOND (NOT INTERESTED IN PAST)
// 	// // HOPEFULLY THIS REDUCES THE SIZE OF THE SQL TABLE BY A LOT

// 	// HOW TO REDUCE REPETITION OF SQL DB CREATION??

// 	// NEED TO ADD calendar.txt before trips.txt, trips.txt before stop_times.txt

// 	// THIS SHOULD GIVE ME THE NECESSARY DATA TO BUILD THE EDGES TO COMMENCE ROUTING

// }

func main() {
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

	cmd := exec.Command("/bin/sh", "unpack.sh")
	_, err = cmd.Output()
	if err != nil {
		fmt.Println("Error running timetable unpacking shell script: ", err)
		return
	}

	err = os.MkdirAll("data", 0755)
	if err != nil {
		log.Fatal(err)
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

	fmt.Println(keys)
	tripSubset := getTrips(serviceCalendar)
	if tripSubset == nil {
		log.Fatal("something fucked with getTrips ngl mate")
	}
	// getStopTimes()

	// processing resp data
	// file, err := os.Open("stopMappings.csv.csv")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// defer file.Close()

	// outfile, err := os.Create("mappings.out")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// defer outfile.Close()

	// scanner := bufio.NewScanner(file)

	// stationIdMap := make(map[string]string)

	// for scanner.Scan() {
	// 	line := scanner.Text()
	// 	fields := strings.Split(line, ",")

	// 	if strings.HasSuffix(fields[2], "Light Rail") {
	// 		break
	// 	}

	// 	stationIdMap[fields[4]] = fields[2]

	// }

	// if err := scanner.Err(); err != nil {
	// 	fmt.Println(err)
	// }
		terminate()


}
