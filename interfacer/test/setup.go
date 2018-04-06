package test

import (
	"strings"
	"time"
	"strconv"
	"net/http"
	"unicode/utf8"

	"github.com/gdamore/tcell"
	"github.com/gdamore/tcell/terminfo"
	gingko "github.com/onsi/ginkgo"
	gomega "github.com/onsi/gomega"

	"browsh/interfacer/src/browsh"
)

var simScreen tcell.SimulationScreen
var startupWait = 10
var perTestTimeout = 2000 * time.Millisecond
var browserFingerprint = " ← | x | "
var rootDir = browsh.Shell("git rev-parse --show-toplevel")
var testSiteURL = "http://localhost:" + browsh.TestServerPort
var ti *terminfo.Terminfo

func initTerm() {
	// The tests check for true colour RGB values. The only downside to forcing true colour
	// in tests is that snapshots of frames with true colour ANSI codes are output to logs.
	// Some people may not have true colour terminals, for example like on Travis, so cat'ing
	// logs may appear corrupt.
	ti, _ = terminfo.LookupTerminfo("xterm-truecolor")
}

// GetFrame ... Returns the current Browsh frame's text
func GetFrame() string {
	var frame, log string
	var line = 0
	var styleDefault = ti.TParm(ti.SetFgBg, int(tcell.ColorWhite), int(tcell.ColorBlack))
	width, _ := simScreen.Size()
	cells, _, _ := simScreen.GetContents()
	for _, element := range cells {
		line++
		frame += string(element.Runes)
		log += elementColourForTTY(element) + string(element.Runes)
		if line == width {
			frame += "\n"
			log += styleDefault + "\n"
			line = 0
		}
	}
	log += styleDefault
	browsh.Log(log)
	return frame
}

// SpecialKey injects a special key into the TTY. See Tcell's `keys.go` file for all
// the available special keys.
func SpecialKey(key tcell.Key) {
	simScreen.InjectKey(key, 0, tcell.ModNone)
}

// Keyboard types a string of keys into the TTY, as if a user would
func Keyboard(keys string) {
	for _, char := range keys {
		simScreen.InjectKey(tcell.KeyRune, char, tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForNextFrame() {
	// Need to wait so long because the frame rate is currently so slow
	// TODO: Reduce the wait when the FPS is higher
	time.Sleep(500 * time.Millisecond)
}

// GotoURL sends the browsh browser to the specified URL
func GotoURL(url string) {
	SpecialKey(tcell.KeyCtrlL)
	Keyboard(url)
	SpecialKey(tcell.KeyEnter)
	// TODO: Waiting and looking for the URL aren't optimal.
	// Better to somehow create a unique identifier to poll for. Polling for the URL isn't
	// good enough because it could be the same URL as the previous test.
	time.Sleep(250 * time.Millisecond)
	gomega.Expect(url).To(BeInFrameAt(9, 1))
}

func elementColourForTTY(element tcell.SimCell) string {
	var fg, bg tcell.Color
	fg, bg, _ = element.Style.Decompose()
	r1, g1, b1 := fg.RGB()
	r2, g2, b2 := bg.RGB()
	return ti.TParm(ti.SetFgBgRGB,
			int(r1), int(g1), int(b1),
			int(r2), int(g2), int(b2))
}

// GetText retruns an individual piece of a frame
func GetText(x, y, length int) string {
	var text string
	frame := []rune(GetFrame())
	width, _ := simScreen.Size()
	index := ((width + 1) * y) + x
	for {
		text += string(frame[index])
		index++
		if runeCount(text) == length { break }
	}
	return text
}

// GetFgColour returns the foreground colour of a single cell
func GetFgColour(x, y int) [3]int32 {
	GetFrame()
	cells, _, _ := simScreen.GetContents()
	width, _ := simScreen.Size()
	index := (width * y) + x
	fg, _, _ := cells[index].Style.Decompose()
	r1, g1, b1 := fg.RGB()
	return [3]int32{r1, g1, b1}
}

// GetBgColour returns the background colour of a single cell
func GetBgColour(x, y int) [3]int32 {
	GetFrame()
	cells, _, _ := simScreen.GetContents()
	width, _ := simScreen.Size()
	index := (width * y) + x
	_, bg, _ := cells[index].Style.Decompose()
	r1, g1, b1 := bg.RGB()
	return [3]int32{r1, g1, b1}
}

func startHTTPServer() {
	// Use `NewServerMux()` so as not to conflict with browsh's websocket server
	serverMux := http.NewServeMux()
	serverMux.Handle("/", http.FileServer(http.Dir(rootDir + "/interfacer/test/sites")))
	http.ListenAndServe(":" + browsh.TestServerPort, serverMux)
}

func startBrowsh() {
	simScreen = tcell.NewSimulationScreen("UTF-8")
	browsh.Start(simScreen)
}

func waitForBrowsh() {
	var count = 0
	for {
		if count > startupWait {
			var message = "Couldn't find browsh " +
				"startup signature within " +
				strconv.Itoa(startupWait) +
				" seconds"
			panic(message)
		}
		time.Sleep(time.Second)
		if (strings.Contains(GetFrame(), browserFingerprint)) {
			break
		}
		count++
	}
}

func runeCount(text string) int {
	return utf8.RuneCountInString(text)
}

var _ = gingko.BeforeSuite(func() {
	initTerm()
	go startHTTPServer()
	go startBrowsh()
	waitForBrowsh()
})

var _	= gingko.AfterSuite(func() {
	browsh.Shell(rootDir + "/webext/contrib/firefoxheadless.sh kill")
})
