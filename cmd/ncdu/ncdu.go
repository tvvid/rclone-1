// Package ncdu implements a text based user interface for exploring a remote

//+build !plan9,!solaris

package ncdu

import (
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/ncdu/scan"
	"github.com/ncw/rclone/fs"
	termbox "github.com/nsf/termbox-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "ncdu remote:path",
	Short: `Explore a remote with a text based user interface.`,
	Long: `
This displays a text based user interface allowing the navigation of a
remote. It is most useful for answering the question - "What is using
all my disk space?".

<script src="https://asciinema.org/a/157793.js" id="asciicast-157793" async></script>

To make the user interface it first scans the entire remote given and
builds an in memory representation.  rclone ncdu can be used during
this scanning phase and you will see it building up the directory
structure as it goes along.

Here are the keys - press '?' to toggle the help on and off

    ` + strings.Join(helpText[1:], "\n    ") + `

This an homage to the [ncdu tool](https://dev.yorhel.nl/ncdu) but for
rclone remotes.  It is missing lots of features at the moment, most
importantly deleting files, but is useful as it stands.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return NewUI(fsrc).Show()
		})
	},
}

// help text
var helpText = []string{
	"rclone ncdu",
	" ↑,↓ or k,j to Move",
	" →,l to enter",
	" ←,h to return",
	" c toggle counts",
	" g toggle graph",
	" n,s,C sort by name,size,count",
	" ^L refresh screen",
	" ? to toggle help on and off",
	" q/ESC/c-C to quit",
}

// UI contains the state of the user interface
type UI struct {
	f             fs.Fs         // fs being displayed
	fsName        string        // human name of Fs
	root          *scan.Dir     // root directory
	d             *scan.Dir     // current directory being displayed
	path          string        // path of current directory
	showBox       bool          // whether to show a box
	boxText       []string      // text to show in box
	entries       fs.DirEntries // entries of current directory
	sortPerm      []int         // order to display entries in after sorting
	invSortPerm   []int         // inverse order
	dirListHeight int           // height of listing
	listing       bool          // whether listing is in progress
	showGraph     bool          // toggle showing graph
	showCounts    bool          // toggle showing counts
	sortByName    int8          // +1 for normal, 0 for off, -1 for reverse
	sortBySize    int8
	sortByCount   int8
	dirPosMap     map[string]dirPos // store for directory positions
}

// Where we have got to in the directory listing
type dirPos struct {
	entry  int
	offset int
}

// Print a string
func Print(x, y int, fg, bg termbox.Attribute, msg string) {
	for _, c := range msg {
		termbox.SetCell(x, y, c, fg, bg)
		x++
	}
}

// Printf a string
func Printf(x, y int, fg, bg termbox.Attribute, format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	Print(x, y, fg, bg, s)
}

// Line prints a string to given xmax, with given space
func Line(x, y, xmax int, fg, bg termbox.Attribute, spacer rune, msg string) {
	for _, c := range msg {
		termbox.SetCell(x, y, c, fg, bg)
		x++
		if x >= xmax {
			return
		}
	}
	for ; x < xmax; x++ {
		termbox.SetCell(x, y, spacer, fg, bg)
	}
}

// Linef a string
func Linef(x, y, xmax int, fg, bg termbox.Attribute, spacer rune, format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	Line(x, y, xmax, fg, bg, spacer, s)
}

// Box the u.boxText onto the screen
func (u *UI) Box() {
	w, h := termbox.Size()

	// Find dimensions of text
	boxWidth := 10
	for _, s := range u.boxText {
		if len(s) > boxWidth && len(s) < w-4 {
			boxWidth = len(s)
		}
	}
	boxHeight := len(u.boxText)

	// position
	x := (w - boxWidth) / 2
	y := (h - boxHeight) / 2
	xmax := x + boxWidth

	// draw text
	fg, bg := termbox.ColorRed, termbox.ColorWhite
	for i, s := range u.boxText {
		Line(x, y+i, xmax, fg, bg, ' ', s)
		fg = termbox.ColorBlack
	}

	// FIXME draw a box around
}

// find the biggest entry in the current listing
func (u *UI) biggestEntry() (biggest int64) {
	if u.d == nil {
		return
	}
	for i := range u.entries {
		size, _, _, _ := u.d.AttrI(u.sortPerm[i])
		if size > biggest {
			biggest = size
		}
	}
	return
}

// Draw the current screen
func (u *UI) Draw() error {
	w, h := termbox.Size()
	u.dirListHeight = h - 3

	// Plot
	err := termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	if err != nil {
		return errors.Wrap(err, "failed to clear screen")
	}

	// Header line
	Linef(0, 0, w, termbox.ColorBlack, termbox.ColorWhite, ' ', "rclone ncdu %s - use the arrow keys to navigate, press ? for help", fs.Version)

	// Directory line
	Linef(0, 1, w, termbox.ColorWhite, termbox.ColorBlack, '-', "-- %s ", u.path)

	// graphs
	const (
		graphBars = 10
		graph     = "##########          "
	)

	// Directory listing
	if u.d != nil {
		y := 2
		perBar := u.biggestEntry() / graphBars
		if perBar == 0 {
			perBar = 1
		}
		dirPos := u.dirPosMap[u.path]
		for i, j := range u.sortPerm[dirPos.offset:] {
			entry := u.entries[j]
			n := i + dirPos.offset
			if y >= h-1 {
				break
			}
			fg := termbox.ColorWhite
			bg := termbox.ColorBlack
			if n == dirPos.entry {
				fg, bg = bg, fg
			}
			size, count, isDir, readable := u.d.AttrI(u.sortPerm[n])
			mark := ' '
			if isDir {
				mark = '/'
			}
			message := ""
			if !readable {
				message = " [not read yet]"
			}
			extras := ""
			if u.showCounts {
				if count > 0 {
					extras += fmt.Sprintf("%8v ", fs.SizeSuffix(count))
				} else {
					extras += "         "
				}

			}
			if u.showGraph {
				bars := (size + perBar/2 - 1) / perBar
				// clip if necessary - only happens during startup
				if bars > 10 {
					bars = 10
				} else if bars < 0 {
					bars = 0
				}
				extras += "[" + graph[graphBars-bars:2*graphBars-bars] + "] "
			}
			Linef(0, y, w, fg, bg, ' ', "%8v %s%c%s%s", fs.SizeSuffix(size), extras, mark, path.Base(entry.Remote()), message)
			y++
		}
	}

	// Footer
	if u.d == nil {
		Line(0, h-1, w, termbox.ColorBlack, termbox.ColorWhite, ' ', "Waiting for root directory...")
	} else {
		message := ""
		if u.listing {
			message = " [listing in progress]"
		}
		size, count := u.d.Attr()
		Linef(0, h-1, w, termbox.ColorBlack, termbox.ColorWhite, ' ', "Total usage: %v, Objects: %d%s", fs.SizeSuffix(size), count, message)
	}

	// Show the box on top if requred
	if u.showBox {
		u.Box()
	}
	err = termbox.Flush()
	if err != nil {
		return errors.Wrap(err, "failed to flush screen")
	}
	return nil
}

// Move the cursor this many spaces adjusting the viewport as necessary
func (u *UI) move(d int) {
	if u.d == nil {
		return
	}

	absD := d
	if d < 0 {
		absD = -d
	}

	entries := len(u.entries)

	// Fetch current dirPos
	dirPos := u.dirPosMap[u.path]

	dirPos.entry += d

	// check entry in range
	if dirPos.entry < 0 {
		dirPos.entry = 0
	} else if dirPos.entry >= entries {
		dirPos.entry = entries - 1
	}

	// check cursor still on screen
	p := dirPos.entry - dirPos.offset // where dirPos.entry appears on the screen
	if p < 0 {
		dirPos.offset -= absD
	} else if p >= u.dirListHeight {
		dirPos.offset += absD
	}

	// check dirPos.offset in bounds
	if entries == 0 || dirPos.offset < 0 {
		dirPos.offset = 0
	} else if dirPos.offset >= entries {
		dirPos.offset = entries - 1
	}

	// write dirPos back for later
	u.dirPosMap[u.path] = dirPos
}

// Sort by the configured sort method
type ncduSort struct {
	sortPerm []int
	entries  fs.DirEntries
	d        *scan.Dir
	u        *UI
}

// Less is part of sort.Interface.
func (ds *ncduSort) Less(i, j int) bool {
	isize, icount, _, _ := ds.d.AttrI(ds.sortPerm[i])
	jsize, jcount, _, _ := ds.d.AttrI(ds.sortPerm[j])
	iname, jname := ds.entries[ds.sortPerm[i]].Remote(), ds.entries[ds.sortPerm[j]].Remote()
	switch {
	case ds.u.sortByName < 0:
		return iname > jname
	case ds.u.sortByName > 0:
		break
	case ds.u.sortBySize < 0:
		if isize != jsize {
			return isize < jsize
		}
	case ds.u.sortBySize > 0:
		if isize != jsize {
			return isize > jsize
		}
	case ds.u.sortByCount < 0:
		if icount != jcount {
			return icount < jcount
		}
	case ds.u.sortByCount > 0:
		if icount != jcount {
			return icount > jcount
		}
	}
	// if everything equal, sort by name
	return iname < jname
}

// Swap is part of sort.Interface.
func (ds *ncduSort) Swap(i, j int) {
	ds.sortPerm[i], ds.sortPerm[j] = ds.sortPerm[j], ds.sortPerm[i]
}

// Len is part of sort.Interface.
func (ds *ncduSort) Len() int {
	return len(ds.sortPerm)
}

// sort the permutation map of the current directory
func (u *UI) sortCurrentDir() {
	u.sortPerm = u.sortPerm[:0]
	for i := range u.entries {
		u.sortPerm = append(u.sortPerm, i)
	}
	data := ncduSort{
		sortPerm: u.sortPerm,
		entries:  u.entries,
		d:        u.d,
		u:        u,
	}
	sort.Sort(&data)
	if len(u.invSortPerm) < len(u.sortPerm) {
		u.invSortPerm = make([]int, len(u.sortPerm))
	}
	for i, j := range u.sortPerm {
		u.invSortPerm[j] = i
	}
}

// setCurrentDir sets the current directory
func (u *UI) setCurrentDir(d *scan.Dir) {
	u.d = d
	u.entries = d.Entries()
	u.path = path.Join(u.fsName, d.Path())
	u.sortCurrentDir()
}

// enters the current entry
func (u *UI) enter() {
	if u.d == nil || len(u.entries) == 0 {
		return
	}
	dirPos := u.dirPosMap[u.path]
	d, _ := u.d.GetDir(u.sortPerm[dirPos.entry])
	if d == nil {
		return
	}
	u.setCurrentDir(d)
}

// up goes up to the parent directory
func (u *UI) up() {
	if u.d == nil {
		return
	}
	parent := u.d.Parent()
	if parent != nil {
		u.setCurrentDir(parent)
	}
}

// popupBox shows a box with the text in
func (u *UI) popupBox(text []string) {
	u.boxText = text
	u.showBox = true
}

// togglePopupBox shows a box with the text in
func (u *UI) togglePopupBox(text []string) {
	if u.showBox {
		u.showBox = false
	} else {
		u.popupBox(text)
	}
}

// toggle the sorting for the flag passed in
func (u *UI) toggleSort(sortType *int8) {
	old := *sortType
	u.sortBySize = 0
	u.sortByCount = 0
	u.sortByName = 0
	if old == 0 {
		*sortType = 1
	} else {
		*sortType = -old
	}
	u.sortCurrentDir()
}

// NewUI creates a new user interface for ncdu on f
func NewUI(f fs.Fs) *UI {
	return &UI{
		f:             f,
		path:          "Waiting for root...",
		dirListHeight: 20, // updated in Draw
		fsName:        f.Name() + ":" + f.Root(),
		showGraph:     true,
		showCounts:    false,
		sortByName:    0, // +1 for normal, 0 for off, -1 for reverse
		sortBySize:    1,
		sortByCount:   0,
		dirPosMap:     make(map[string]dirPos),
	}
}

// Show shows the user interface
func (u *UI) Show() error {
	err := termbox.Init()
	if err != nil {
		log.Fatal(err)
	}
	defer termbox.Close()

	// scan the disk in the background
	u.listing = true
	rootChan, errChan, updated := scan.Scan(u.f)

	// Poll the events into a channel
	events := make(chan termbox.Event)
	doneWithEvent := make(chan bool)
	go func() {
		for {
			events <- termbox.PollEvent()
			<-doneWithEvent
		}
	}()

	// Main loop, waiting for events and channels
outer:
	for {
		//Reset()
		err := u.Draw()
		if err != nil {
			return errors.Wrap(err, "draw failed")
		}
		var root *scan.Dir
		select {
		case root = <-rootChan:
			u.root = root
			u.setCurrentDir(root)
		case err := <-errChan:
			if err != nil {
				return errors.Wrap(err, "ncdu directory listing")
			}
			u.listing = false
		case <-updated:
			// redraw
			// might want to limit updates per second
			u.sortCurrentDir()
		case ev := <-events:
			doneWithEvent <- true
			if ev.Type == termbox.EventKey {
				switch ev.Key + termbox.Key(ev.Ch) {
				case termbox.KeyEsc, termbox.KeyCtrlC, 'q':
					if u.showBox {
						u.showBox = false
					} else {
						break outer
					}
				case termbox.KeyArrowDown, 'j':
					u.move(1)
				case termbox.KeyArrowUp, 'k':
					u.move(-1)
				case termbox.KeyPgdn, '-', '_':
					u.move(u.dirListHeight)
				case termbox.KeyPgup, '=', '+':
					u.move(-u.dirListHeight)
				case termbox.KeyArrowLeft, 'h':
					u.up()
				case termbox.KeyArrowRight, 'l', termbox.KeyEnter:
					u.enter()
				case 'c':
					u.showCounts = !u.showCounts
				case 'g':
					u.showGraph = !u.showGraph
				case 'n':
					u.toggleSort(&u.sortByName)
				case 's':
					u.toggleSort(&u.sortBySize)
				case 'C':
					u.toggleSort(&u.sortByCount)
				case '?':
					u.togglePopupBox(helpText)

				// Refresh the screen. Not obvious what key to map
				// this onto, but ^L is a common choice.
				case termbox.KeyCtrlL:
					err := termbox.Sync()
					if err != nil {
						fs.Errorf(nil, "termbox sync returned error: %v", err)
					}
				}
			}
		}
		// listen to key presses, etc
	}
	return nil
}
