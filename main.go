package main

import (
	"fmt"
	"os"
	"strings"
)

type proc struct {
	name     string
	children []string
}

func getProcessById(id string) proc {
	name, err := os.ReadFile("/proc/" + id + "/comm")
	if err != nil {
		fmt.Println("Error reading file: ", err)
	}

	badChildren, err := os.ReadFile("/proc/" + id + "/task/" + id + "/children")
	if err != nil {
		fmt.Println("Error reading file: ", err)
	}

	children := strings.Split(string(badChildren), " ")

	return proc{string(name), children}

}

func getRootIds() []proc {
	children := getProcessById("1").children

	var processes []proc

	for child := 0; child < len(children); child++ {
		processes = append(processes, getProcessById(children[child]))
	}

	return processes
}
func main() {
	fmt.Println(getRootIds())

}
