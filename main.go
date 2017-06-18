package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"gopkg.in/yaml.v2"
)

type MyMainWindow struct {
	*walk.MainWindow
	settings Setting

	nameEdit *walk.LineEdit
	regEdit  *walk.LineEdit
	nameBox  *walk.ComboBox
	ignBox   *walk.CheckBox

	logTE *walk.TextEdit
}

type Item struct {
	Name   string `yaml:"Name"`
	RegExp string `yaml:"RegExp"`
}
type Setting struct {
	Items []*Item `yaml:"Items"`
}

type CommentItem struct {
	Comment   string
	Timestamp time.Time
}
type CommentItems []CommentItem

func (c CommentItems) Len() int {
	return len(c)
}

func (c CommentItems) Less(i, j int) bool {
	return c[i].Timestamp.Before(c[j].Timestamp)
}

func (c CommentItems) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

var logch = make(chan string, 10)

const (
	LETTERS        = "0123456789"
	COMMENT_FORMAT = "<chat user_id=\"a\" date=\"1\" no=\"%d\" vpos=\"%s\">%s</chat>\r\n"
)

func (mw *MyMainWindow) getSettings() []*Item {
	buf, err := ioutil.ReadFile("config.yml")
	if err != nil {
		return nil
	}

	var s Setting
	if err := yaml.Unmarshal(buf, &s); err != nil {
		return nil
	}
	mw.settings = s
	return s.Items
}

func (mw *MyMainWindow) setSettings() {
	for i, v := range mw.settings.Items {
		if v.Name == mw.nameBox.Text() {
			mw.regEdit.SetText(mw.settings.Items[i].RegExp)
			return
		}
	}
}

func main() {
	mw := new(MyMainWindow)

	rand.Seed(time.Now().UnixNano())
	go func() {
		for {
			msg := <-logch
			mw.logTE.AppendText(msg + "\n")
			time.Sleep(100 * time.Millisecond)
		}
	}()

	if _, err := (MainWindow{
		AssignTo: &mw.MainWindow,
		Title:    "Re-Nicome",
		MinSize:  Size{600, 400},
		Layout:   VBox{},
		OnDropFiles: func(files []string) {
			mw.replaceFiles(files)
		},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{
						Text: "Find what:",
					},
					LineEdit{
						AssignTo: &mw.regEdit,
					},

					Label{
						Text: "Settings:",
					},
					ComboBox{
						DisplayMember: "Name",
						Model:         mw.getSettings(),
						OnCurrentIndexChanged: mw.setSettings,
						AssignTo:              &mw.nameBox,
					},
					Label{
						Text: "Ignore unparseable lines:",
					},
					CheckBox{
						AssignTo: &mw.ignBox,
					},
				},
			},

			Label{
				Text: "Drag and Drop comment files on this window.",
			},
			TextEdit{
				AssignTo: &mw.logTE,
				ReadOnly: true,
			},
			PushButton{
				Text:      "Save Settings",
				OnClicked: func() { mw.saveSettingsDialog(mw.MainWindow) },
			},
		},
	}.Run()); err != nil {
		log.Fatal(err)
	}
}

func (mw *MyMainWindow) replaceFiles(files []string) {
	_, err := regexp.Compile(mw.regEdit.Text())
	if err != nil {
		mw.errorDialog(nil, "Unable to compile value. Check expression.")
		return
	}

	for _, v := range files {
		go mw.replaceFile(v)
		time.Sleep(100 * time.Millisecond)
	}
}

// regexp: ^__TIME[UNIXTIME]__\\t__COMMENT__$
// regexp: ^__TIME[15:04:05]__ \(.+?\) __COMMENT__$
// regexp: ^[__TIME[2006/01/02 15:04:05]__] __COMMENT__（.*$
func (mw *MyMainWindow) replaceFile(path string) {
	parsedCom, err := mw.getParsedComments(path)
	if err != nil {
		logch <- err.Error()
	}
	sort.Sort(parsedCom)
	mw.saveCommentItems(path, parsedCom)
}

func (mw *MyMainWindow) getParsedComments(path string) (CommentItems, error) {
	s := mw.regEdit.Text()
	ret, _ := regexp.Compile(`__TIME\[(.+?)\]__`)
	rec, _ := regexp.Compile(`__COMMENT__`)

	isTimeFst := false
	if strings.Index(s, `__COMMENT__`) > strings.Index(s, `__TIME`) {
		isTimeFst = true
	}

	timeFmt := ret.FindStringSubmatch(s)
	if timeFmt == nil || len(timeFmt) != 2 {
		return nil, fmt.Errorf("Error[1]: Unable to parse file: " + path)
	}
	timeFmt[1] = strings.Replace(timeFmt[1], `\`, "", -1)

	s = ret.ReplaceAllString(s, "(.+?)")
	s = rec.ReplaceAllString(s, "(.+?)")
	// s == "(.+?)\t(.+?)"

	re, err := regexp.Compile(s)
	if err != nil {
		return nil, fmt.Errorf("Error[2]: Unable to parse file: " + path)
	}

	lines, err := readCommentFile(path)
	if err != nil {
		return nil, err
	}

	ignore := mw.ignBox.Checked()
	comments := make(CommentItems, 0)
	for _, comment := range lines {
		if comment == "" {
			continue
		}

		tc := re.FindStringSubmatch(comment)
		if len(tc) != 3 {
			if ignore {
				continue
			} else {
				return nil, fmt.Errorf("Error[3]: Unable to parse file: " + path)
			}
		}

		var timestr, comstr string
		if isTimeFst {
			timestr = tc[1]
			comstr = tc[2]
		} else {
			timestr = tc[2]
			comstr = tc[1]
		}

		var t time.Time
		if strings.ToUpper(timeFmt[1]) == "UNIXTIME" {
			tint, _ := strconv.ParseInt(timestr, 10, 64)
			t = time.Unix(tint, int64(rand.Intn(60)))
		} else {
			loc, _ := time.LoadLocation("Asia/Tokyo")
			t, err = time.ParseInLocation(timeFmt[1], timestr, loc)
			if err != nil {
				return nil, fmt.Errorf("Error[4]: Unable to parse file: " + path)
			}
		}
		comments = append(comments, CommentItem{comstr, t})
	}
	return comments, nil
}

func (mw *MyMainWindow) saveCommentItems(path string, comments CommentItems) {
	s := mw.regEdit.Text()
	ret, _ := regexp.Compile(`__TIME\[(.+?)\]__`)

	timeFmt := ret.FindStringSubmatch(s)
	if timeFmt == nil || len(timeFmt) != 2 {
		return
	}

	// open save file
	fp, err := os.OpenFile(path+".txt", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		logch <- err.Error()
		return
	}
	defer fp.Close()
	writer := bufio.NewWriter(fp)

	i := 1
	var oldtime int64 = math.MaxInt64
	for _, comment := range comments {
		if oldtime == math.MaxInt64 {
			oldtime = comment.Timestamp.Unix()
		}
		vpos := strconv.FormatInt(comment.Timestamp.Unix()-oldtime, 10)
		if vpos != "0" {
			for i := 0; i < 2; i++ {
				vpos = vpos + string(LETTERS[int(rand.Int63()%int64(len(LETTERS)))])
			}
		}
		outcom := fmt.Sprintf(COMMENT_FORMAT, i, vpos, comment.Comment)
		i++
		_, err = writer.WriteString(outcom)
		if err != nil {
			logch <- err.Error()
			return
		}
		writer.Flush()
	}
	logch <- "Complete: " + path
}

func readCommentFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("File %s could not read: %v\n", path, err)
	}
	defer f.Close()

	lines := make([]string, 1, 30000)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if serr := scanner.Err(); serr != nil {
		return nil, fmt.Errorf("File %s scan error: %v\n", path, err)
	}

	return lines, nil
}

func (mw *MyMainWindow) saveSettingsDialog(owner walk.Form) (int, error) {
	if mw.regEdit.Text() == "" {
		mw.errorDialog(owner, "Unable to use blank values.")
		return 1, errors.New("Unable to use blank values.")
	} else if _, err := regexp.Compile(mw.regEdit.Text()); err != nil {
		mw.errorDialog(owner, "Unable to compile value. Check expression.")
		return 1, err
	}

	var dlg *walk.Dialog
	var acceptPB, cancelPB *walk.PushButton

	return Dialog{
		AssignTo:      &dlg,
		Title:         "Set setting name",
		DefaultButton: &acceptPB,
		CancelButton:  &cancelPB,
		MinSize:       Size{300, 100},
		Layout:        VBox{},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{
						Text: "Name:",
					},
					LineEdit{
						AssignTo: &mw.nameEdit,
					},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					PushButton{
						AssignTo: &acceptPB,
						Text:     "OK",
						OnClicked: func() {
							if mw.nameEdit.Text() == "" {
								mw.errorDialog(owner, "Invalid name.")
								return
							} else {
								mw.saveSettings()
								mw.nameBox.SetModel(mw.settings.Items)
								dlg.Accept()
								return
							}
						},
					},
					PushButton{
						AssignTo:  &cancelPB,
						Text:      "Cancel",
						OnClicked: func() { dlg.Cancel() },
					},
				},
			},
		},
	}.Run(owner)
}

func (mw *MyMainWindow) errorDialog(owner walk.Form, msg string) (int, error) {
	var dlg *walk.Dialog
	return Dialog{
		AssignTo: &dlg,
		Title:    "Error",
		MinSize:  Size{200, 80},
		Layout:   VBox{},
		Children: []Widget{
			Composite{
				Layout: VBox{},
				Children: []Widget{
					Label{
						Text: msg,
					},
					PushButton{
						Text: "OK",
						OnClicked: func() {
							dlg.Accept()
							return
						},
					},
				},
			},
		},
	}.Run(owner)
}

func (mw *MyMainWindow) saveSettings() {
	var item = Item{
		mw.nameEdit.Text(),
		mw.regEdit.Text(),
	}
	var flag bool = true
	for i, v := range mw.settings.Items {
		if v.Name == item.Name {
			flag = false
			mw.settings.Items[i].RegExp = item.RegExp
			break
		}
	}
	if flag {
		mw.settings.Items = append(mw.settings.Items, &item)
	}

	fp, err := os.OpenFile("config.yml", os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()
	writer := bufio.NewWriter(fp)
	yml, err := yaml.Marshal(mw.settings)
	if err != nil {
		log.Fatal(err)
	}
	_, err = writer.WriteString(string(yml))
	if err != nil {
		log.Fatal(err)
	}
	writer.Flush()
}
