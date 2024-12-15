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
	children []proc
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
		panic("reading status file: " + id)
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

		data.mem = strconv.FormatFloat(float64(int(float64(usage)/float64(mem.totalMem)*10000))/100, 'f', -1, 64)
	}

	if true {
		badChildren, err := os.ReadFile("/proc/" + id + "/task/" + id + "/children")
		if err != nil {
			fmt.Println("Error reading file: ", err)
		}

		childrenIDs := strings.Fields(string(badChildren))

		children := []proc{}

		for _, child := range childrenIDs {
			children = append(children, getProcessData(child, flags, users, mem, l))
		}

		data.children = children
	}

	return data
}

func drawText(s tcell.Screen, x1, y1 int, style tcell.Style, text string) {
	row := y1
	col := x1
	for _, r := range []rune(text) {
		s.SetContent(col, row, r, nil, style)
		col++
	}
}

func getLength(processes *[]proc, show *map[string]bool) int {
	length := len(*processes)
	for _, value := range *processes {
		if !(*show)[value.id] {
			continue
		}
		length += getLength(&value.children, show)
	}

	return length
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

const idWidth = 5
const userWidth = 10
const memoryWidth = 10

func recursChildren(s tcell.Screen, processes *[]proc, sort string, show *map[string]bool, cursor, pad int, row *int, bars []bool, defStyle, highlightStyle *tcell.Style, l *logger) {

	var style tcell.Style

	sortProcesses(processes, sort)

	for index, value := range *processes {

		//message := value.id + " " + value.user + " " + value.mem + padding + " " + coolCharacter + "─" + value.name

		if *row == cursor {
			style = *highlightStyle
		} else {
			style = *defStyle
		}

		//display the data
		collum := 0

		if value.id != "" {
			drawText(s, collum, *row+2, style, value.id)
			collum += idWidth
		}

		if value.user != "" {
			drawText(s, collum, *row+2, style, value.user)
			collum += userWidth
		}

		if value.mem != "" {
			drawText(s, collum, *row+2, style, value.mem)
			collum += memoryWidth
		}

		padding := ""
		for _, bar := range bars {
			if bar {
				padding += "│  "
			} else {
				padding += "   "
			}
		}

		drawText(s, collum, *row+2, style, padding)
		collum += 3 * pad

		//some fun formatting things
		coolCharacter := "├"
		if index == len(*processes)-1 {
			coolCharacter = "└"
		}
		expanded := "+"
		if (*show)[value.id] {
			expanded = "─"
		}
		//yeah ik its only temporary
		if value.id != "1" {
			drawText(s, collum, *row+2, style, coolCharacter)
			drawText(s, collum+1, *row+2, style, expanded)
		}

		if value.name != "" {
			drawText(s, collum+3, *row+2, style, value.name)
		}
		*row += 1

		if len(value.children) != 0 && (*show)[value.id] { // 																	what is this python?
			recursChildren(s, &value.children, sort, show, cursor, pad+1, row, append(bars, index != len(*processes)-1), defStyle, highlightStyle, l)
		}

	}
}

func refreshScreen(s tcell.Screen, processes []proc, cursor int, sort string, show *map[string]bool, l *logger) {

	defStyle := tcell.StyleDefault.Background(tcell.Color16).Foreground(tcell.Color100)
	highLightStyle := tcell.StyleDefault.Background(tcell.Color160).Foreground(tcell.Color100)

	s.Clear()

	row := 0
	//l.info("100 percent we give: " + strconv.Itoa(cursor))
	recursChildren(s, &processes, sort, show, cursor, 0, &row, []bool{}, &defStyle, &highLightStyle, l)
	s.Show()

}

func getIDFromRow(processes *[]proc, row, i *int, show *map[string]bool, l *logger) string {

	for _, proc := range *processes {
		if *row == *i {
			return proc.id
		}

		*i++

		if (*show)[proc.id] && len(proc.children) != 0 {
			id := getIDFromRow(&proc.children, row, i, show, l)
			if id != "" {
				return id
			}
		}
	}

	return ""
}

func syncChannels(s tcell.Screen, flags *map[string]bool, show *map[string]bool, data chan []proc, cursor, sort, toggle chan int, l *logger) {

	defer func() {
		maybePanic := recover()
		s.Fini()
		fmt.Println("i quitted")
		if maybePanic != nil {
			panic(maybePanic)
		}
	}()

	length := 1
	cursorRow := 0
	sortCollumn := 0

	sortMap := []string{}
	for key, value := range *flags {
		if value {
			sortMap = append(sortMap, key)
		}
	}

	var processes []proc

	for {
		select {
		case processes = <-data:
			length = getLength(&processes, show)
			refreshScreen(s, processes, cursorRow, sortMap[sortCollumn], show, l)

		case cursorChange := <-cursor:
			cursorRow = (cursorRow + cursorChange + length) % length
			refreshScreen(s, processes, cursorRow, sortMap[sortCollumn], show, l)

		case sortChange := <-sort:
			sortCollumn = (sortCollumn + sortChange + len(sortMap)) % len(sortMap)
			refreshScreen(s, processes, cursorRow, sortMap[sortCollumn], show, l)

		case toggle := <-toggle:
			i := 0
			id := getIDFromRow(&processes, &cursorRow, &i, show, l)
			if toggle == 1 {
				(*show)[id] = !(*show)[id]
			}
			l.info("bruH: " + strconv.FormatBool((*show)[processes[0].id]))
			refreshScreen(s, processes, cursorRow, sortMap[sortCollumn], show, l)
		}

	}
}

func listenForInput(s tcell.Screen, cursor, sort, toggle chan int, l *logger) {

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

				case "q":
					quit()
				}

			case tcell.KeyEnter:
				l.info("detected keypress")
				toggle <- 1

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

		data <- []proc{getProcessData("1", flags, users, mem, l)}
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
	show := map[string]bool{"1": true}
	users := genUserIDs()
	mem := genMemInfo(logger)

	data := make(chan []proc)
	cursor := make(chan int)
	sort := make(chan int)
	toggle := make(chan int)

	go updateData(s, data, &flags, users, mem, logger)
	go listenForInput(s, cursor, sort, toggle, logger)
	go syncChannels(s, &flags, &show, data, cursor, sort, toggle, logger)

	time.Sleep(time.Minute)

}
