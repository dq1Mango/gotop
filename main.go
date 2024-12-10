package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
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
	user     string
	cpu      string
	mem      string
	children []string
}

type params struct {
	name     bool
	user     bool
	cpu      bool
	mem      bool
	children bool
}

type userIDs struct {
	mapping map[string]string
}

func userSplitter(chr rune) bool {
	return chr == '\n' || chr == ')'
}

func getUserFromID(id string) string {
	cmd := exec.Command("id", id)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
		panic("running \"id\"")
	}
	name := strings.FieldsFunc(string(output), userSplitter)
	return name[1]
}

func genUserIDs() *userIDs {
	mapping := make(map[string]string)

	mapping["1"] = getUserFromID("1")
	mapping["1000"] = getUserFromID("1000")

	return &userIDs{mapping}
}

func (user *userIDs) addUser(id, name *string) {
	//should add a check here but i dont write bugs so im sure its fine
	user.mapping[*id] = *name
}

type memInfo struct {
	totalMem     int
	availableMem int
}

func genMemInfo() *memInfo {

	file, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		fmt.Println("could not read meminfo and i really should be using the logger function")
	}

	badData := string(file)
	badData = strings.Replace(badData, " ", "", 0)
	badData = strings.Replace(badData, "kB", "", 0)
	info := strings.Fields(badData)

	total, err := strconv.Atoi(info[1])
	if err != nil {
		//one of these days ill get the logger and catch these errors properly
		fmt.Println("Error:", err)
	}

	available, err := strconv.Atoi(info[3])
	if err != nil {
		//one of these days ill get the logger and catch these errors properly
		fmt.Println("Error:", err)
	}

	return &memInfo{total, available}
}

func getProcessData(id string, flags *params, users *userIDs, mem *memInfo) proc {
	var data proc

	if flags.name {
		name, err := os.ReadFile("/proc/" + id + "/comm")
		if err != nil {
			fmt.Println("Error reading file: ", err)
		}

		data.name = string(name)
	}

	if flags.user {
		badUser, err := os.ReadFile("/proc/" + id + "/comm")
		if err != nil {
			fmt.Println("error reading name file")
		}
		user := string(badUser)
		name, ok := users.mapping[user]
		if ok {
			data.user = name
		} else {
			name = getUserFromID(id)
			data.user = name
			users.addUser(&user, &name)
		}
	}

	if flags.mem {
		file, err := os.ReadFile("/proc/" + id + "/statm")
		if err != nil {
			log.Fatalf("could not read file with da meminfo")
			panic("error reading file") //pretty sure this is what im supposed to do but ig we will find out :)
		}

		badMemory := string(file)
		memory := strings.Split(badMemory, " ")
		usage, _ := strconv.Atoi(memory[1]) //yeah ik ill fix it later

		data.mem = string(usage / mem.totalMem)
	}

	if flags.children {
		badChildren, err := os.ReadFile("/proc/" + id + "/task/" + id + "/children")
		if err != nil {
			fmt.Println("Error reading file: ", err)
		}

		children := strings.Split(strings.Trim(string(badChildren), " "), " ")

		data.children = children
	}

	return data
}

func getRootIds(flags *params, users *userIDs, mem *memInfo) map[string]proc {
	children := getProcessData("1", flags, users, mem).children

	var processes = make(map[string]proc)

	for _, child := range children {
		processes[child] = getProcessData(child, flags, users, mem)
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
			if maybeCursor < 0 || maybeCursor >= len(processes) {
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

func updateData(data chan map[string]proc, flags *params, users *userIDs, mem *memInfo, l *logger) {

	for {

		data <- getRootIds(flags, users, mem)

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

	flags := params{true, true, false, true, true}
	users := genUserIDs()
	mem := genMemInfo()

	data := make(chan map[string]proc)
	input := make(chan int)

	go updateData(data, &flags, users, mem, logger)
	go listenForInput(s, input, logger)
	go syncChannels(s, data, input, logger)

	time.Sleep(time.Minute)

}
