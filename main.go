package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func buildNodes() {
	
}

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

	// processing resp data
	file, err := os.Open("stopMappings.csv.csv")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	outfile, err := os.Create("mappings.out")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer outfile.Close()

	scanner := bufio.NewScanner(file)

	stationIdMap := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ",")

		if strings.HasSuffix(fields[2], "Light Rail") {
			break
		}

		stationIdMap[fields[4]] = fields[2]

	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}
}
