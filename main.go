package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell"
)

type logger struct {
	f *os.File
}

func newLogger() *logger {
	file, err := os.OpenFile("log.md", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		panic("opening log")
	}

	return &logger{file}
}

func (l *logger) info(message string) {
	// Write the new line to the file
	_, err := l.f.WriteString("\n" + message)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}
}

func (l *logger) close() {
	l.f.Close()
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

func refreshScreen(s tcell.Screen, processes map[string]proc, cursor int) {

	defStyle := tcell.StyleDefault.Background(tcell.Color16).Foreground(tcell.Color100)
	highLightStyle := tcell.StyleDefault.Background(tcell.Color160).Foreground(tcell.Color100)

	s.Clear()

	i := 0
	for key, value := range processes {

		/*if i == cursorRow {
			style = highLightStyle
		} else {
			style = defStyle
		}*/

		message := key + ": " + value.name
		drawText(s, 0, i+2, defStyle, message)

		if i == cursor {
			drawText(s, 0, i+2, highLightStyle, message)
		}

		i++
	}
	s.Show()

}

func syncChannels(s tcell.Screen, data chan map[string]proc, cursor chan int, l *logger) {
	cursorRow := 0
	var processes map[string]proc

	for {
		select {
		case processes = <-data:
			refreshScreen(s, processes, cursorRow)
		case cursorChange := <-cursor:
			maybeCursor := cursorRow + cursorChange
			if maybeCursor < 0 || maybeCursor > len(processes) {
				continue
			}

			cursorRow = maybeCursor
			refreshScreen(s, processes, cursorRow)

		}
	}
}

func listenForInput(s tcell.Screen, input chan int, l *logger) {

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

			switch ev.Key() {
			case tcell.KeyRune:

				l.info("we got the rune: " + string(ev.Rune()))

				switch string(ev.Rune()) {
				case "j":
					input <- 1

				case "k":
					input <- -1
				}

			case tcell.KeyEscape, tcell.KeyCtrlC:
				quit()
			}
		}
	}
}

func updateData(data chan map[string]proc, l *logger) {

	for {

		data <- getRootIds()

		time.Sleep(time.Second * 2)

	}
}

func main() {
	//first log message
	logger := newLogger()
	defer logger.close()

	logger.info("---New Log---")

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

	quit := func() {
		// You have to catch panics in a defer, clean up, and
		// re-raise them - otherwise your application can
		// die without leaving any diagnostic trace.
		maybePanic := recover()
		s.Fini()
		if maybePanic != nil {
			panic(maybePanic)
		}
	}
	defer quit()

	data := make(chan map[string]proc)
	input := make(chan int)

	go updateData(data, logger)
	go listenForInput(s, input, logger)
	go syncChannels(s, data, input, logger)

	time.Sleep(time.Minute)

}
