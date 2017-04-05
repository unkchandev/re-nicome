package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"regexp"
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

	logTE *walk.TextEdit
}

type Item struct {
	Name   string `yaml:"Name"`
	RegExp string `yaml:"RegExp"`
}
type Setting struct {
	Items []*Item `yaml:"Items"`
}

var logch = make(chan string, 10)

const LETTERS = "0123456789"

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
			fmt.Println(msg)
			mw.logTE.AppendText(msg + "\n")
			time.Sleep(100 * time.Millisecond)
		}
	}()

	if _, err := (MainWindow{
		AssignTo: &mw.MainWindow,
		Title:    "RegExp Replace",
		MinSize:  Size{400, 400},
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
	mw.logTE.AppendText("das")
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
	s := mw.regEdit.Text()
	ret, _ := regexp.Compile(`__TIME\[(.+?)\]__`)
	rec, _ := regexp.Compile(`__COMMENT__`)
	i := 1

	timeFmt := ret.FindStringSubmatch(s)
	if timeFmt == nil || len(timeFmt) != 2 {
		logch <- "Error[1]: Unable to parse file: " + path
		return
	}

	s = ret.ReplaceAllString(s, "(.+?)")
	s = rec.ReplaceAllString(s, "(.+?)")
	// s == "(.+?)\t(.+?)"

	re, err := regexp.Compile(s)
	if err != nil {
		logch <- "Error[2]: Unable to parse file: " + path
		return
	}

	lines, err := readCommentFile(path)
	if err != nil {
		logch <- err.Error()
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

	for _, comment := range lines {
		if comment == "" {
			continue
		}

		tc := re.FindStringSubmatch(comment)
		if len(tc) != 3 {
			logch <- "Error[3]: Unable to parse file: " + path
			return
		}
		timestr := tc[1]
		comstr := tc[2]
		if strings.ToUpper(timeFmt[1]) != "UNIXTIME" {
			loc, _ := time.LoadLocation("Asia/Tokyo")
			t, err := time.ParseInLocation(timeFmt[1], timestr, loc)
			if err != nil {
				logch <- "Error[4]: Unable to parse file: " + path
				return
			}
			timestr = strconv.FormatInt(t.Unix(), 10)
		}

		// magic
		for keta := 12 - len(timestr); keta > 0; keta-- {
			timestr = timestr + string(LETTERS[int(rand.Int63()%int64(len(LETTERS)))])
		}

		outcom := fmt.Sprintf("<chat no=\"%d\" vpos=\"%s\">%s</chat>\n", i, timestr, comstr)
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
