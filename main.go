package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
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
	id       string
	name     string
	user     string
	cpu      string
	mem      string
	children []string
}

type userIDs struct {
	mapping map[string]string
}

/*func userSplitter(chr rune) bool {
	return chr == '(' || chr == ')'
}*/

func getUserFromID(id string) string {
	file, err := os.ReadFile("/proc/" + id + "/status")
	if err != nil {
		panic("reading status file " + id)
	}

	data := strings.Fields(string(file))

	cmd := exec.Command("id", "-nu", data[19])
	output, err := cmd.Output()
	if err != nil {
		panic("running \"id\" err: " + id)
	}
	//name := strings.FieldsFunc(string(output), userSplitter)
	return string(output)
}

func genUserIDs() *userIDs {

	mapping := make(map[string]string)

	mapping["1"] = getUserFromID("1")

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

func genMemInfo(l *logger) *memInfo {

	file, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		l.info("not gambling here")
		panic("ahah")
	}

	badData := string(file)
	info := strings.Fields(badData)

	total, err := strconv.Atoi(info[1])
	if err != nil {
		//one of these days ill get the logger and catch these errors properly
		fmt.Println("Error:", err)
		panic("dont know tbh")
	}

	available, err := strconv.Atoi(info[4])
	if err != nil {
		//one of these days ill get the logger and catch these errors properly
		fmt.Println("Error:", err)
		panic("the same as the one before me")
	}

	l.info("total: " + strconv.Itoa(total))

	return &memInfo{total, available}
}

func getProcessData(id string, flags *map[string]bool, users *userIDs, mem *memInfo, l *logger) proc {

	var data proc

	data.id = id

	if (*flags)["name"] {
		name, err := os.ReadFile("/proc/" + id + "/comm")
		if err != nil {
			fmt.Println("Error reading file: ", err)
		}

		data.name = string(name)
	}

	if (*flags)["user"] {
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

	if (*flags)["mem"] {
		file, err := os.ReadFile("/proc/" + id + "/statm")
		if err != nil {
			log.Fatalf("could not read file with da meminfo")
			panic("error reading file") //pretty sure this is what im supposed to do but ig we will find out :)
		}

		badMemory := string(file)
		memory := strings.Split(badMemory, " ")
		usage, _ := strconv.Atoi(memory[1]) //yeah ik ill fix it later

		data.mem = strconv.FormatFloat(float64(usage)/float64(mem.totalMem)*100, 'f', -1, 64)
		//l.info("memory: " + data.mem)
	}

	if (*flags)["children"] {
		badChildren, err := os.ReadFile("/proc/" + id + "/task/" + id + "/children")
		if err != nil {
			fmt.Println("Error reading file: ", err)
		}

		children := strings.Split(strings.Trim(string(badChildren), " "), " ")

		data.children = children
	}

	//l.info("made it here: " + id)
	return data
}

func getRootIds(flags *map[string]bool, users *userIDs, mem *memInfo, l *logger) []proc {
	children := getProcessData("1", &map[string]bool{"children": true}, users, mem, l).children

	processes := []proc{}

	//l.info("not entirely sure what i changed: " + strconv.Itoa(len(children)))
	for _, child := range children {
		processes = append(processes, getProcessData(child, flags, users, mem, l))
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

func compareStrings(a, b string) int {
	if a > b {
		return 1
	} else if a < b {
		return -1
	} else {
		return 0
	}
}

func sortProcesses(processes *[]proc, sort string) {
	if sort == "id" {
		slices.SortStableFunc(*processes, func(a, b proc) int {
			first, _ := strconv.Atoi(a.id)
			second, _ := strconv.Atoi(b.id)
			if first > second {
				return 1
			} else if first < second {
				return -1
			} else {
				return 0
			}
		})
	} else if sort == "user" {
		slices.SortStableFunc(*processes, func(a, b proc) int {
			return compareStrings(a.user, b.user)
		})
	} else if sort == "mem" {
		slices.SortStableFunc(*processes, func(a, b proc) int {
			return compareStrings(b.mem, a.mem)
		})
	} else if sort == "name" {
		slices.SortStableFunc(*processes, func(a, b proc) int {
			return compareStrings(a.name, b.name)
		})
	}
}

func refreshScreen(s tcell.Screen, processes []proc, cursor int, sort string) {

	defStyle := tcell.StyleDefault.Background(tcell.Color16).Foreground(tcell.Color100)
	highLightStyle := tcell.StyleDefault.Background(tcell.Color160).Foreground(tcell.Color100)

	s.Clear()

	sortProcesses(&processes, sort)

	i := 0
	for _, value := range processes {

		message := value.id + ": " + value.user + ", " + value.mem + ", " + value.name
		drawText(s, 0, i+2, defStyle, message)

		if i == cursor {
			drawText(s, 0, i+2, highLightStyle, message)
		}

		i++
	}
	s.Show()

}

func syncChannels(s tcell.Screen, flags *map[string]bool, data chan []proc, cursor chan int, sort chan int, l *logger) {
	cursorRow := 0
	sortCollumn := 0

	sortMap := []string{}
	for key, value := range *flags {
		if value {
			sortMap = append(sortMap, key)
		}
	}

	//temporary to advoid sorting by process name

	var processes []proc

	for {
		select {
		case processes = <-data:
			l.info("recieved the data: " + strconv.Itoa(len(processes)))
			refreshScreen(s, processes, cursorRow, sortMap[sortCollumn])

		case cursorChange := <-cursor:
			cursorRow = (cursorRow + cursorChange + len(processes)) % len(processes)
			refreshScreen(s, processes, cursorRow, sortMap[sortCollumn])

		case sortChange := <-sort:
			sortCollumn = (sortCollumn + sortChange + len(sortMap)) % len(sortMap)
			refreshScreen(s, processes, cursorRow, sortMap[sortCollumn])
		}
	}
}

func listenForInput(s tcell.Screen, cursor chan int, sort chan int, l *logger) {

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
					cursor <- 1

				case "k":
					cursor <- -1

				case "h":
					sort <- -1

				case "l":
					sort <- 1
				}

			case tcell.KeyEscape, tcell.KeyCtrlC:
				quit()
			}
		}
	}
}

func updateData(s tcell.Screen, data chan []proc, flags *map[string]bool, users *userIDs, mem *memInfo, l *logger) {

	defer func() {
		maybePanic := recover()
		s.Fini()
		fmt.Println("i quitted")
		if maybePanic != nil {
			panic(maybePanic)
		}
	}()

	for {

		data <- getRootIds(flags, users, mem, l)
		l.info("sent new data")
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
		fmt.Println("i quit")
		logger.info("i quit")
		maybePanic := recover()
		s.Fini()
		if maybePanic != nil {
			panic(maybePanic)
		}
	}
	defer quit()

	flags := map[string]bool{"id": true, "user": true, "cpu": false, "mem": true, "name": true}
	users := genUserIDs()
	mem := genMemInfo(logger)

	data := make(chan []proc)
	cursor := make(chan int)
	sort := make(chan int)

	go updateData(s, data, &flags, users, mem, logger)
	go listenForInput(s, cursor, sort, logger)
	go syncChannels(s, &flags, data, cursor, sort, logger)

	time.Sleep(time.Minute)

}
