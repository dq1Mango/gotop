package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell"
)

func logger(message string) {
	file, err := os.OpenFile("log.md", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Write the new line to the file
	_, err = file.WriteString("\n" + message)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}
}

// idk where to put this
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

	children := strings.Split(strings.Trim(string(badChildren), " "), " ")

	for _, child := range children {
		fmt.Println(child)
	}

	return proc{string(name), children}

}

func getRootIds() map[string]proc {
	children := getProcessById("1").children

	var processes = make(map[string]proc)

	for _, child := range children {
		processes[child] = getProcessById(child)
	}

	return processes
}

func drawText(s tcell.Screen, x1, y1 int, style tcell.Style, text string) {
	row := y1
	col := x1
	for _, r := range []rune(text) {
		s.SetContent(col, row, r, nil, style)
		col++
	}
}

func listenForInput(s tcell.Screen) {

	quit := func() {
		s.Fini()
		os.Exit(0)
	}

	for {
		// Poll event
		ev := s.PollEvent()

		// Process event
		switch ev := ev.(type) {
		case *tcell.EventResize:
			s.Sync()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				quit()
			}
		}
	}
}

func updateData(s tcell.Screen) {
	defStyle := tcell.StyleDefault.Background(tcell.Color16).Foreground(tcell.Color100)
	highLightStyle := tcell.StyleDefault.Background(tcell.Color160).Foreground(tcell.Color100)

	for {

		rootIDs := getRootIds()

		s.Clear()

		var style tcell.Style
		i := 0
		for key, value := range rootIDs {

			if i == cursorRow {
				style = highLightStyle
			} else {
				style = defStyle
			}

			message := key + ": " + value.name
			drawText(s, 0, i+2, style, message)

			i++
		}

		s.Show()

		time.Sleep(time.Second * 2)

	}
}

var cursorRow = 0

func main() {
	//first log message
	logger("---New Log---")

	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("%+v", err)
	}
	if err := s.Init(); err != nil {
		log.Fatalf("%+v", err)
	}

	// Set default text style
	defStyle := tcell.StyleDefault.Background(tcell.Color16).Foreground(tcell.Color100)
	s.SetStyle(defStyle)

	// Clear screen
	s.Clear()

	go updateData(s)
	go listenForInput(s)

	time.Sleep(time.Minute)

}
